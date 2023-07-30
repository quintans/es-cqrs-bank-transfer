package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/google/uuid"
	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain/entity"
	"github.com/quintans/eventsourcing/log"
	"github.com/quintans/eventsourcing/projection"
	"github.com/quintans/faults"
)

const (
	index         = "balance"
	hitsField     = "hits"
	ownerField    = "owner"
	sequenceField = "sequence"
	source        = "_source"
	hits          = "hits"
)

type GetResponse struct {
	ID      uuid.UUID   `json:"_id"`
	Version int64       `json:"_version"`
	Source  interface{} `json:"_source"`
}

type SearchResponse struct {
	Hits Hits `json:"hits"`
}

type Hits struct{}

type BalanceRepository struct {
	client *elasticsearch.Client
	logger log.Logger
}

func NewBalanceRepository(logger log.Logger, client *elasticsearch.Client) BalanceRepository {
	return BalanceRepository{
		client: client,
		logger: logger,
	}
}

func (b BalanceRepository) GetAllOrderByOwnerAsc(ctx context.Context) ([]entity.Balance, error) {
	req := esapi.SearchRequest{
		Index: []string{index},
		Sort:  []string{ownerField + ":asc"},
	}
	res, err := req.Do(ctx, b.client)
	if err != nil {
		return []entity.Balance{}, faults.Errorf("Error getting response for SearchRequest: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, faults.Errorf("[%s] Error GetAllOrderByOwnerAsc", res.Status())
	}

	return responseToBalances(res)
}

func responseToBalances(res *esapi.Response) ([]entity.Balance, error) {
	mapResp := map[string]interface{}{}
	if err := json.NewDecoder(res.Body).Decode(&mapResp); err != nil {
		return nil, faults.Errorf("Error parsing SearchRequest response body: %s", err)
	}
	balances := []entity.Balance{}
	// Iterate the document "hits" returned by API call
	for _, hit := range mapResp[hits].(map[string]interface{})[hits].([]interface{}) {
		doc := hit.(map[string]interface{})
		b, err := sourceToBalance(doc)
		if err != nil {
			return nil, err
		}
		balances = append(balances, b)
	}
	return balances, nil
}

func sourceToBalance(doc map[string]interface{}) (entity.Balance, error) {
	source := doc[source].(map[string]interface{})
	b := entity.Balance{}
	var err error
	b.ID, err = uuid.Parse(doc["_id"].(string))
	if err != nil {
		return entity.Balance{}, faults.Wrap(err)
	}
	if v, ok := doc["_version"]; ok {
		b.Version = int64(v.(float64))
	}
	if v, ok := source[sequenceField]; ok {
		b.Sequence = uint64(v.(float64))
	}
	if v, ok := source["kind"]; ok {
		b.Owner = v.(string)
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
	return b, nil
}

func (b BalanceRepository) GetMaxSequence(ctx context.Context) (projection.Token, error) {
	s := `{
		"sort": [
		  {
			"sequence": { "order": "desc"}
		  }
		]
	  }`

	size := 1
	req := esapi.SearchRequest{
		Index: []string{index},
		Body:  strings.NewReader(s),
		Size:  &size,
	}
	res, err := req.Do(ctx, b.client)
	if err != nil {
		return projection.Token{}, faults.Errorf("Error getting response for SearchRequest: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return projection.Token{}, faults.Errorf("[%s] Error GetMaxEventID", res.Status())
	}

	balances, err := responseToBalances(res)
	if err != nil {
		return projection.Token{}, err
	}
	if len(balances) > 0 {
		return projection.NewToken(balances[0].Kind, balances[0].Sequence), nil
	}
	return projection.NewToken(projection.CatchUpToken, 0), nil
}

func (b BalanceRepository) CreateAccount(ctx context.Context, balance entity.Balance) error {
	// we don't want to repeat the ID and version values in the doc
	docID := balance.ID
	balance.ID = uuid.Nil
	balance.Version = 0

	var s strings.Builder
	if err := json.NewEncoder(&s).Encode(balance); err != nil {
		return err
	}

	req := esapi.IndexRequest{
		Index:      index,
		DocumentID: docID.String(),
		Body:       strings.NewReader(s.String()),
		Refresh:    "true",
	}

	res, err := req.Do(ctx, b.client)
	if err != nil {
		return faults.Errorf("Error getting response for IndexRequest: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return faults.Errorf("[%s] Error CreatAccount document ID=%s", res.Status(), docID)
	}

	return nil
}

func (b BalanceRepository) GetByID(ctx context.Context, aggregateID uuid.UUID) (entity.Balance, error) {
	req := esapi.GetRequest{
		Index:      index,
		DocumentID: aggregateID.String(),
	}
	res, err := req.Do(ctx, b.client)
	if err != nil {
		return entity.Balance{}, faults.Errorf("Error getting response for GetRequest: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return entity.Balance{}, nil
	}

	if res.IsError() {
		return entity.Balance{}, faults.Errorf("[%s] Error getting document ID=%s", res.Status(), aggregateID)
	}
	// Deserialize the response into a map.
	r := GetResponse{
		Source: &entity.Balance{},
	}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return entity.Balance{}, faults.Errorf("Error parsing the response body for GetRequest: %w", err)
	}
	balance := r.Source.(*entity.Balance)
	balance.ID = r.ID
	balance.Version = r.Version
	return *balance, nil
}

func (b BalanceRepository) Update(ctx context.Context, balance entity.Balance) error {
	// we don't want to repeat the ID and version values in the doc
	docID := balance.ID
	balance.ID = uuid.Nil
	balance.Version = 0

	var s strings.Builder
	s.WriteString(`{"doc":`)
	if err := json.NewEncoder(&s).Encode(balance); err != nil {
		return err
	}
	s.WriteString("}")

	req := esapi.UpdateRequest{
		Index:      index,
		DocumentID: docID.String(),
		Body:       strings.NewReader(s.String()),
		Refresh:    "true",
	}

	res, err := req.Do(ctx, b.client)
	if err != nil {
		return faults.Errorf("Error getting response for UpdateRequest(balance): %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return faults.Errorf("[%s] Error updating document ID=%s", res.Status(), docID)
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
			return nil, faults.Errorf("Path not found: %s", k)
		}
		current = v.(map[string]interface{})
	}
	return current[fields[size-1]], nil
}
