package docker

import (
	"github.com/foxdalas/docker-registry-client/registry"
	log "github.com/sirupsen/logrus"
	"os"
)

func New(username string, password string, log log.Entry) (*docker, error) {
	url := "https://registry-1.docker.io/"
	hub, err := registry.New(url, username, password, log.Infof)
	if err != nil {
		return nil, err
	}

	return &docker{
		registry:   hub,
	}, nil
}

func (d *docker) getTags(image string) ([]string, error) {
	tags, err := d.registry.Tags(image)
	return tags, err
}

func (d *docker) IsDockerImageExist(image string, tag string) bool {
	tags, err := d.getTags(image)
	if err != nil {
		log.Error(err)
		os.Exit(1)

	}
	if d.isDockerTagExist(tags, tag) {
		return true
	}
	return false
}

func (d *docker) isDockerTagExist(tags []string, tag string) bool {
	return contains(tags, tag)
}

func contains(slice []string, item string) bool {
	set := make(map[string]struct{}, len(slice))
	for _, s := range slice {
		set[s] = struct{}{}
	}

	_, ok := set[item]
	return ok
}
