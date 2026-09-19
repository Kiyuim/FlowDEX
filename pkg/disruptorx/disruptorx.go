package disruptorx

import (
	"fmt"

	"github.com/smartystreets-prototypes/go-disruptor"
)

// DisruptorWrapper is a generic wrapper for the disruptor with multiple consumer groups
type DisruptorWrapper[T any] struct {
	disruptor  disruptor.Disruptor
	bufferSize int64
	ringBuffer []T
	consumers  []Consumer[T]
}

// Consumer is the interface that must be implemented by consumer groups
type Consumer[T any] interface {
	Consume(lower, upper int64, buffer []T)
}

// NewDisruptorWrapper creates a new instance of DisruptorWrapper with specified consumer groups
func NewDisruptorWrapper[T any](bufferSize int64, consumers ...Consumer[T]) (*DisruptorWrapper[T], error) {
	// Ensure the buffer size is a power of two
	if bufferSize&(bufferSize-1) != 0 {
		return nil, fmt.Errorf("bufferSize must be a power of 2")
	}

	// Initialize Ring Buffer
	ringBuffer := make([]T, bufferSize)

	// Create a new DisruptorWrapper
	wrapper := &DisruptorWrapper[T]{
		bufferSize: bufferSize,
		ringBuffer: ringBuffer,
		consumers:  consumers,
	}

	// Register consumer groups
	consumerGroup := make([]disruptor.Consumer, len(consumers))
	for i, c := range consumers {
		consumerGroup[i] = &internalConsumer[T]{buffer: ringBuffer, consumer: c}
	}

	// Create the disruptor instance
	myDisruptor := disruptor.New(
		disruptor.WithCapacity(int64(int(bufferSize))),
		disruptor.WithConsumerGroup(consumerGroup...),
	)

	wrapper.disruptor = myDisruptor
	return wrapper, nil
}

// Start begins the disruptor, allowing consumers to start consuming messages
func (w *DisruptorWrapper[T]) Start() {
	// Start reading messages from the Ring Buffer
	w.disruptor.Read() // This will block until you close the disruptor
}

// Close stops the disruptor and releases resources
func (w *DisruptorWrapper[T]) Close() {
	_ = w.disruptor.Close()
}

// Publish publishes data to the disruptor, making it available to the consumers
func (w *DisruptorWrapper[T]) Publish(data ...T) {
	for _, d := range data {
		// Reserve a sequence for the message
		sequence := w.disruptor.Reserve(1)
		// Place the data in the Ring Buffer at the corresponding position
		w.ringBuffer[sequence%w.bufferSize] = d
		// Commit the sequence to notify consumers
		w.disruptor.Commit(sequence, sequence)
	}
}

// internalConsumer is an internal wrapper to adapt the generic Consumer to the disruptor interface
type internalConsumer[T any] struct {
	buffer   []T
	consumer Consumer[T]
}

func (c *internalConsumer[T]) Consume(lower, upper int64) {
	// Delegate the consumption of messages to the actual consumer
	c.consumer.Consume(lower, upper, c.buffer)
}
