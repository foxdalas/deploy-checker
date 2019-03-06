package k8s

import (
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"os"
	"path/filepath"
	"strings"
)

func (k *k8s) findDeployments(searchDir string) []string {
	fileList := []string{}
	err := filepath.Walk(searchDir, func(path string, f os.FileInfo, err error) error {
		if strings.Contains(path, "deployment.yml") {
			fileList = append(fileList, path)
			k.Log().Infof("Founded deployment file %s", path)
		}
		return nil
	})
	if err != nil {
		k.Log().Error(err)
	}

	return fileList
}

func (k *k8s) writeDeploymentFile(path string) {
	f, err := os.Create(path)
	if err != nil {
		k.Log().Fatal(err)
	}

	defer f.Close()

	k.Log().Infof("Updating file %s", path)
	s := json.NewYAMLSerializer(json.DefaultMetaFactory, nil, nil)
	err = s.Encode(k.yamlDeployment, f)
	if err != nil {
		k.Log().Fatal(err)
	}
}

func (k *k8s) Log() *log.Entry {
	return k.checker.Log().WithField("context", "k8s")
}
