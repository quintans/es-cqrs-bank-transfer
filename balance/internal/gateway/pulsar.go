package gateway

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/quintans/es-cqrs-bank-transfer/balance/internal/domain"
	log "github.com/sirupsen/logrus"
)

const (
	EventIDKey        = "EventID"
	NotificationTopic = "notifications"
)

type Messenger struct {
	PulsarAddress string
	Client        pulsar.Client
}

func (m Messenger) GetLastMessageID(ctx context.Context, topic string) ([]byte, string, error) {
	// 2020-07-30: A separate client is created for reading, otherwise the producer will hang
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

		log.Infof("Will start polling from last event ID: %s", eid)
		return msg.ID().Serialize(), eid, nil
	}

	log.Info("Will start polling from the begginning")
	return pulsar.LatestMessageID().Serialize(), "", nil
}

func (m Messenger) NotifyProjectionRegistry(ctx context.Context, n domain.Notification) error {
	producer, err := m.Client.CreateProducer(pulsar.ProducerOptions{
		Topic: NotificationTopic,
	})
	if err != nil {
		return err
	}
	// since this not used often, we will close it
	//? will this hang new producers?
	defer producer.Close()

	payload, err := json.Marshal(n)
	if err != nil {
		return err
	}
	_, err = producer.Send(context.Background(), &pulsar.ProducerMessage{
		Payload: payload,
	})
	return err
}
