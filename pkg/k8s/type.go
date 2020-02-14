package k8s

import (
	"github.com/foxdalas/deploy-checker/pkg/checker_const"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/api/apps/v1beta1"
	"k8s.io/client-go/kubernetes"
)

type resourcesFile struct {
	path         string
	data         []byte
	resourceType string
}

type k8s struct {
	checker checker.Checker
	log     *logrus.Entry

	client *kubernetes.Clientset

	namespace      string
	deploymentFile string

	parallel bool

	yamlResources struct {
		deployment  *v1.Deployment
		statefulset *v1beta1.StatefulSet
	}

	k8sResources struct {
		deployment  *v1.Deployment
		statefulset *v1beta1.StatefulSet
	}

	development bool
}

type Alerts struct {
	Groups []Group `yaml:"groups"`
}

type AlertFile struct {
	Groups []Group `yaml:"groups"`
}

type Group struct {
	Name  string `yaml:"name"`
	Rules []struct {
		Alert       string `yaml:"alert"`
		Annotations struct {
			Description string `yaml:"description"`
			Summary     string `yaml:"summary"`
		} `yaml:"annotations"`
		Expr   string `yaml:"expr"`
		For    string `yaml:"for"`
		Labels struct {
			Severity string `yaml:"severity"`
			Namespace string `yaml:"namespace"`
		} `yaml:"labels"`
	} `yaml:"rules"`
}

const (
	TimedOutReason = "ProgressDeadlineExceeded"
)
