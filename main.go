package main

import (
	"github.com/heroku/docker-registry-client/registry"
	"flag"
	"os"
	"errors"
	"github.com/foxdalas/traefik/log"
)

type Checker struct  {
	docker	docker
}

type docker struct {
	repository string
	tag string

	usename string
	password string
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

func params (c *Checker) error {
	flag.StringVar(&c.docker.repository, "repository", "", "Docker repository")
	flag.StringVar(&c.docker.tag, "tag", "", "Docker repository tag")

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
	return nil
}

func (c *Checker) connectToRegistry() (*registry.Registry, error) {
	url      := "https://registry-1.docker.io/"
	username := c.docker.usename // anonymous
	password := c.docker.password // anonymous
	hub, err := registry.New(url, username, password)

	return hub, err
}

func (c *Checker) getTags(hub *registry.Registry) ([]string, error)  {
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