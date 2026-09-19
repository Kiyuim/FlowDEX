package mq

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/IBM/sarama"
)

// https://github.com/Azure/azure-event-hubs-for-kafka/blob/master/quickstart/go-sarama-client/README.md
func TestKafkaProducer(t *testing.T) {
	brokerList := []string{"test-litentry-event.servicebus.windows.net:9093"}
	fmt.Println("Event Hubs broker", brokerList)

	producer, err := sarama.NewSyncProducer(brokerList, getConfig())

	if err != nil {
		fmt.Println("Failed to start Sarama producer:", err)
		os.Exit(1)
	}
	sarama.Logger = log.New(os.Stdout, "[sarama] ", log.LstdFlags)

	eventHubsTopic := "sol-trades"
	fmt.Println("Event Hubs topic", eventHubsTopic)
	producerOpen := true
	go func() {
		for {
			if producerOpen {
				ts := time.Now().String()
				msg := &sarama.ProducerMessage{Topic: eventHubsTopic, Key: sarama.StringEncoder("key-" + ts), Value: sarama.StringEncoder("value-" + ts)}
				p, o, err := producer.SendMessage(msg)
				if err != nil {
					fmt.Println("Failed to send msg:", err)
					return
				}
				fmt.Printf("sent message to partition %d offset %d\n", p, o)
			}
			time.Sleep(3 * time.Second) // intentional pause
		}
	}()

	close := make(chan os.Signal)
	signal.Notify(close, syscall.SIGTERM, syscall.SIGINT)
	fmt.Println("Waiting for program to exit...")
	<-close

	fmt.Println("closing producer")
	err = producer.Close()
	producerOpen = false
	if err != nil {
		fmt.Println("failed to close producer", err)
	}
	fmt.Println("closed producer")
}

func getConfig() *sarama.Config {
	config := sarama.NewConfig()
	config.Net.DialTimeout = 10 * time.Second
	config.Consumer.Offsets.Initial = sarama.OffsetNewest // Start from the latest message

	// SASL/PLAIN config
	config.Net.SASL.Enable = true
	config.Net.SASL.User = "alikafka_post-cn-zp54bjj8x004"
	config.Net.SASL.Password = "htIJB7fFfcXbNVnH8rWLD1m52gfkOQI1"
	config.Net.SASL.Mechanism = sarama.SASLTypePlaintext

	// Disable TLS for SASL_PLAINTEXT
	config.Net.TLS.Enable = false
	config.Net.TLS.Config = nil

	config.Producer.Return.Successes = true
	config.Metadata.Full = false
	config.ClientID = "consumer-dex-test"
	return config
}
