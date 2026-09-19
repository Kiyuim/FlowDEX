package mq

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/IBM/sarama"
	"github.com/zeromicro/go-zero/core/logx"
)

var _kafkaClient sarama.SyncProducer

type KqConf struct {
	Brokers  []string `json:",env=KAFKA_BROKERS"`
	Group    string   `json:",env=KAFKA_GROUP"`
	CaFile   string   `json:",optional,env=KAFKA_CAFILE"`
	Username string   `json:",optional,env=KAFKA_USERNAME"`
	Password string   `json:",optional,env=KAFKA_PASSWORD"`
}

type accessLogEntry struct {
	encoded []byte
}

func (ale *accessLogEntry) Length() int {
	return len(ale.encoded)
}

func (ale *accessLogEntry) Encode() ([]byte, error) {
	return ale.encoded, nil
}

func NewKafka(conf KqConf) sarama.SyncProducer {
	var err error
	logx.Infof("Initializing Kafka producer with brokers: %v, username: %s", conf.Brokers, conf.Username)

	if len(conf.Brokers) == 0 {
		logx.Errorf("❌ No Kafka brokers configured")
		return nil
	}

	// SASL creds are only required against a real SASL-secured broker (e.g. the
	// Alibaba Kafka used in prod). A local/docker-compose Kafka has no SASL, so
	// empty username/password now falls through to a plaintext connection
	// instead of refusing to start the producer — see
	// docs/项目已知问题与修复记录.md.
	_kafkaClient, err = newAccessLogProducer(conf.Brokers, conf.CaFile, conf.Username, conf.Password)
	if err != nil || _kafkaClient == nil {
		logx.Errorf("❌ _kafkaClient Start error: %v", err)
		logx.Errorf("❌ Brokers: %v, Username: %s", conf.Brokers, conf.Username)
		logx.Errorf("⚠️  Kafka producer initialization failed. Messages will not be sent to Kafka.")
		logx.Errorf("💡 Troubleshooting:")
		logx.Errorf("   1. Verify Kafka broker is running: netstat -tlnp | grep 9093")
		logx.Errorf("   2. Check broker SASL configuration matches client")
		logx.Errorf("   3. Verify username/password are correct")
		logx.Errorf("   4. Test connection: go run tools/kafka_check.go %s %s <password>", conf.Brokers[0], conf.Username)
		// panic("_kafkaClient Start error") // Uncomment to fail fast if Kafka is required
		return nil
	}
	logx.Infof("✅ Kafka producer initialized successfully")
	return _kafkaClient
}
func SendEventLogKafkaInfoMessage(topic string, key string, data []byte) error {
	if _kafkaClient == nil {
		logx.Errorf("❌ Cannot send message to Kafka: _kafkaClient is nil. Kafka was not initialized properly.")
		return errors.New("_kafkaClient is Nil")
	}

	message := &sarama.ProducerMessage{
		Topic: topic,
		Key:   &accessLogEntry{encoded: []byte(key)},
		Value: &accessLogEntry{encoded: data},
	}

	p, o, err := _kafkaClient.SendMessage(message)
	if err != nil {
		logx.Errorf("[kafka] send event log to kafka failed: error:%v", err)
		return err
	}
	logx.Infof("[kafka] send event log to kafka success: %v:%v:%v, %v, len(data): %v",
		topic, p, o, key, len(data))
	return nil
}

func newAccessLogProducer(brokers []string, _, username, password string) (sarama.SyncProducer, error) {
	// No SASL creds configured (local/docker-compose Kafka) — connect plaintext,
	// no auth attempt at all.
	if username == "" || password == "" {
		logx.Infof("No Kafka SASL credentials configured; connecting without SASL (local/dev broker)")
		return newPlaintextProducer(brokers)
	}

	// Try both SASL_PLAINTEXT and SASL_SSL configurations
	// Some brokers use SASL_PLAINTEXT (no TLS), others use SASL_SSL (with TLS)

	// Attempt 1: SASL_PLAINTEXT (no TLS)
	logx.Infof("Attempting SASL_PLAINTEXT connection (no TLS)...")
	producer, err := trySASLConnection(brokers, username, password, false)
	if err == nil {
		logx.Infof("✅ Successfully connected using SASL_PLAINTEXT")
		return producer, nil
	}
	logx.Infof("SASL_PLAINTEXT failed: %v", err)

	// Attempt 2: SASL_SSL (with TLS)
	logx.Infof("Attempting SASL_SSL connection (with TLS)...")
	producer, err = trySASLConnection(brokers, username, password, true)
	if err != nil {
		logx.Errorf("Both SASL_PLAINTEXT and SASL_SSL failed. Last error: %v", err)
		return nil, fmt.Errorf("all connection attempts failed: SASL_PLAINTEXT error: %v, SASL_SSL error: %w", err, err)
	}
	logx.Infof("✅ Successfully connected using SASL_SSL")
	return producer, nil
}

func newPlaintextProducer(brokers []string) (sarama.SyncProducer, error) {
	config := sarama.NewConfig()
	config.Net.DialTimeout = 10 * time.Second
	config.Net.ReadTimeout = 10 * time.Second
	config.Net.WriteTimeout = 10 * time.Second
	config.Net.KeepAlive = 30 * time.Second

	sarama.Logger = log.New(os.Stdout, "[sarama] ", log.LstdFlags)
	config.Producer.Timeout = 5 * time.Second
	config.Producer.MaxMessageBytes = 1024 * 1024 * 10
	config.Producer.Partitioner = sarama.NewHashPartitioner
	config.Metadata.AllowAutoTopicCreation = true
	config.Producer.Retry.Max = 3
	config.Producer.Retry.Backoff = 100 * time.Millisecond
	config.Producer.Return.Errors = true
	config.Producer.Return.Successes = true
	config.Net.MaxOpenRequests = 1024
	config.ChannelBufferSize = 256
	config.ClientID = "producer-dex-consumer-plaintext-nosasl"

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, err
	}
	fmt.Println("producer connect success (no SASL)")
	return producer, nil
}

func trySASLConnection(brokers []string, username, password string, useTLS bool) (sarama.SyncProducer, error) {
	config := sarama.NewConfig()

	// Network related configurations
	config.Net.DialTimeout = 10 * time.Second
	config.Net.ReadTimeout = 10 * time.Second
	config.Net.WriteTimeout = 10 * time.Second
	config.Net.KeepAlive = 30 * time.Second

	sarama.Logger = log.New(os.Stdout, "[sarama] ", log.LstdFlags)
	config.Producer.Timeout = 5 * time.Second
	config.Producer.MaxMessageBytes = 1024 * 1024 * 10
	config.Producer.Partitioner = sarama.NewHashPartitioner
	config.Metadata.AllowAutoTopicCreation = true

	// Retry configurations
	config.Producer.Retry.Max = 3
	config.Producer.Retry.Backoff = 100 * time.Millisecond
	config.Producer.Return.Errors = true

	// TLS configuration
	if useTLS {
		config.Net.TLS.Enable = true
		config.Net.TLS.Config = &tls.Config{
			InsecureSkipVerify: true, // WARNING: for test only!
		}
	} else {
		config.Net.TLS.Enable = false
	}

	config.Net.MaxOpenRequests = 1024
	config.ChannelBufferSize = 256

	// SASL configurations
	config.Net.SASL.Enable = true
	config.Net.SASL.Mechanism = sarama.SASLTypePlaintext
	config.Net.SASL.User = username
	config.Net.SASL.Password = password
	config.Net.SASL.Handshake = true

	config.Producer.Return.Successes = true

	if useTLS {
		config.ClientID = "producer-dex-consumer-ssl"
	} else {
		config.ClientID = "producer-dex-consumer-plaintext"
	}

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, err
	}
	fmt.Println("producer connect success")
	return producer, nil
}
