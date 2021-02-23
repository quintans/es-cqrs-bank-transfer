package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type AccountDTO struct {
	Owner   string `json:"owner,omitempty"`
	Balance int64  `json:"balance,omitempty"`
	Status  string `json:"status,omitempty"`
}

type BalanceDTO struct {
	ID      string `json:"id"`
	Status  string `json:"status,omitempty"`
	Balance int64  `json:"balance,omitempty"`
	Owner   string `json:"owner,omitempty"`
}

const (
	accountsUBaseRL = "http://localhost:8000"
	accountsURL     = accountsUBaseRL + "/accounts"
	balanceURL      = "http://localhost:8030/balance"
)

func TestNoTransfer(t *testing.T) {
	id, err := createUserAccount("Paulo")
	require.NoError(t, err)
	require.NotEmpty(t, id)

	account, err := getUserAccount(id)
	require.NoError(t, err)
	require.Equal(t, AccountDTO{
		Owner:  "Paulo",
		Status: "OPEN",
	}, account)

	var balance BalanceDTO
	time.Sleep(time.Second)
	balance, err = getUserBalance(id)

	require.NoError(t, err)
	require.Equal(t, BalanceDTO{
		ID:     id,
		Owner:  "Paulo",
		Status: "OPEN",
	}, balance)

	// deposit
	txID, err := transact("", id, 100)
	require.NoError(t, err)
	require.NotEmpty(t, txID)

	time.Sleep(time.Second)
	balance, err = getUserBalance(id)
	require.NoError(t, err)
	require.Equal(t, BalanceDTO{
		ID:      id,
		Owner:   "Paulo",
		Status:  "OPEN",
		Balance: 100,
	}, balance)

	// withdraw
	txID, err = transact(id, "", 20)
	require.NoError(t, err)
	require.NotEmpty(t, txID)

	time.Sleep(time.Second)
	balance, err = getUserBalance(id)
	require.NoError(t, err)
	require.Equal(t, BalanceDTO{
		ID:      id,
		Owner:   "Paulo",
		Status:  "OPEN",
		Balance: 80,
	}, balance)
}

func TestTransfer(t *testing.T) {
	id1, err := createUserAccount("Paulo")
	require.NoError(t, err)
	require.NotEmpty(t, id1)

	// deposit
	txID, err := transact("", id1, 100)
	require.NoError(t, err)
	require.NotEmpty(t, txID)

	id2, err := createUserAccount("Quintans")
	require.NoError(t, err)
	require.NotEmpty(t, id2)

	txID, err = transact(id1, id2, 20)
	require.NoError(t, err)
	require.NotEmpty(t, txID)

	time.Sleep(time.Second)
	balance, err := getUserBalance(id1)
	require.NoError(t, err)
	require.Equal(t, BalanceDTO{
		ID:      id1,
		Owner:   "Paulo",
		Status:  "OPEN",
		Balance: 80,
	}, balance)

	balance, err = getUserBalance(id2)
	require.NoError(t, err)
	require.Equal(t, BalanceDTO{
		ID:      id2,
		Owner:   "Quintans",
		Status:  "OPEN",
		Balance: 20,
	}, balance)
}

func createUserAccount(name string) (string, error) {
	payload := fmt.Sprintf(`{"owner":"%s"}`, name)
	var id string
	err := httpDo(http.MethodPost, accountsURL, []byte(payload), http.StatusCreated, &id)
	if err != nil {
		return "", err
	}

	return id, nil
}

func getUserAccount(id string) (AccountDTO, error) {
	account := AccountDTO{}
	err := httpDo(http.MethodGet, accountsURL+"/"+id, nil, http.StatusOK, &account)
	if err != nil {
		return AccountDTO{}, err
	}

	return account, nil
}

func getUserBalance(id string) (BalanceDTO, error) {
	balance := BalanceDTO{}
	err := httpDo(http.MethodGet, balanceURL+"/"+id, nil, http.StatusOK, &balance)
	if err != nil {
		return BalanceDTO{}, err
	}

	return balance, nil
}

func transact(from, to string, money int64) (string, error) {
	var id string
	payload := fmt.Sprintf(`{"from":"%s", "to":"%s", "money": %d}`, from, to, money)
	err := httpDo(http.MethodPost, accountsUBaseRL+"/transactions", []byte(payload), http.StatusCreated, &id)
	if err != nil {
		return "", err
	}

	return id, nil
}

func httpDo(method, url string, payload []byte, status int, target interface{}) error {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}

	// set the request header Content-Type for json
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	// initialize http client
	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != status {
		return fmt.Errorf("Expecting status %d, got %d", status, resp.StatusCode)
	}

	if target != nil {
		err = json.NewDecoder(resp.Body).Decode(target)
		if err != nil {
			return err
		}
	}

	return nil
}
