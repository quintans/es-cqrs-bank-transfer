package main_test

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/quintans/faults"
	testcontainers "github.com/testcontainers/testcontainers-go"
)

const elasticInit = `
{
  "mappings": {
    "properties": {
      "event_id": {
        "type": "keyword"
      },
      "owner": {
        "type": "keyword"
      }
    }
  }
}`

func TestMain(m *testing.M) {
	ctx := context.Background()

	var once sync.Once
	destroy, err := dockerCompose(ctx)
	if err != nil {
		destroy()
		log.Fatal(err)
	}
	defer func() {
		once.Do(destroy)
	}()

	initElastic()

	waitForServers()

	// test run
	code := m.Run()
	once.Do(destroy)
	os.Exit(code)
}

func dockerCompose(ctx context.Context) (func(), error) {
	path := "./docker-compose.yml"

	compose := testcontainers.NewLocalDockerCompose([]string{path}, "cqrs-set")
	destroyFn := func() {
		exErr := compose.Down()
		if err := checkIfError(exErr); err != nil {
			log.Printf("Error on compose shutdown: %v\n", err)
		}
	}

	exErr := compose.Down()
	if err := checkIfError(exErr); err != nil {
		return func() {}, err
	}
	exErr = compose.
		WithCommand([]string{"up", "--build", "-d"}).
		Invoke()
	err := checkIfError(exErr)
	if err != nil {
		return destroyFn, err
	}

	return destroyFn, err
}

func checkIfError(err testcontainers.ExecError) error {
	if err.Error != nil {
		return faults.Errorf("Failed when running %v: %v", err.Command, err.Error)
	}

	if err.Stdout != nil {
		return faults.Errorf("An error in Stdout happened when running %v: %v", err.Command, err.Stdout)
	}

	if err.Stderr != nil {
		return faults.Errorf("An error in Stderr happened when running %v: %v", err.Command, err.Stderr)
	}
	return nil
}

func initElastic() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var err error
	var i int
	for i = 0; i < 10; i++ {
		err = httpDo(http.MethodGet, "http://localhost:9200/", nil, http.StatusOK, nil)
		if err != nil {
			log.Printf("Unable to ping elasticsearch: %v", err)
			<-ticker.C
		}
	}

	if err != nil {
		log.Fatalf("Error pinging elasticsearch: %s", err)
	}

	// set the HTTP method, url, and request body
	req, err := http.NewRequest(http.MethodPut, "http://localhost:9200/balance", bytes.NewBuffer([]byte(elasticInit)))
	if err != nil {
		log.Fatal(err)
	}

	// set the request header Content-Type for json
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	// initialize http client
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Expecting status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func waitForServers() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var err error
	var i int
	for i = 0; i < 5; i++ {
		err = httpDo(http.MethodGet, "http://localhost:8000/", nil, http.StatusOK, nil)
		if err != nil {
			log.Printf("Unable to ping account service: %v", err)
			<-ticker.C
		}
	}

	if err != nil {
		log.Fatalf("Error pinging account server: %v", err)
	}

	for x := i; x < 5; x++ {
		err = httpDo(http.MethodGet, "http://localhost:8030/", nil, http.StatusOK, nil)
		if err != nil {
			log.Printf("Unable to ping balance service: %v", err)
			<-ticker.C
		}
	}

	if err != nil {
		log.Fatalf("Error pinging balance server: %v", err)
	}
}
