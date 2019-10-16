package k8s

import (
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (k *k8s) getStatefulSet(name string, namespace string) *appsv1.StatefulSet {
	obj, err := k.client.AppsV1().StatefulSets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		k.Log().Fatal(err)
	}
	return obj
}

func (k *k8s) isStatefulsetExist(name string, namespace string) bool {
	_, err := k.client.AppsV1().StatefulSets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return true
}
