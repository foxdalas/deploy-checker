package k8s

import (
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	log "github.com/sirupsen/logrus"
	"k8s.io/api/apps/v1"
	"io/ioutil"
	"k8s.io/client-go/kubernetes/scheme"
	"fmt"
)


func New(kubeconfig string) *kubernetes.Clientset {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Warnf("failed to create in-cluster client: %v.", err)
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			log.Warnf("failed to create kubeconfig client: %v.", err)
			log.Panic("kube init failed as both in-cluster and dev connection unavailable")
		}
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Panic(err)
	}

	return clientset
}

func IsDeploymentExist(client *kubernetes.Clientset, namespace string, name string) bool {
	deploymentsClient := client.AppsV1().Deployments(namespace)
	_, err := deploymentsClient.Get(name,metav1.GetOptions{})
	if err != nil {
		return true
	}
	return false
}

func ReadKubernetesDeployment(client *kubernetes.Clientset, namespace string, name string) *v1.Deployment {
	deploymentsClient := client.AppsV1().Deployments(namespace)
	obj, err := deploymentsClient.Get(name,metav1.GetOptions{})
	if err != nil {
		log.Panic(err)
	}

	return obj
}

func ReadDeploymentFile(file string) *v1.Deployment {
	dat, err := ioutil.ReadFile(file)
	if err != nil {
		log.Error(err)
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode([]byte(dat), nil, nil)
	if err != nil {
		log.Fatal(fmt.Sprintf("Error while decoding YAML object. Err was: %s", err))
	}
	switch o := obj.(type) {
	case *v1.Deployment:
		return o
	default:
		log.Panicf("File %s is not a kubernetes deployment", file)
		return nil
	}
}