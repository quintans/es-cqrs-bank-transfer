package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/caarlos0/env/v6"
	"github.com/nats-io/stan.go"
	"github.com/quintans/eventstore/sink"
	"github.com/quintans/eventstore/store"
	"github.com/quintans/eventstore/store/mongodb"
	"github.com/quintans/eventstore/worker"
	"github.com/quintans/toolkit/locks"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	ConsulAddress  string        `env:"CONSUL_ADDRESS"`
	LockExpiry     time.Duration `env:"LOCK_EXPIRY" envDefault:"10s"`
	PartitionSlots []string      `env:"PARTITION_SLOTS" envSeparator:","`
	ConfigEs
	ConfigNats
}

type ConfigEs struct {
	EsUser     string `env:"ES_USER" envDefault:"root"`
	EsPassword string `env:"ES_PASSWORD" envDefault:"password"`
	EsHost     string `env:"ES_HOST"`
	EsPort     int    `env:"ES_PORT" envDefault:"27017"`
	EsName     string `env:"ES_NAME" envDefault:"accounts"`
}

type ConfigNats struct {
	NatsAddress string `env:"NATS_ADDRESS"`
	Topic       string `env:"TOPIC" envDefault:"accounts"`
}

func init() {
	log.SetOutput(os.Stdout)
	log.SetFormatter(&log.TextFormatter{
		DisableQuote: true,
	})
}

func main() {
	cfg := &Config{}
	err := env.Parse(cfg)
	if err != nil {
		log.Fatal(err)
	}

	partitionSlots, err := worker.ParseSlots(cfg.PartitionSlots)
	if err != nil {
		log.Fatal(err)
	}
	var partitions uint32
	for _, v := range partitionSlots {
		if v.To > partitions {
			partitions = v.To
		}
	}

	slotLen := len(partitionSlots)
	sinkers := make([]*sink.NatsSink, slotLen)
	for i := 0; i < slotLen; i++ {
		idx := strconv.Itoa(i)
		sinkers[i] = sink.NewNatsSink(cfg.ConfigNats.Topic, partitions, "test-cluster", "pusher-id-"+idx, stan.NatsURL(cfg.ConfigNats.NatsAddress))
	}
	defer func() {
		for _, s := range sinkers {
			s.Close()
		}
	}()

	dbURL := fmt.Sprintf("mongodb://%s:%d/%s?replicaSet=rs0", cfg.EsHost, cfg.EsPort, cfg.EsName)
	pool, err := locks.NewConsulLockPool(cfg.ConsulAddress)
	if err != nil {
		log.Fatal("Error instantiating Locker: %v", err)
	}

	workers := make([]worker.Worker, slotLen)
	for i, v := range partitionSlots {
		listener, err := mongodb.NewFeed(dbURL, cfg.EsName, mongodb.WithPartitions(partitions, v.From, v.To))
		if err != nil {
			log.Fatalf("Error instantiating store listener: %v", err)
		}

		idx := strconv.Itoa(i)
		workers[i] = worker.NewRunWorker(
			"mongodb-NATS-feeder-"+idx,
			pool.NewLock("forwarder-lock-"+idx, cfg.LockExpiry),
			store.NewForwarder(
				listener,
				sinkers[i],
			))
	}

	ctx, cancel := context.WithCancel(context.Background())
	memberlist, err := worker.NewConsulMemberList(cfg.ConsulAddress, "forwarder-member", cfg.LockExpiry)
	if err != nil {
		log.Fatal(err)
	}
	go worker.BalanceWorkers(ctx, memberlist, workers, cfg.LockExpiry/2)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	cancel()
}
