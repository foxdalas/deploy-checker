package elastic

import (
	"context"
	"errors"
	"fmt"
	"github.com/foxdalas/deploy-checker/pkg/checker_const"
	elastic "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"
)

func New(checker checker.Checker, elasticHost []string) (*elasticSearch, error) {

	client, err := elastic.NewClient(
		elastic.SetURL(elasticHost...),
		elastic.SetSniff(false),
		elastic.SetRetrier(NewEsRetrier()),
		elastic.SetHealthcheck(true),
		elastic.SetHealthcheckTimeout(time.Second*60),
		elastic.SetErrorLog(checker.Log()),
		elastic.SetInfoLog(checker.Log()),
	)
	if err != nil {
		return nil, err
	}

	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)

	return &elasticSearch{
		checker: checker,
		client:  client,
		index:   "lita" + "-" + time.Now().Local().Format("2006.01.02"),
		ctx:     ctx,
	}, nil
}

func (e *elasticSearch) sendDocument(apps string, tags string, user string, namespace string, build string) {
	msg := fmt.Sprintf("Deploy apps %s with build %s in namespace %s", apps, build, namespace)
	datacenter := os.Getenv("DATACENTER")
	production := "false"
	switch datacenter {
	case "dev":
		production = "false"
	case "testing":
		production = "false"
	default:
		production = "true"
	}
	annotags := user + "," + datacenter

	message := &document{
		Timestamp:  time.Now().UTC(),
		User:       user,
		Namespace:  namespace,
		Msg:        msg,
		Tags:       tags,
		Build:      build,
		Annotags:   annotags,
		Datacenter: os.Getenv("DATACENTER"),
		Apps:       strings.Split(apps, ","),
		Production: production,
	}

	_, err := e.client.Index().Index(e.index).Type("doc").BodyJson(message).Do(e.ctx)
	if err != nil {
		e.Log().Error(err)
	}
}

func (e *elasticSearch) Notify(apps string, tags string, user string, namespace string, build string) {
	e.sendDocument(apps, tags, user, namespace, build)
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
