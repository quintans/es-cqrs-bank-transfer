package main

import (
	"context"

	"github.com/quintans/eventstore/feed/poller"
	"github.com/quintans/eventstore/sink"
	log "github.com/sirupsen/logrus"
)

type Feeder struct {
	feed poller.Poller
	snk  sink.Sink
}

func NewFeeder(feed poller.Poller, snk sink.Sink) Feeder {
	return Feeder{
		feed: feed,
		snk:  snk,
	}
}

func (f Feeder) Boot(ctx context.Context) error {
	go func() {
		log.Println("Starting Polling forward")
		f.feed.Forward(ctx, f.snk)
	}()
	return nil
}
