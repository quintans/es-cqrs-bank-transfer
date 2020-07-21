package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/stretchr/testify/assert"
)

const (
	lookupURL = "pulsar://localhost:6650"
	topic     = "persistent://public/default/test"
)

func TestReaderLatestInclusiveHasNext(t *testing.T) {
	client, err := pulsar.NewClient(pulsar.ClientOptions{
		URL: lookupURL,
	})

	assert.Nil(t, err)
	defer client.Close()

	ctx := context.Background()

	// create producer
	producer, err := client.CreateProducer(pulsar.ProducerOptions{
		Topic:           topic,
		DisableBatching: true,
	})
	assert.Nil(t, err)
	defer producer.Close()

	// send 10 messages
	var lastMsgID pulsar.MessageID
	for i := 0; i < 10; i++ {
		lastMsgID, err = producer.Send(ctx, &pulsar.ProducerMessage{
			Payload: []byte(fmt.Sprintf("hello-%d", i)),
		})
		assert.NoError(t, err)
		assert.NotNil(t, lastMsgID)
	}

	// create reader on the last message (inclusive)
	reader, err := client.CreateReader(pulsar.ReaderOptions{
		Topic:                   topic,
		StartMessageID:          pulsar.LatestMessageID(),
		StartMessageIDInclusive: true,
	})

	assert.Nil(t, err)
	defer reader.Close()

	var msgID pulsar.MessageID
	if reader.HasNext() {
		msg, err := reader.Next(context.Background())
		assert.NoError(t, err)

		assert.Equal(t, []byte("hello-10"), msg.Payload())
		msgID = msg.ID()
	}

	assert.Equal(t, lastMsgID, msgID)
}
