package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caarlos0/env/v6"
	_ "github.com/lib/pq"
	"github.com/nats-io/stan.go"
	"github.com/quintans/eventstore/common"
	"github.com/quintans/eventstore/feed"
	"github.com/quintans/eventstore/feed/pglistener"
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
	ConfigPusher
}

type ConfigEs struct {
	EsUser     string `env:"ES_USER" envDefault:"root"`
	EsPassword string `env:"ES_PASSWORD" envDefault:"password"`
	EsHost     string `env:"ES_HOST"`
	EsPort     int    `env:"ES_PORT" envDefault:"5432"`
	EsName     string `env:"ES_NAME" envDefault:"accounts"`
}

type ConfigNats struct {
	NatsAddress string `env:"NATS_ADDRESS"`
	Topic       string `env:"TOPIC" envDefault:"accounts"`
}

type ConfigPusher struct {
	PgChannel string `env:"PG_CHANNEL" envDefault:"eventsourcing"`
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

	sinker, err := sink.NewNatsSink(cfg.ConfigNats.Topic, 0,
		"test-cluster", "pusher-id", stan.NatsURL(cfg.ConfigNats.NatsAddress))
	if err != nil {
		log.Fatalf("Error instantiating Sinker: %v", err)
	}

	defer sinker.Close()

	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", cfg.EsUser, cfg.EsPassword, cfg.EsHost, cfg.EsPort, cfg.EsName)
	repo, err := repo.NewPgEsRepository(dbURL)
	if err != nil {
		log.Fatalf("Error instantiating event store: %v", err)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())

	listener, err := pglistener.New(dbURL, repo, cfg.PgChannel)
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

	monitor := common.NewBootMonitor(bootable, common.WithLock(locker), common.WithRefreshInterval(cfg.LockExpiry/2))
	go monitor.Start(ctx)

	go player.StartGrpcServer(ctx, cfg.GrpcAddress, repo)

	<-quit
	cancel()
}
