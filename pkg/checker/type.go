package checker

import (
	log "github.com/sirupsen/logrus"
	"k8s.io/api/extensions/v1beta1"
	"sync"
)

type Checker struct {
	Ver     string
	Logging *log.Entry

	User string

	//Docker
	DockerRepository string
	DockerTag        string
	DockerUsername   string
	DockerPassword   string

	//K8S
	KubeConfig    string
	KubeNamespace string

	//ElasticSearch
	ElasticSearchURL string

	Apps   string
	Prefix string

	//K8S Deployments
	CurrentDeployment *v1beta1.Deployment
	InRepoDeployment  *v1beta1.Deployment

	DeployProgress   bool
	Report           bool
	DeployMonitoring string

	StopCh    chan struct{}
	WaitGroup sync.WaitGroup
}

type rollbarData struct {
	AccessToken   string `json:"access_token"`
	Environment   string `json:"environment"`
	Revision      string `json:"revision"`
	LocalUsername string `json:"local_username"`
	Comment       string `json:"comment"`
}
