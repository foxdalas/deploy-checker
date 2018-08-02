package docker

import (
	"github.com/foxdalas/docker-registry-client/registry"
	log "github.com/sirupsen/logrus"
	"os"
)

func New(username string, password string, repository string, tag string, log log.Entry) (*docker, error) {
	url := "https://registry-1.docker.io/"
	hub, err := registry.New(url, username, password, log.Infof)
	if err != nil {
		return nil, err
	}

	return &docker{
		registry:   hub,
		repository: repository,
		tag:        tag,
	}, nil
}

func (d *docker) getTags() ([]string, error) {
	tags, err := d.registry.Tags(d.repository)
	return tags, err
}

func (d *docker) IsDockerImageExist() bool {
	tags, err := d.getTags()
	if err != nil {
		log.Error(err)
		os.Exit(1)

	}
	if d.isDockerTagExist(tags) {
		return true
	}
	return false
}

func (d *docker) isDockerTagExist(tags []string) bool {
	return contains(tags, d.tag)
}

func contains(slice []string, item string) bool {
	set := make(map[string]struct{}, len(slice))
	for _, s := range slice {
		set[s] = struct{}{}
	}

	_, ok := set[item]
	return ok
}
