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
	"github.com/quintans/eventstore/common"
	"github.com/quintans/eventstore/player"
	"github.com/quintans/eventstore/sink"
	"github.com/quintans/eventstore/store"
	"github.com/quintans/eventstore/store/mongodb"
	"github.com/quintans/toolkit/locks"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	RedisAddresses []string      `env:"REDIS_ADDRESSES" envSeparator:","`
	LockExpiry     time.Duration `env:"LOCK_EXPIRY" envDefault:"2s"`
	GrpcAddress    string        `env:"ES_ADDRESS" envDefault:":3000"`
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
}

func main() {
	cfg := &Config{}
	err := env.Parse(cfg)
	if err != nil {
		log.Fatal(err)
	}

	partitionSlots, err := common.ParseSlots(cfg.PartitionSlots)
	if err != nil {
		log.Fatal(err)
	}
	var partitionsNo uint32
	for _, ps := range partitionSlots {
		partitionsNo += ps.Size()
	}

	sinker := sink.NewNatsSink(cfg.ConfigNats.Topic, partitionsNo, "test-cluster", "pusher-id", stan.NatsURL(cfg.ConfigNats.NatsAddress))
	defer sinker.Close()

	dbURL := fmt.Sprintf("mongodb://%s:%s@%s:%d?connect=direct", cfg.EsUser, cfg.EsPassword, cfg.EsHost, cfg.EsPort)
	pool, err := locks.NewRedisLockPool(cfg.RedisAddresses)
	if err != nil {
		log.Fatal("Error instantiating Locker: %v", err)
	}

	size := len(partitionSlots)
	lockMonitors := make([]common.LockMonitor, size)
	for i := 0; i < size; i++ {
		listener, err := mongodb.NewFeed(dbURL, cfg.EsName)
		if err != nil {
			log.Fatalf("Error instantiating store listener: %v", err)
		}
		forwarder := store.NewForwarder(
			listener,
			sinker,
		)

		monitor := common.NewBootMonitor("MongoDB -> NATS feeder", forwarder, common.WithRefreshInterval(cfg.LockExpiry/2))
		lockMonitors[i] = common.LockMonitor{
			Lock:    pool.NewLock("forwarder-lock", cfg.LockExpiry),
			Monitor: &monitor,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	memberlist := common.NewRedisMemberlist(cfg.RedisAddresses[0], "forwarder-member", cfg.LockExpiry)
	go common.BalancePartitions(ctx, memberlist, lockMonitors, cfg.LockExpiry/2)

	repo, err := mongodb.NewStore(dbURL, cfg.EsName)
	if err != nil {
		log.Fatalf("Error instantiating event store: %v", err)
	}
	go player.StartGrpcServer(ctx, cfg.GrpcAddress, repo)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	cancel()
}
