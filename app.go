package main

import (
    "fmt"
    "flag"
    "path/filepath"
    "context"
    "strings"

    //"k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/clientcmd"
    "k8s.io/client-go/util/homedir"
    corev1 "k8s.io/api/core/v1"
)

const PORTWORX_PROVISIONER = "pxd.portworx.com"
const PORTWORX_SCHEDULER = "stork"

//const PORTWORX_PROVISIONER = "kubernetes.io/aws-ebs"
var NS_EMAIL_ANNOTATION = "managedprojects/technical-contacts"

type InvalidPod struct {
    Name, Namespace string
}

func main() {
    fmt.Println("Starting PX-Scheduler check report")
    clientset := connect()

    invalidPods := getInvalidPods(clientset)
    invalidPodByMail := make(map[string][]InvalidPod, len(invalidPods))

    for _, pod := range invalidPods {
	name := pod.Name
	namespace := pod.Namespace
	emails := getNamespaceEmails(clientset, namespace)
	for _, email := range(strings.Split(emails, ";")) {
	    invalidPodByMail[email] = append(invalidPodByMail[email], InvalidPod{name, namespace})
	}
    }

    for email, podList := range invalidPodByMail {
	for _, pod := range podList {
	    fmt.Println("E-mail:", email)
	    fmt.Println("Pod: ", pod.Name)
	    fmt.Println("Namespace: ", pod.Namespace)
	}
    }
}

func connect() *kubernetes.Clientset {
    var kubeconfig *string
    
    if home := homedir.HomeDir(); home != "" {
	kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
    } else {
	kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
    }
    flag.Parse()

    // Use the current context for the kubeconfig
    config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
    if err != nil {
	panic(err.Error())
    }

    // Create the clientset
    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
	panic(err.Error())
    }

    return clientset
}

func getInvalidPods(clientset *kubernetes.Clientset) []corev1.Pod {
    pods := getAllPods(clientset)

    invalidPods := []corev1.Pod{}
    for _, pod := range pods.Items {
	namespace := pod.Namespace
	for _, volume := range pod.Spec.Volumes {
	    // If the volume is given by Portworx
	    if volume.VolumeSource.PortworxVolume != nil && pod.Spec.SchedulerName != PORTWORX_SCHEDULER {
		// Portworx volume found in pod not scheduled with stork
		invalidPods = append(invalidPods, pod)
	    }

	    // If the volume is given by Portworx via a PVC
	    //	TODO: Check when would this not be the case
	    if volume.VolumeSource.PersistentVolumeClaim != nil && pod.Spec.SchedulerName != PORTWORX_SCHEDULER {
		name := volume.VolumeSource.PersistentVolumeClaim.ClaimName
		pvc, err := clientset.CoreV1().PersistentVolumeClaims(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		_ = err
		if pvc.Annotations["volume.kubernetes.io/storage-provisioner"] == PORTWORX_PROVISIONER {
		    invalidPods = append(invalidPods, pod)
		} else if pvc.Annotations["volume.beta.kubernetes.io/storage-provisioner"] == PORTWORX_PROVISIONER {
		    invalidPods = append(invalidPods, pod)
		}
	    }

	}
    }
    return invalidPods
}

func getAllPods(clientset *kubernetes.Clientset) *corev1.PodList {
    pods, err := clientset.CoreV1().Pods("sergioperezbis-dev").List(context.TODO(), metav1.ListOptions{})
    if err != nil {
	panic(err.Error())
    }
    return pods
}

func getNamespaceEmails(clientset *kubernetes.Clientset, namespace string) string {
    ns, err := clientset.CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{})
    if err != nil {
	panic(err.Error())
    }
    return ns.Annotations["managedprojects/technical-contacts"]
}
