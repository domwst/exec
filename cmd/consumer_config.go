package cmd

// TODO: Move on from json to HOCON

type ConsumerConfig struct {
	StreamName  string   `json:"stream-name"`
	Name        string   `json:"name"`          // Durable/queue name
	AckWaitTime Duration `json:"ack-wait-time"` // How long after no response the worker considered dead
}
