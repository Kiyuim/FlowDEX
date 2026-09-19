package mq

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"testing"

	"github.com/IBM/sarama"
)

func TestKafkaConsumer(t *testing.T) {
	brokerList := []string{"115.159.107.189:9093"}
	consumerGroupID := "my-consumer-group2"

	fmt.Println("Event Hubs broker", brokerList)
	fmt.Println("Sarama client consumer group ID", consumerGroupID)

	sarama.Logger = log.New(os.Stdout, "[sarama] ", log.LstdFlags)

	consumer, err := sarama.NewConsumerGroup(brokerList, consumerGroupID, getConfig())
	if err != nil {
		fmt.Println("error creating new consumer group", err)
		os.Exit(1)
	}

	fmt.Println("new consumer group created")

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for {
			err = consumer.Consume(ctx, []string{"web3fun"}, messageHandler{})
			if err != nil {
				fmt.Println("error consuming from group", err)
				os.Exit(1)
			}
			if ctx.Err() != nil {
				return
			}
		}
	}()

	close := make(chan os.Signal)
	signal.Notify(close, syscall.SIGTERM, syscall.SIGINT)
	fmt.Println("Waiting for program to exit")
	<-close
	cancel()
	fmt.Println("closing consumer group....")

	if err := consumer.Close(); err != nil {
		fmt.Println("error trying to close consumer", err)
		os.Exit(1)
	}
	fmt.Println("consumer group closed")
}

type messageHandler struct{}

func (h messageHandler) Setup(s sarama.ConsumerGroupSession) error {
	fmt.Println("Partition allocation -", s.Claims())
	return nil
}

func (h messageHandler) Cleanup(s sarama.ConsumerGroupSession) error {
	fmt.Println("Consumer group clean up initiated")
	return nil
}

func (h messageHandler) ConsumeClaim(s sarama.ConsumerGroupSession, c sarama.ConsumerGroupClaim) error {
	for msg := range c.Messages() {
		fmt.Printf("Message received - topic: %s, partition: %d, offset: %d, value: %s\n",
			msg.Topic, msg.Partition, msg.Offset, string(msg.Value))
		s.MarkMessage(msg, "") // Mark message as processed
	}
	return nil // Ensure no error is returned
}
