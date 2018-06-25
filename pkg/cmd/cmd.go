package cmd

import (
	"errors"
	"flag"
	"github.com/foxdalas/deploy-checker/pkg/checker"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/util/homedir"
	"os"
	"path/filepath"
	"strings"
)

func Run(version string) {
	c := checker.New(version, makeLog())

	// parse env vars
	err := params(c)
	if err != nil {
		c.Log().Fatal(err)
	}
	c.Init()
}

func makeLog() *log.Entry {
	logtype := strings.ToLower(os.Getenv("LOG_TYPE"))
	if logtype == "" {
		logtype = "text"
	}
	if logtype == "json" {
		log.SetFormatter(&log.JSONFormatter{})
	} else if logtype == "text" {
		log.SetFormatter(&log.TextFormatter{
			ForceColors: true,
		})
	} else {
		log.WithField("logtype", logtype).Fatal("Given logtype was not valid, check LOG_TYPE configuration")
		os.Exit(1)
	}

	loglevel := strings.ToLower(os.Getenv("LOG_LEVEL"))
	if len(loglevel) == 0 {
		log.SetLevel(log.InfoLevel)
	} else if loglevel == "debug" {
		log.SetLevel(log.DebugLevel)
	} else if loglevel == "info" {
		log.SetLevel(log.InfoLevel)
	} else if loglevel == "warn" {
		log.SetLevel(log.WarnLevel)
	} else if loglevel == "error" {
		log.SetLevel(log.ErrorLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
	return log.WithField("context", "checker")
}

func params(c *checker.Checker) error {
	flag.BoolVar(&c.DeployProgress, "processing", false, "Checking kubernetes deploy progress")
	flag.BoolVar(&c.Report, "report", false, "Send deploy state to elasticsearch")

	if home := homedir.HomeDir(); home != "" {
		flag.StringVar(&c.KubeConfig, "kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		flag.StringVar(&c.KubeConfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.StringVar(&c.KubeNamespace, "namespace", "", "Kubernetes namespace")
	flag.StringVar(&c.DeploymentFile, "deployment", "", "Deployment file")

	flag.StringVar(&c.DockerRepository, "repository", "", "Docker repository")
	flag.StringVar(&c.DockerTag, "tag", "", "Docker repository tag")

	flag.StringVar(&c.User, "user", "ci", "Run user")

	flag.StringVar(&c.Apps, "apps", "", "Application list with separator")

	c.DockerUsername = os.Getenv("DOCKER_USERNAME")
	c.DockerPassword = os.Getenv("DOCKER_PASSWORD")

	c.ElasticSearchURL = os.Getenv("ELASTICSEARCH_URL")

	flag.Parse()

	if !c.DeployProgress && !c.Report {
		if c.DockerRepository == "" {
			return errors.New("Please provide -repository option")
		}
		if c.DockerTag == "" {
			return errors.New("Please provide -tag option")
		}

		if c.DockerUsername == "" {
			return errors.New("Please provide DOCKER_USERNAME environment value")
		}
		if c.DockerPassword == "" {
			return errors.New("Please provide DOCKER_PASSWORD environment value")
		}
	}

	if !c.Report {
		if c.KubeNamespace == "" {
			return errors.New("Please provide -namespace option")
		}
	}

	if c.Report {
		if c.Apps == "" {
			return errors.New("Please provide -apps option")
		}
	}
	return nil
}