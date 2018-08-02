package docker

import (
	"github.com/foxdalas/docker-registry-client/registry"
)

type docker struct {
	registry   *registry.Registry
	repository string
	tag        string
}
