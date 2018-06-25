package elastic

import (
	"context"
	"errors"
	"fmt"
	"github.com/foxdalas/deploy-checker/pkg/checker_const"
	"github.com/olivere/elastic"
	log "github.com/sirupsen/logrus"
	"net/http"
	"syscall"
	"time"
)

func New(checker checker.Checker, elasticHost string) (*elasticSearch, error) {
	ctx := context.Background()

	client, err := elastic.NewClient(
		elastic.SetSniff(false),
		elastic.SetURL(elasticHost),
		elastic.SetRetrier(NewEsRetrier()),
	)
	if err != nil {
		return nil, err
	}

	_, _, err = client.Ping(elasticHost).Do(ctx)
	if err != nil {
		return nil, err
	}

	return &elasticSearch{
		checker: checker,
		client:  client,
		index:   "lita" + "-" + time.Now().Local().Format("2006.01.02"),
		ctx:     ctx,
	}, nil
}

func (e *elasticSearch) sendDocument(apps string, tags string, user string, namespace string) {
	msg := fmt.Sprintf("Deploy apps %s in namespace %s", apps, namespace)
	message := &document{
		Timestamp: time.Now().UTC(),
		User:      user,
		Msg:       msg,
		Tags:      tags,
	}

	_, err := e.client.Index().Index(e.index).Type("lita").BodyJson(message).Do(e.ctx)
	if err != nil {
		e.Log().Error(err)
	}
}

func (e *elasticSearch) Notify(apps string, tags string, user string, namespace string) {
	exist, err := e.isIndexExist()
	if err != nil {
		e.Log().Fatal(err)
	}
	if exist {
		e.sendDocument(apps, tags, user, namespace)
	} else {
		e.Log().Infof("Index %s in not exists", e.index)
		e.createIndex()
		e.sendDocument(apps, tags, user, namespace)
	}
}

type EsRetrier struct {
	backoff elastic.Backoff
}

func NewEsRetrier() *EsRetrier {
	return &EsRetrier{
		backoff: elastic.NewExponentialBackoff(10*time.Millisecond, 8*time.Second),
	}
}

func (r *EsRetrier) Retry(ctx context.Context, retry int, req *http.Request, resp *http.Response, err error) (time.Duration, bool, error) {
	// Fail hard on a specific error
	if err == syscall.ECONNREFUSED {
		return 0, false, errors.New("Elasticsearch or network down")
	}

	// Stop after 5 retries
	if retry >= 5 {
		return 0, false, nil
	}

	// Let the backoff strategy decide how long to wait and whether to stop
	wait, stop := r.backoff.Next(retry)
	return wait, stop, nil
}

func (e *elasticSearch) isIndexExist() (bool, error) {
	return e.client.IndexExists(e.index).Do(e.ctx)
}

func (e *elasticSearch) Log() *log.Entry {
	return e.checker.Log().WithField("context", "elasticsearch")
}

func (e *elasticSearch) createIndex() {
	mapping := `{
		"mappings": {
			"lita": {
				"properties": {
					"@timestamp": {
						"type": "date"
					},
					"msg": {
						"type": "text",
						"fields": {
							"keyword": {
								"type": "keyword",
								"ignore_above": 256
							}
						}
					},
					"tags": {
						"type": "text",
						"fields": {
							"keyword": {
								"type": "keyword",
								"ignore_above": 256
							}
						}
					},
					"timestamp": {
						"type": "long"
					},
					"user": {
						"type": "text",
						"fields": {
							"keyword": {
								"type": "keyword",
								"ignore_above": 256
							}
						}
					}
				}
			}
		}
	}`
	createIndex, err := e.client.CreateIndex(e.index).BodyString(mapping).Do(e.ctx)
	if err != nil {
		e.Log().Fatal(err)
	}
	if createIndex.Acknowledged {
		e.Log().Infof("Create index %s done", e.index)
	}
}