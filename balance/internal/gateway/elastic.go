package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain/entity"
	log "github.com/sirupsen/logrus"
)

const (
	index        = "balance"
	hitsField    = "hits"
	ownerField   = "owner"
	eventIDField = "event_id"
	source       = "_source"
	hits         = "hits"
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
		Sort:  []string{ownerField + ":asc"},
	}
	res, err := req.Do(ctx, b.Client)
	if err != nil {
		return []entity.Balance{}, fmt.Errorf("Error getting response for SearchRequest: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("[%s] Error GetAllOrderByOwnerAsc", res.Status())
	}

	balances := responseToBalances(res)
	return balances, nil
}

func responseToBalances(res *esapi.Response) []entity.Balance {
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
	return balances
}

func sourceToBalance(doc map[string]interface{}) entity.Balance {
	source := doc[source].(map[string]interface{})
	b := entity.Balance{}
	b.ID = doc["_id"].(string)
	if v, ok := doc["_version"]; ok {
		b.Version = int64(v.(float64))
	}
	if v, ok := source["event_id"]; ok {
		b.EventID = v.(string)
	}
	if v, ok := source["partition"]; ok {
		b.Partition = uint32(v.(float64))
	}
	if v, ok := source["status"]; ok {
		b.Status = event.Status(v.(string))
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

func (b BalanceRepository) GetMaxEventID(ctx context.Context, partition int) (string, error) {
	s := fmt.Sprintf(`{
		"sort": [
		  {
			"event_id": { "order": "desc"}
		  }
		],
		"query" : {
			"term" : { "partition" : %d }
		}
	  }`, partition)

	size := 1
	req := esapi.SearchRequest{
		Index: []string{index},
		Body:  strings.NewReader(s),
		Size:  &size,
	}
	res, err := req.Do(ctx, b.Client)
	if err != nil {
		return "", fmt.Errorf("Error getting response for SearchRequest: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return "", fmt.Errorf("[%s] Error GetMaxEventID", res.Status())
	}

	balances := responseToBalances(res)
	if len(balances) > 0 {
		return balances[0].EventID, nil
	}
	return "", nil
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
		return fmt.Errorf("[%s] Error CreatAccount document ID=%s", res.Status(), docID)
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
	s.WriteString(`{"doc":`)
	if err := json.NewEncoder(&s).Encode(balance); err != nil {
		return err
	}
	s.WriteString("}")

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
		return fmt.Errorf("[%s] Error updating document ID=%s", res.Status(), docID)
	}
	return nil
}

func (b BalanceRepository) ClearAllData(ctx context.Context) error {
	// delete all docs
	refresh := true
	del := esapi.DeleteByQueryRequest{
		Index: []string{index},
		Body: strings.NewReader(`{
			"query" : { 
				"match_all" : {}
			}
		}`),
		Refresh: &refresh,
	}
	resDel, err := del.Do(ctx, b.Client)
	if err != nil {
		return fmt.Errorf("Error getting response when deleting docs ClearAllData(balance): %w", err)
	}
	defer resDel.Body.Close()
	if resDel.IsError() {
		return fmt.Errorf("[%s] Error deleting docs", resDel.Status())
	}

	return nil
}

func walkMap(m map[string]interface{}, path string) (interface{}, error) {
	fields := strings.Split(path, ".")
	current := m
	size := len(fields)
	for i := 0; i < size-1; i++ {
		k := fields[i]
		v := current[k]
		if v == nil {
			return nil, fmt.Errorf("Path not found: %s", k)
		}
		current = v.(map[string]interface{})
	}
	return current[fields[size-1]], nil
}
