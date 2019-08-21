package k8s

import (
	"bytes"
	"fmt"
	"github.com/foxdalas/deploy-checker/pkg/checker_const"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

func New(checker checker.Checker, kubeconfig string, namespace string, development bool) (*k8s, error) {
	var config *rest.Config
	var err error

	if os.Getenv("KUBECONFIG_CONTENT") != "" {
		checker.Log().Info("Using configuration from environment value KUBECONFIG_CONTENT")
		config, err = clientcmd.RESTConfigFromKubeConfig([]byte(os.Getenv("KUBECONFIG_CONTENT")))
		if err != nil {
			return nil, err
		}
	} else {
		config, err = rest.InClusterConfig()
		if err != nil {
			checker.Log().Warnf("failed to create in-cluster client: %v.", err)
			config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
			if err != nil {
				return nil, err
			}
		}
	}

	clientset, err := kubernetes.NewForConfig(config)

	return &k8s{
		checker:   checker,
		client:    clientset,
		namespace: namespace,
	}, err
}

func (k *k8s) isDeploymentExist(name string) bool {
	deploymentsClient := k.client.AppsV1().Deployments(k.namespace)
	_, err := deploymentsClient.Get(name, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return true
}

func (k *k8s) getKubernetesDeployment(name string) *appsv1.Deployment {
	deploymentsClient := k.client.AppsV1().Deployments(k.namespace)
	obj, err := deploymentsClient.Get(name, metav1.GetOptions{})
	if err != nil {
		k.Log().Fatal(err)
	}

	return obj
}

func (k *k8s) getDeploymentFile(path string) {
	dat, err := ioutil.ReadFile(path)
	if err != nil {
		k.Log().Error(err)
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode([]byte(dat), nil, nil)
	if err != nil {
		k.Log().Fatalf("Error while decoding YAML object. Err was: %s", err)
	}
	switch o := obj.(type) {
	case *appsv1.Deployment:
		k.yamlDeployment = o
		k.Log().Debugf("Deployment file %s is apps/v1", path)
		return
	case *v1beta1.Deployment:
		k.Log().Warnf("DEPLOYMENT FORMAT IS %s PLEASE USE APPS/V1", o.APIVersion)
		k.yamlDeployment, err = k.convertDeployment(o)
		k.writeDeploymentFile(path)
		k.getDeploymentFile(path)
  case *batchv1.Job: //batch here was added for testing purpose, remove this case anytime
    k.Log().Warnf("Type Job is not supported! Skiping..", path)
    return
	default:
		k.Log().Fatalf("File %s is not a kubernetes deployment", path)
	}
}

func (k *k8s) convertDeployment(data *v1beta1.Deployment) (*appsv1.Deployment, error) {
	var deployment *appsv1.Deployment
	dst := &appsv1.Deployment{}
	if err := scheme.Scheme.Convert(data, dst, nil); err != nil {
		return deployment, err
	}
	deployment = dst
	deployment.TypeMeta.APIVersion = "apps/v1"
	deployment.TypeMeta.Kind = "Deployment"

	return deployment, nil
}

func (k *k8s) updateDeploymentFile(path string) {
	if *k.k8sDeployment.Spec.Replicas != *k.yamlDeployment.Spec.Replicas {
		k.Log().Infof("Current deployment is changed. Replicas in repository %d and %d replicas in k8s", *k.yamlDeployment.Spec.Replicas,
			*k.k8sDeployment.Spec.Replicas)
	}
	//Fix replicas
	*k.yamlDeployment.Spec.Replicas = *k.k8sDeployment.Spec.Replicas
	k.writeDeploymentFile(path)
}

func (k *k8s) cleanupResources() {
	k.Log().Info("Cleanup deployment for development environment")
	var containers []v1.Container

	resources := v1.ResourceRequirements{
		Limits: v1.ResourceList{
			"memory": resource.MustParse("4Gi"),
		},
		Requests: v1.ResourceList{
			"cpu":    resource.MustParse("1m"),
			"memory": resource.MustParse("1Mi"),
		},
	}

	for _, c := range k.yamlDeployment.Spec.Template.Spec.Containers {
		c.Resources = resources
		containers = append(containers, c)
	}
	k.yamlDeployment.Spec.Template.Spec.Containers = containers
}

func (k *k8s) replaceNodeSelector() {
	k.Log().Info("Replace nodeSelector")

	k.yamlDeployment.Spec.Template.Spec.NodeSelector = map[string]string{
		"kubernetes.io/role": "development",
	}
}

func (k *k8s) prepareDeploymentForDevelopement(path string) {
	k.cleanupResources()
	k.replaceNodeSelector()
	k.writeDeploymentFile(path)
}

func (k *k8s) PrepareDeployment(development bool, dockerRepo string) []string {
	var images []string
	for _, path := range k.findDeployments(".") {
		k.getDeploymentFile(path)

		if development {
			k.prepareDeploymentForDevelopement(path)
		}

		if k.isDeploymentExist(k.yamlDeployment.Name) {
			k.k8sDeployment = k.getKubernetesDeployment(k.yamlDeployment.Name)
			k.updateDeploymentFile(path)
		} else {
			k.Log().Infof("Deployment not found in kubernetes. Is a new deploy %s", k.yamlDeployment.Name)
		}
		image, err := k.getImages(dockerRepo)
		if err != nil {
			k.Log().Error(err)
		}
		images = append(images, image)
	}
	return images
}

var variableRegex = regexp.MustCompile(`\$[A-Z]+(_|[A-Z]+)*`)

func (k *k8s) UnprocessedVariablesDeployments() []string {
	var res []string
	for _, path := range k.findDeployments(".") {
		k.getDeploymentFile(path)

		if !k.isDeploymentExist(k.yamlDeployment.Name) {
			k.Log().Infof("Deployment not found in kubernetes. Is a new deploy %s", k.yamlDeployment.Name)
			continue
		}

		deployment := k.getKubernetesDeployment(k.yamlDeployment.Name)

		var yamlBytes []byte
		s := json.NewYAMLSerializer(json.DefaultMetaFactory, nil, nil)
		err := s.Encode(deployment, bytes.NewBuffer(yamlBytes))
		if err != nil {
			k.Log().Fatal(err)
		}

		if variableRegex.Match(yamlBytes) {
			res = append(res, k.yamlDeployment.Name)
		}
	}

	return res
}

func (k *k8s) DeploymentProgress(deployment *appsv1.Deployment) appsv1.DeploymentConditionType {
	conditions := deployment.Status.Conditions
	lastCondition := conditions[len(k.k8sDeployment.Status.Conditions)-1]
	return lastCondition.Type
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

func getDeploymentCondition(status appsv1.DeploymentStatus, condType appsv1.DeploymentConditionType) *appsv1.DeploymentCondition {
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}

func (k *k8s) Wait(name string, wg *sync.WaitGroup) error {
	defer wg.Done()
	var message string
	ticker := 0
	for {
		state, status, err := k.deploymentInProgress(name)
		if err != nil {
			k.Log().Error(err)
			return err
		}
		if message != state {
			k.Log().Info(state)
			message = state
		}
		if status {
			return nil
		}
		time.Sleep(time.Second * 5)
		ticker++
	}
}

func (k *k8s) GetConfigMap(name string, namespace string) (*v1.ConfigMap, error) {
	return k.client.CoreV1().ConfigMaps(namespace).Get(name, metav1.GetOptions{})
}

func (k *k8s) SetConfigMap(configmap *v1.ConfigMap, namespace string) (*v1.ConfigMap, error) {
	return k.client.CoreV1().ConfigMaps(namespace).Update(configmap)
}

func (k *k8s) CreateConfigMap(configmap *v1.ConfigMap, namespace string) (*v1.ConfigMap, error) {
	return k.client.CoreV1().ConfigMaps(namespace).Create(configmap)
}

func (k *k8s) GetAlerts(data string) *Alerts {
	parsed := &Alerts{}
	err := yaml.Unmarshal([]byte(data), parsed)
	if err != nil {
		k.Log().Error(err)
	}
	return parsed
}

func (k *k8s) GetAlertFromFile(root string) (AlertFile, error) {
	alertsData := AlertFile{}

	var files []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".yml" {
			k.Log().Info(path)
			files = append(files, path)
		}

		return nil
	})
	if err != nil {
		return alertsData, err
	}
	for _, file := range files {
		parsed := AlertFile{}
		data, err := ioutil.ReadFile(file)
		if err != nil {
			return alertsData, err
		}

		err = yaml.Unmarshal(data, &parsed)
		if err != nil {
			return alertsData, err
		}

		for _, group := range parsed.Groups {
			alertsData.Groups = append(alertsData.Groups, group)
		}

	}
	return alertsData, err
}

func (k *k8s) getImages(repo string) (string, error) {
	for _, container := range k.yamlDeployment.Spec.Template.Spec.Containers {
		if strings.Split(container.Image, "/")[0] == repo {
			return strings.Split(container.Image, ":")[0], nil
		}
	}
	return "", errors.New("Image does not exist in deployment file")
}
