package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caarlos0/env/v6"
	"github.com/nats-io/stan.go"
	"github.com/quintans/es-cqrs-bank-transfer/account/shared/event"
	"github.com/quintans/eventstore/common"
	"github.com/quintans/eventstore/feed"
	"github.com/quintans/eventstore/feed/mongolistener"
	"github.com/quintans/eventstore/player"
	"github.com/quintans/eventstore/repo"
	"github.com/quintans/eventstore/sink"
	"github.com/quintans/toolkit/locks"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	RedisAddresses []string      `env:"REDIS_ADDRESSES" envSeparator:","`
	LockExpiry     time.Duration `env:"LOCK_EXPIRY" envDefault:"20s"`
	GrpcAddress    string        `env:"ES_ADDRESS" envDefault:":3000"`
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
}

func main() {
	cfg := &Config{}
	err := env.Parse(cfg)
	if err != nil {
		log.Fatal(err)
	}

	sinker := sink.NewNatsSink(cfg.ConfigNats.Topic, 0, "test-cluster", "pusher-id", stan.NatsURL(cfg.ConfigNats.NatsAddress))
	defer sinker.Close()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	dbURL := fmt.Sprintf("mongodb://%s:%s@%s:%d?connect=direct", cfg.EsUser, cfg.EsPassword, cfg.EsHost, cfg.EsPort)
	listener, err := mongolistener.New(dbURL, cfg.EsName)
	if err != nil {
		log.Fatalf("Error instantiating store listener: %v", err)
	}
	bootable := feed.New(
		listener,
		sinker,
	)
	locker, err := locks.NewRedisLock(cfg.RedisAddresses, "pusher", cfg.LockExpiry)
	if err != nil {
		log.Fatal("Error instantiating Locker: %v", err)
	}

	monitor := common.NewBootMonitor("MongoDB -> NATS feeder", bootable, common.WithLock(locker), common.WithRefreshInterval(cfg.LockExpiry/2))
	ctx, cancel := context.WithCancel(context.Background())
	go monitor.Start(ctx)

	repo, err := repo.NewMongoEsRepository(dbURL, cfg.EsName, event.EventFactory{})
	if err != nil {
		log.Fatalf("Error instantiating event store: %v", err)
	}
	go player.StartGrpcServer(ctx, cfg.GrpcAddress, repo)

	<-quit
	cancel()
}
