package checker

import (
	"bytes"
	"encoding/json"
	"github.com/foxdalas/deploy-checker/pkg/checker_const"
	"github.com/foxdalas/deploy-checker/pkg/docker"
	"github.com/foxdalas/deploy-checker/pkg/elastic"
	"github.com/foxdalas/deploy-checker/pkg/k8s"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"k8s.io/api/core/v1"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

var _ checker.Checker = &Checker{}

func New(version string, logging *log.Entry) *Checker {
	return &Checker{
		Ver:       version,
		Logging:   logging,
		StopCh:    make(chan struct{}),
		WaitGroup: sync.WaitGroup{},
	}
}

func (c *Checker) Init() {
	c.Log().Infof("Checker %s starting", c.Ver)
	k, err := k8s.New(c, c.KubeConfig, c.KubeNamespace, c.Development)
	if err != nil {
		c.Log().Fatal()
	}

	// handle sigterm correctly
	cc := make(chan os.Signal, 1)
	signal.Notify(cc, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		s := <-cc
		logger := c.Log().WithField("signal", s.String())
		logger.Debug("received signal")
		c.Stop()
	}()

	if c.DeployProgress {
		return
	}
	if c.Report && !c.MonitoringOnly {
		var wg sync.WaitGroup
		exitCode := 0
		for _, app := range strings.Split(c.Apps, ",") {
			c.Log().Infof("Starting monitoring deployment for %s", app)
			wg.Add(1)
			go func(name string) {
				err = k.Wait(name, &wg)
				if err != nil {
					exitCode = 1
				}
			}(app)
		}
		c.Log().Info("Waiting for deployment to finish")
		wg.Wait()
		if os.Getenv("ROLLBAR_ACCESS_TOKEN") != "" {
			c.rollbarReport()
		}
		err = c.elasticReport()
		if err != nil {
			exitCode = 2
		}
		if _, err := os.Stat(c.MonitoringRules); !os.IsNotExist(err) {
			if !c.Development {
				c.monitoringK8s()
			}
		} else {
			c.Log().Warnf("Directory %s is not exist.", c.MonitoringRules)
		}
		os.Exit(exitCode)
	}

	if c.MonitoringOnly {
		c.monitoringK8s()
		return
	}

	c.predeployChecks(c.Prefix, c.Apps)
}

func (c *Checker) Log() *log.Entry {
	return c.Logging
}

func (c *Checker) Version() string {
	return c.Ver
}

func (c *Checker) Stop() {
	c.Log().Info("shutting things down")
	close(c.StopCh)
	os.Exit(0)
}

func (c *Checker) predeployDocker(images []string) {
	docker, err := docker.New(c.DockerUsername, c.DockerPassword, *c.Logging)
	var wg sync.WaitGroup
	for _, image := range images {
		wg.Add(1)
		go func(app string) {
			defer wg.Done()

			c.Log().Infof("Checking image %s with tag %s", app, c.DockerTag)
			if err != nil {
				c.Log().Fatal(err)
			}
			if docker.IsDockerImageExist(app, c.DockerTag) {
				c.Log().Infof("Docker container %s with tag %s exist", app, c.DockerTag)
			} else {
				log.Errorf("Docker container %s with tag %s exist", app, c.DockerTag)
			}
		}(image)
	}
	wg.Wait()
}

func (c *Checker) predeployK8s() []string {
	k, err := k8s.New(c, c.KubeConfig, c.KubeNamespace, c.Development)
	if err != nil {
		c.Log().Fatal(err)
	}
	c.Log().Info("Starting pre deploy check")
	return k.PrepareDeployment(c.Development, c.DockerRepository)
}

func (c *Checker) monitoringK8s() {
	k, err := k8s.New(c, c.KubeConfig, c.KubeNamespace, c.Development)
	if err != nil {
		c.Log().Fatal(err)
	}

	repoAlert, err := k.GetAlertFromFile(c.MonitoringRules)
	if err != nil {
		c.Log().Fatalf("Problem in repo file: %s", err)
	}

	c.Log().Info("Getting current configmap")
	configmap, err := k.GetConfigMap("prometheus-aviasales", "prometheus")

	backup := &v1.ConfigMap{}
	b, err := json.Marshal(configmap)
	if err != nil {
		c.Log().Fatal(err)
	}

	err = json.Unmarshal(b, backup)
	if err != nil {
		c.Log().Fatal(err)
	}

	data := k.GetAlerts(configmap.Data["alerts"])

	c.Log().Info("Merging configmaps")
	for _, group := range repoAlert.Groups {
		c.Log().Infof("Procesing group name %s", group.Name)
		for k, v := range data.Groups {
			if v.Name == group.Name {
				c.Log().Infof("Alerts for %s is already exist. Deleting", group.Name)
				data.Groups = append(data.Groups[:k], data.Groups[k+1:]...)
			}
		}
		data.Groups = append(data.Groups, group)
	}
	binaryData, err := yaml.Marshal(data)
	if err != nil {
		c.Log().Error(err)
	}
	configmap.Data["alerts"] = string(binaryData)
	c.Log().Infof("Uploading alerts to configmap %s", configmap.Name)
	_, err = k.SetConfigMap(configmap, "prometheus")
	if err != nil {
		c.Log().Fatal(err)
	}
}

func (c *Checker) predeployChecks(prefix string, apps string) {
	images := c.predeployK8s()
	if !c.SkipCheckImage {
		c.predeployDocker(images)
	}
	c.Log().Info("All checks passed")
}

func (c *Checker) elasticReport() error {
	e, err := elastic.New(c, c.ElasticSearchURL)
	if err != nil {
		c.Log().Error(err)
		return err
	}
	e.Notify(c.Apps, "deploy_log", c.User, c.KubeNamespace, c.DockerTag)
	return nil
}

func (c *Checker) rollbarReport() {

	data := rollbarData{
		AccessToken:   os.Getenv("ROLLBAR_ACCESS_TOKEN"),
		Environment:   os.Getenv("DATACENTER"),
		Revision:      os.Getenv("COMMIT_HASH"),
		LocalUsername: c.User,
		Comment:       os.Getenv("ROLLBAR_COMMENT"),
	}

	b, err := json.Marshal(data)
	if err != nil {
		c.Log().Error(err)
	}

	req, err := http.NewRequest("POST", "https://api.rollbar.com/api/1/deploy/", bytes.NewBuffer(b))
	if err != nil {
		c.Log().Error(err)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log().Panic(err)
	}
	defer resp.Body.Close()
}
