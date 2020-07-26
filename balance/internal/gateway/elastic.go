package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain/entity"
)

const (
	index      = "balance"
	hitsField  = "hits"
	ownerField = "owner"
	source     = "_source"
	hits       = "hits"
)

type GetResponse struct {
	ID      string      `json:"_id"`
	Version int64       `json:"_version"`
	Source  interface{} `json:"_source"`
}

type SearchResponse struct {
	Hits Hits `json:"hits"`
}

type Hits struct {
}

type BalanceRepository struct {
	Client *elasticsearch.Client
}

func (b BalanceRepository) GetAllOrderByOwnerAsc(ctx context.Context) ([]entity.Balance, error) {
	req := esapi.SearchRequest{
		Index: []string{index},
		Sort:  []string{ownerField + ".keyword:asc"},
	}
	res, err := req.Do(ctx, b.Client)
	if err != nil {
		return []entity.Balance{}, fmt.Errorf("Error getting response for SearchRequest: %w", err)
	}
	defer res.Body.Close()

	mapResp := map[string]interface{}{}
	if err := json.NewDecoder(res.Body).Decode(&mapResp); err != nil {
		log.Fatalf("Error parsing SearchRequest response body: %s", err)
	}
	balances := []entity.Balance{}
	// Iterate the document "hits" returned by API call
	for _, hit := range mapResp[hits].(map[string]interface{})[hits].([]interface{}) {
		doc := hit.(map[string]interface{})
		balances = append(balances, sourceToBalance(doc))
	}

	return balances, nil
}

func sourceToBalance(doc map[string]interface{}) entity.Balance {
	source := doc[source].(map[string]interface{})
	b := entity.Balance{}
	b.ID = doc["_id"].(string)
	if v, ok := doc["_version"]; ok {
		b.Version = int64(v.(float64))
	}
	if v, ok := source["balance"]; ok {
		b.Balance = int64(v.(float64))
	}
	if v, ok := source["owner"]; ok {
		b.Owner = v.(string)
	}
	return b
}

func (b BalanceRepository) GetEventID(ctx context.Context, aggregateID string) (string, error) {
	balance, err := b.GetByID(ctx, aggregateID)
	if err != nil {
		return "", err
	}
	return balance.EventID, nil
}

func (b BalanceRepository) CreateAccount(ctx context.Context, balance entity.Balance) error {
	// we don't want to repeat the ID and version values in the doc
	docID := balance.ID
	balance.ID = ""
	balance.Version = 0

	var s strings.Builder
	if err := json.NewEncoder(&s).Encode(balance); err != nil {
		return err
	}

	req := esapi.IndexRequest{
		Index:      index,
		DocumentID: docID,
		Body:       strings.NewReader(s.String()),
		Refresh:    "true",
	}

	res, err := req.Do(ctx, b.Client)
	if err != nil {
		return fmt.Errorf("Error getting response for IndexRequest: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("[%s] Error indexing document ID=%s", res.Status(), balance.ID)
	}
	return nil
}

func (b BalanceRepository) GetByID(ctx context.Context, aggregateID string) (entity.Balance, error) {
	req := esapi.GetRequest{
		Index:      index,
		DocumentID: aggregateID,
	}
	res, err := req.Do(ctx, b.Client)
	if err != nil {
		return entity.Balance{}, fmt.Errorf("Error getting response for GetRequest: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return entity.Balance{}, nil
	}

	if res.IsError() {
		return entity.Balance{}, fmt.Errorf("[%s] Error getting document ID=%s", res.Status(), aggregateID)
	}
	// Deserialize the response into a map.
	r := GetResponse{
		Source: &entity.Balance{},
	}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return entity.Balance{}, fmt.Errorf("Error parsing the response body for GetRequest: %w", err)
	}
	balance := r.Source.(*entity.Balance)
	balance.ID = r.ID
	balance.Version = r.Version
	return *balance, nil
}

func (b BalanceRepository) Update(ctx context.Context, balance entity.Balance) error {
	// we don't want to repeat the ID and version values in the doc
	docID := balance.ID
	balance.ID = ""
	balance.Version = 0

	var s strings.Builder
	if err := json.NewEncoder(&s).Encode(balance); err != nil {
		return err
	}

	req := esapi.UpdateRequest{
		Index:      index,
		DocumentID: docID,
		Body:       strings.NewReader(s.String()),
		Refresh:    "true",
	}

	res, err := req.Do(ctx, b.Client)
	if err != nil {
		return fmt.Errorf("Error getting response for UpdateRequest(balance): %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("[%s] Error indexing document ID=%s", res.Status(), balance.ID)
	}
	return nil
}
