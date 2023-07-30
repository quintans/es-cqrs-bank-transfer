package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/quintans/faults"

	"github.com/quintans/eventsourcing/projection"
)

var _ projection.ResumeStore = (*ProjectionResume)(nil)

type GetESResponse struct {
	ID      string      `json:"_id"`
	Version int64       `json:"_version"`
	Source  interface{} `json:"_source"`
}

type ProjectionResumeRow struct {
	ID    string `json:"id"`
	Token string `json:"token"`
}

type ProjectionResume struct {
	client *elasticsearch.Client
	index  string
}

func NewProjectionResume(client *elasticsearch.Client, index string) ProjectionResume {
	return ProjectionResume{
		client: client,
		index:  index,
	}
}

func (es ProjectionResume) GetStreamResumeToken(ctx context.Context, key projection.ResumeKey) (projection.Token, error) {
	req := esapi.GetRequest{
		Index:      es.index,
		DocumentID: key.String(),
	}
	res, err := req.Do(ctx, es.client)
	if err != nil {
		return projection.Token{}, faults.Errorf("Error getting response for GetRequest: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return projection.Token{}, faults.Wrap(projection.ErrResumeTokenNotFound)
	}

	if res.IsError() {
		return projection.Token{}, faults.Errorf("[%s] Error getting document ID=%s", res.Status(), key)
	}
	// Deserialize the response into a map.
	r := GetESResponse{
		Source: &ProjectionResumeRow{},
	}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return projection.Token{}, faults.Errorf("Error parsing the response body for GetRequest: %w", err)
	}
	row := r.Source.(*ProjectionResumeRow)

	return projection.ParseToken(row.Token)
}

func (es ProjectionResume) SetStreamResumeToken(ctx context.Context, key projection.ResumeKey, token projection.Token) error {
	res, err := es.client.Update(
		es.index,
		key.String(),
		strings.NewReader(fmt.Sprintf(`{
		  "doc": {
			"token": "%s"
		  },
		  "doc_as_upsert": true
		}`, token.String())),
	)
	if err != nil {
		return faults.Errorf("Error getting elastic search response: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return faults.Errorf("[%s] Error updating elastic search index '%s' %s=%s", res.Status(), es.index, key, token)
	}
	return nil
}
