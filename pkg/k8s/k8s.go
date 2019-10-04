package k8s

import (
	"github.com/foxdalas/deploy-checker/pkg/checker_const"
	"github.com/pkg/errors"
	yml "gopkg.in/yaml.v2"
	"io/ioutil"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
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
		checker:     checker,
		client:      clientset,
		namespace:   namespace,
		development: development,
	}, err
}

func (k *k8s) processingResources(resources []resourcesFile) {
	wg := sync.WaitGroup{}
	for _, res := range resources {
		wg.Add(1)
		go func(dat resourcesFile) {
			k.procerssingFile(&dat)
			wg.Done()
		}(res)
	}

	wg.Wait()
}

func (k *k8s) procerssingFile(res *resourcesFile) {
	k.Log().Debugf("Processing file %s", res.path)
	//Fix replicas
	k.fixReplicas(res)
	//Update timestamps
	k.updateTimestamp(res)
	//Prepare for development environment
	if k.development {
		k.prepareForDevelopment(res)
	}
	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode(res.data, nil, nil)
	if err != nil {
		k.Log().Fatalf("Error while decoding YAML object in file %s. Err was: %s", res.path, err)
	}
	switch o := obj.(type) {
	case *appsv1.Deployment:
		k.writeResourceFile(k.objectToBytes(o), res.path)
	case *v1beta1.Deployment:
		k.convertResources(res)
		k.procerssingFile(res)
	case *appsv1.StatefulSet:
		k.writeResourceFile(k.objectToBytes(o), res.path)
	case *batchv1.Job: //batch here was added for testing purpose, remove this case anytime
		k.Log().Warnf("Type Job is not supported! Skiping..", res.path)
	default:
		k.Log().Fatalf("File %s is not a kubernetes deployment", res.path)
	}
}

func (k *k8s) PrepareResources(dir string, development bool) {
	//var images []string
	k.processingResources(k.findResources(dir))
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
	err := yml.Unmarshal([]byte(data), parsed)
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

		err = yml.Unmarshal(data, &parsed)
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
	for _, container := range k.yamlResources.deployment.Spec.Template.Spec.Containers {
		if strings.Split(container.Image, "/")[0] == repo {
			return strings.Split(container.Image, ":")[0], nil
		}
	}
	return "", errors.New("Image does not exist in deployment file")
}
