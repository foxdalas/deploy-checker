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

func (k *k8s) StatefulSetProgress(statefulset *appsv1.StatefulSet) appsv1.StatefulSetConditionType {
	conditions := statefulset.Status.Conditions
	lastCondition := conditions[len(k.k8sResources.statefulset.Status.Conditions)-1]
	return lastCondition.Type
}

func (k *k8s) isDStatefulsetExist(name string, namespace string) bool {
	_, err := k.client.AppsV1().StatefulSets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return true
}
