package k8s

import (
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (k *k8s) getKubernetesDeployment(name string, namespace string) *appsv1.Deployment {
	obj, err := k.client.AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		k.Log().Fatal(err)
	}
	return obj
}

func (k *k8s) DeploymentProgress(deployment *appsv1.Deployment) appsv1.DeploymentConditionType {
	conditions := deployment.Status.Conditions
	lastCondition := conditions[len(k.k8sResources.deployment.Status.Conditions)-1]
	return lastCondition.Type
}

func getDeploymentCondition(status appsv1.DeploymentStatus, condType appsv1.DeploymentConditionType) *appsv1.DeploymentCondition {
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}

func (k *k8s) deploymentInProgress(name string) (string, bool, error) {
	deployment, err := k.client.AppsV1().Deployments(k.namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		k.Log().Error(err)
	}
	if deployment.Generation <= deployment.Status.ObservedGeneration {
		cond := getDeploymentCondition(deployment.Status, appsv1.DeploymentProgressing)
		if cond != nil && cond.Reason == TimedOutReason {
			return "", false, fmt.Errorf("deployment %q exceeded its progress deadline", name)
		}
		if deployment.Spec.Replicas != nil && deployment.Status.UpdatedReplicas < *deployment.Spec.Replicas {
			return fmt.Sprintf("Waiting for deployment %q rollout to finish: %d out of %d new replicas have been updated...", name, deployment.Status.UpdatedReplicas, *deployment.Spec.Replicas), false, nil
		}
		if deployment.Status.Replicas > deployment.Status.UpdatedReplicas {
			return fmt.Sprintf("Waiting for deployment %q rollout to finish: %d old replicas are pending termination...", name, deployment.Status.Replicas-deployment.Status.UpdatedReplicas), false, nil
		}
		if deployment.Status.AvailableReplicas < deployment.Status.UpdatedReplicas {
			return fmt.Sprintf("Waiting for deployment %q rollout to finish: %d of %d updated replicas are available...", name, deployment.Status.AvailableReplicas, deployment.Status.UpdatedReplicas), false, nil
		}
		return fmt.Sprintf("deployment %q successfully rolled out", name), true, nil
	}
	return fmt.Sprintf("Waiting for deployment spec update to be observed..."), false, nil
}
