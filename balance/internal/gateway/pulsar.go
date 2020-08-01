package gateway

import (
	"context"
	"fmt"
	"log"

	"github.com/apache/pulsar-client-go/pulsar"
)

const (
	EventIDKey = "EventID"
)

type Messenger struct {
	PulsarAddress string
}

func (m Messenger) GetLastMessageID(ctx context.Context, topic string) ([]byte, string, error) {
	// 2020-07-26: A separate client is created for reading, otherwise the producer will hang
	client, err := pulsar.NewClient(pulsar.ClientOptions{
		URL: "pulsar://" + m.PulsarAddress,
	})
	if err != nil {
		return nil, "", fmt.Errorf("Could not instantiate Pulsar client for reading: %w", err)
	}
	defer client.Close()

	reader, err := client.CreateReader(pulsar.ReaderOptions{
		Topic:                   topic,
		StartMessageID:          pulsar.LatestMessageID(),
		StartMessageIDInclusive: true,
	})
	if err != nil {
		return nil, "", fmt.Errorf("Unable to create reader for topic %s: %w", topic, err)
	}
	defer reader.Close()

	if reader.HasNext() {
		msg, err := reader.Next(ctx)
		if err != nil {
			return nil, "", fmt.Errorf("Unable to read last message in topic %s: %w", topic, err)
		}
		eid := msg.Properties()[EventIDKey]

		log.Printf("Will start polling from last event ID: %s", eid)
		return msg.ID().Serialize(), eid, nil
	}

	log.Println("Will start polling from the begginning")
	return pulsar.EarliestMessageID().Serialize(), "", nil
}
