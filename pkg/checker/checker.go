package checker

import (
	"github.com/foxdalas/deploy-checker/pkg/checker_const"
	"github.com/foxdalas/deploy-checker/pkg/docker"
	"github.com/foxdalas/deploy-checker/pkg/elastic"
	"github.com/foxdalas/deploy-checker/pkg/k8s"
	log "github.com/sirupsen/logrus"
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

	k, err := k8s.New(c, c.KubeConfig, c.KubeNamespace, c.DeploymentFile)
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
	if c.Report {
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
		c.elasticReport()
		os.Exit(exitCode)
	}
	c.predeployChecks()
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

func (c *Checker) predeployDocker() {
	docker, err := docker.New(c.DockerUsername, c.DockerPassword, c.DockerRepository, c.DockerTag)
	if err != nil {
		c.Log().Fatal(err)
	}
	if docker.IsDockerImageExist() {
		c.Log().Infof("Docker container %s with tag %s exist", c.DockerRepository, c.DockerTag)
	} else {
		log.Errorf("Docker container %s with tag %s exist", c.DockerRepository, c.DockerTag)
	}
}

func (c *Checker) predeployK8s() {
	k, err := k8s.New(c, c.KubeConfig, c.KubeNamespace, c.DeploymentFile)
	if err != nil {
		c.Log().Fatal(err)
	}
	k.PrepareDeployment()
}

func (c *Checker) predeployChecks() {
	c.predeployDocker()
	c.predeployK8s()
	c.Log().Info("All checks passed")
}

func (c *Checker) elasticReport() {
	e, err := elastic.New(c, c.ElasticSearchURL)
	if err != nil {
		c.Log().Fatal(err)
	}
	e.Notify(c.Apps, "deploy_log", c.User, c.KubeNamespace)
}
