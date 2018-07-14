package main

import (
	"errors"
	"flag"
	"github.com/heroku/docker-registry-client/registry"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/util/homedir"
	"os"
	"path/filepath"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/api/apps/v1"
	"github.com/foxdalas/deploy-checker/pkg/k8s"
)

type Checker struct {
	docker docker
	kube   kube
	deployment string
	currentDeployment *v1.Deployment
	inRepoDeployment *v1.Deployment
}

type docker struct {
	repository string
	tag        string

	usename  string
	password string
}

type kube struct {
	kubeconfig    string
	kubenamespace string
}

func init() {

}

func main() {
	c := &Checker{}

	err := params(c)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	//Connect to kubernates
	kubernetesClient := k8s.New(c.kube.kubeconfig)

	//Read Deployment file
	c.inRepoDeployment = k8s.ReadDeploymentFile(c.deployment)

	if k8s.IsDeploymentExist(kubernetesClient, c.kube.kubenamespace, c.inRepoDeployment.Name) {
		c.currentDeployment = k8s.ReadKubernetesDeployment(kubernetesClient, c.kube.kubenamespace, c.inRepoDeployment.Name)
		c.updateDeploymentFile()
	} else {
		log.Infof("Deployment not found in kubernetes. Is a new deploy %s", c.inRepoDeployment.Name)
	}

	hub, err := c.connectToRegistry()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	tags, err := c.getTags(hub)
	if err != nil {
		log.Error(err)
		os.Exit(1)

	}
	if c.isTagExist(tags) {
		log.Infof("Docker container %s with tag %s exist", c.docker.repository, c.docker.tag)
	} else {
		log.Errorf("Docker container %s with tag %s exist", c.docker.repository, c.docker.tag)
	}
}

func params(c *Checker) error {
	flag.StringVar(&c.docker.repository, "repository", "", "Docker repository")
	flag.StringVar(&c.docker.tag, "tag", "", "Docker repository tag")
	if home := homedir.HomeDir(); home != "" {
		flag.StringVar(&c.kube.kubeconfig, "kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		flag.StringVar(&c.kube.kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.StringVar(&c.kube.kubenamespace, "namespace", "", "Kubernetes namespace")
	flag.StringVar(&c.deployment, "deployment", "", "Deployment file")

	c.docker.usename = os.Getenv("DOCKER_USERNAME")
	c.docker.password = os.Getenv("DOCKER_PASSWORD")

	flag.Parse()

	if c.docker.repository == "" {
		return errors.New("Please provide -repository option")
	}
	if c.docker.tag == "" {
		return errors.New("Please provide -tag option")
	}

	if c.docker.usename == "" {
		return errors.New("Please provide DOCKER_USERNAME environment value")
	}
	if c.docker.password == "" {
		return errors.New("Please provide DOCKER_PASSWORD environment value")
	}
	if c.kube.kubenamespace == "" {
		return errors.New("Please provide -namespace option")
	}

	return nil
}

func (c *Checker) connectToRegistry() (*registry.Registry, error) {
	url := "https://registry-1.docker.io/"
	username := c.docker.usename  // anonymous
	password := c.docker.password // anonymous
	hub, err := registry.New(url, username, password)

	return hub, err
}

func (c *Checker) getTags(hub *registry.Registry) ([]string, error) {
	tags, err := hub.Tags(c.docker.repository)
	return tags, err
}

func (c *Checker) isTagExist(tags []string) bool {
	return contains(tags, c.docker.tag)
}

func contains(slice []string, item string) bool {
	set := make(map[string]struct{}, len(slice))
	for _, s := range slice {
		set[s] = struct{}{}
	}

	_, ok := set[item]
	return ok
}

func (c *Checker) updateDeploymentFile() {
	log.Infof("Current deployment: %d", *c.currentDeployment.Spec.Replicas)
	log.Infof("Repo deployment: %d", *c.inRepoDeployment.Spec.Replicas)

	//Fix replicas
	c.inRepoDeployment.Spec.Replicas = c.currentDeployment.Spec.Replicas

	f, err := os.Create(c.deployment)
	if err != nil {
		log.Panic(err)
	}
	defer f.Close()

	log.Infof("Updating file %s", c.deployment)
	s := json.NewYAMLSerializer(json.DefaultMetaFactory, nil, nil)
	err = s.Encode(c.inRepoDeployment, f)
	if err !=nil {
		log.Panic(err)
	}
}