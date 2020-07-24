package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain/entity"
)

const (
	index = "balance"
)

type esResponse struct {
	ID      string      `json:"_id"`
	Version int64       `json:"_version"`
	Source  interface{} `json:"_source"`
}

type BalanceRepository struct {
	Client *elasticsearch.Client
}

func (b BalanceRepository) GetEventID(ctx context.Context, aggregateID string) (string, error) {
	req := esapi.GetRequest{
		Index:      index,
		DocumentID: aggregateID,
	}
	res, err := req.Do(ctx, b.Client)
	if err != nil {
		return "", fmt.Errorf("Error getting response for GetRequest: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return "", fmt.Errorf("[%s] Error getting document ID=%s", res.Status(), aggregateID)
	}
	// Deserialize the response into a map.
	r := esResponse{
		Source: &entity.Balance{},
	}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return "", fmt.Errorf("Error parsing the response body for GetRequest: %w", err)
	}
	balance := r.Source.(*entity.Balance)
	return balance.EventID, nil
}

func (b BalanceRepository) CreateAccount(ctx context.Context, balance entity.Balance) error {
	var s strings.Builder
	if err := json.NewEncoder(&s).Encode(balance); err != nil {
		return err
	}

	req := esapi.IndexRequest{
		Index:      index,
		DocumentID: balance.ID,
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

func (b BalanceRepository) Update(ctx context.Context, balance entity.Balance) error {
	var s strings.Builder
	if err := json.NewEncoder(&s).Encode(balance); err != nil {
		return err
	}

	req := esapi.UpdateRequest{
		Index:      index,
		DocumentID: balance.ID,
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
