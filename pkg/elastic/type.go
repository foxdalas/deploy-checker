package elastic

import (
	"github.com/foxdalas/deploy-checker/pkg/checker_const"
	elastic "github.com/olivere/elastic/v7"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"time"
)

type elasticSearch struct {
	checker checker.Checker
	log     *logrus.Entry

	ctx context.Context

	client *elastic.Client
	index  string
}

type document struct {
	Timestamp  time.Time `json:"@timestamp"`
	User       string    `json:"user"`
	Namespace  string    `json:"namespace"`
	Msg        string    `json:"msg"`
	Tags       string    `json:"tags"`
	Build      string    `json:"build"`
	Datacenter string    `json:"datacenter"`
	Annotags   string    `json:"annotags"`
	Apps       []string  `json:"apps"`
	Production string    `json:"production"`
}

type EsRetrier struct {
	backoff elastic.Backoff
}
