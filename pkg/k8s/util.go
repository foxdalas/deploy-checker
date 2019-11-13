package k8s

import (
	"bytes"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	v1 "k8s.io/api/apps/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
	"os"
	"path/filepath"
	"strings"
)

func (k *k8s) findResources(searchDir string) []resourcesFile {
	var data []resourcesFile

	err := filepath.Walk(searchDir, func(path string, f os.FileInfo, err error) error {
		if strings.Contains(path, "vendor") {
			return nil
		}
		if strings.Contains(path, "deployment.yml") {
			dat, err := readFile(path)
			if err != nil {
				k.Log().Fatal(err)
			}
			k.Log().Debugf("Found deployment file %s", path)
			data = append(data, resourcesFile{
				path:         path,
				data:         dat,
				resourceType: "deployment",
			})
		}
		if strings.Contains(path, "statefulset.yml") {
			dat, err := readFile(path)
			if err != nil {
				k.Log().Fatal(err)
			}
			k.Log().Debugf("Found statefulset file %s", path)
			data = append(data, resourcesFile{
				path:         path,
				data:         dat,
				resourceType: "statefulset",
			})
		}
		return nil
	})
	if err != nil {
		k.Log().Error(err)
	}

	return data
}

func (k *k8s) writeDeploymentFile(path string) {
	f, err := os.Create(path)
	if err != nil {
		k.Log().Fatal(err)
	}

	defer f.Close()

	k.Log().Debugf("Updating file %s", path)
	s := json.NewYAMLSerializer(json.DefaultMetaFactory, nil, nil)
	err = s.Encode(k.yamlResources.deployment, f)
	if err != nil {
		k.Log().Fatalf("File %s: %s", path, err)
	}
}

func (k *k8s) writeResourceFile(data []byte, path string) {
	f, err := os.Create(path)
	if err != nil {
		k.Log().Fatal(err)
	}
	defer f.Close()
	_, err = f.Write(data)
	if err != nil {
		k.Log().Fatal(err)
	}
}

func (k *k8s) writeStatefulsetFile(path string) {
	f, err := os.Create(path)
	if err != nil {
		k.Log().Fatal(err)
	}

	defer f.Close()

	k.Log().Infof("Updating file %s", path)
	s := json.NewYAMLSerializer(json.DefaultMetaFactory, nil, nil)
	err = s.Encode(k.yamlResources.statefulset, f)
	if err != nil {
		k.Log().Fatalf("File %s: %s", path, err)
	}
}

func (k *k8s) Log() *log.Entry {
	return k.checker.Log().WithField("context", "k8s")
}

func readFile(path string) ([]byte, error) {
	dat, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return dat, nil
}

func (k *k8s) resourceToBytes(data []byte) ([]byte, error) {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode(data, nil, nil)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	s := json.NewYAMLSerializer(json.DefaultMetaFactory, nil, nil)
	err = s.Encode(obj, &buf)
	if err != nil {
		k.Log().Fatal(err)
	}
	k.Log().Info(string(buf.Bytes()))
	return buf.Bytes(), err
}

func (k *k8s) objectToBytes(obj runtime.Object) []byte {
	s := json.NewYAMLSerializer(json.DefaultMetaFactory, nil, nil)
	var buf bytes.Buffer

	err := s.Encode(obj, &buf)
	if err != nil {
		k.Log().Fatal(err)
	}
	return buf.Bytes()
}

func (k *k8s) getKubernetesStatefulset(name string, namespace string) *appsv1beta1.StatefulSet {
	obj, err := k.client.AppsV1beta1().StatefulSets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		k.Log().Fatal(err)
	}
	return obj
}

func (k *k8s) convertResources(res *resourcesFile) {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode(res.data, nil, nil)
	if err != nil {
		k.Log().Fatalf("Error while decoding YAML object in file %s. Err was: %s", res.path, err)
	}

	switch res.resourceType {
	case "deployment":
		dst := &v1.Deployment{}
		if err := scheme.Scheme.Convert(obj, dst, nil); err != nil {
			k.Log().Fatal(err)
		}
		dst.TypeMeta.APIVersion = "apps/v1"
		dst.TypeMeta.Kind = "Deployment"
		res.data = k.objectToBytes(dst)
	case "statefulset":
		dst := &v1.StatefulSet{}
		dst.TypeMeta.APIVersion = "apps/v1"
		dst.TypeMeta.Kind = "Statefulset"
		res.data = k.objectToBytes(dst)
	}
}

func (k *k8s) replaceNodeSelector(res *resourcesFile) {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode(res.data, nil, nil)
	if err != nil {
		k.Log().Fatalf("ReplaceNodeSelector Error while decoding YAML object in file %s. Err was: %s", res.path, err)
	}
	nodeSelector := map[string]string{"kubernetes.io/role": "development"}

	switch o := obj.(type) {
	case *v1.Deployment:
		o.Spec.Template.Spec.NodeSelector = nodeSelector
		res.data = k.objectToBytes(o)
	case *v1.StatefulSet:
		o.Spec.Template.Spec.NodeSelector = nodeSelector
		res.data = k.objectToBytes(o)
	}
}

func (k *k8s) cleanupResources(res *resourcesFile) {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode(res.data, nil, nil)
	if err != nil {
		k.Log().Fatalf("Cleanup Resources Error while decoding YAML object in file %s. Err was: %s", res.path, err)
	}

	var containers []corev1.Container

	resources := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			"memory": resource.MustParse("4Gi"),
		},
		Requests: corev1.ResourceList{
			"cpu":    resource.MustParse("1m"),
			"memory": resource.MustParse("1Mi"),
		},
	}

	switch o := obj.(type) {
	case *v1.Deployment:
		for _, c := range o.Spec.Template.Spec.Containers {
			c.Resources = resources
			containers = append(containers, c)
		}
		o.Spec.Template.Spec.Containers = containers
		res.data = k.objectToBytes(o)
	case *v1.StatefulSet:
		for _, c := range o.Spec.Template.Spec.Containers {
			c.Resources = resources
			containers = append(containers, c)
		}
		o.Spec.Template.Spec.Containers = containers
		res.data = k.objectToBytes(o)
	}
}

func (k *k8s) updateTimestamp(res *resourcesFile) {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode(res.data, nil, nil)
	if err != nil {
		k.Log().Fatalf("Cleanup Resources Error while decoding YAML object in file %s. Err was: %s", res.path, err)
	}
	switch o := obj.(type) {
	case *v1.Deployment:
		o.CreationTimestamp = metav1.Now()
		o.ObjectMeta.CreationTimestamp = metav1.Now()
		o.Spec.Template.CreationTimestamp = metav1.Now()
		res.data = k.objectToBytes(o)
	case *v1.StatefulSet:
		o.CreationTimestamp = metav1.Now()
		o.ObjectMeta.CreationTimestamp = metav1.Now()
		o.Spec.Template.CreationTimestamp = metav1.Now()
		res.data = k.objectToBytes(o)
	}
}

func (k *k8s) prepareForDevelopment(res *resourcesFile) {
	k.replaceNodeSelector(res)
	k.cleanupResources(res)
}

func (k *k8s) fixReplicas(res *resourcesFile) {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode(res.data, nil, nil)
	if err != nil {
		k.Log().Fatalf("Cleanup Resources Error while decoding YAML object in file %s. Err was: %s", res.path, err)
	}
	switch o := obj.(type) {
	case *v1.Deployment:
		if k.isResourceExist(o.Name, o.Namespace, res.resourceType) {
			k.Log().Infof("Deployment %s exist in namespace %s", o.Name, o.Namespace)
			deployment := k.getKubernetesDeployment(o.Name, o.Namespace)
			if *deployment.Spec.Replicas != *o.Spec.Replicas {
				k.Log().Infof("Current deployment is changed. Replicas in repository %d and %d replicas in k8s", *o.Spec.Replicas,
					*deployment.Spec.Replicas)
				*o.Spec.Replicas = *deployment.Spec.Replicas
				res.data = k.objectToBytes(o)
			}
		}
	case *v1.StatefulSet:
		if k.isResourceExist(o.Name, o.Namespace, res.resourceType) {
			k.Log().Debugf("Statefulset %s exist in namespace %s", o.Name, o.Namespace)
			statefulset := k.getKubernetesStatefulset(o.Name, o.Namespace)
			if *statefulset.Spec.Replicas != *o.Spec.Replicas {
				k.Log().Debugf("Current deployment is changed. Replicas in repository %d and %d replicas in k8s", *o.Spec.Replicas,
					*statefulset.Spec.Replicas)
				*o.Spec.Replicas = *statefulset.Spec.Replicas
				res.data = k.objectToBytes(o)
			}
		}
	}
}

func (k *k8s) isResourceExist(name string, namespace string, resType string) bool {
	var err error
	switch resType {
	case "deployment":
		_, err = k.client.AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
	case "statefulset":
		_, err = k.client.AppsV1().StatefulSets(namespace).Get(name, metav1.GetOptions{})
	}
	if err != nil {
		return false
	}
	return true
}
