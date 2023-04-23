package cmd

import "github.com/nats-io/nats.go"

type ConnectionConfig struct {
	User     string `json:"user"`
	Password string `json:"password"`
	NatsUrl  string `json:"nats-url"`
}

func (config *ConnectionConfig) Connect() (*nats.Conn, error) {
	return nats.Connect(
		config.NatsUrl,
		nats.UserInfo(config.User, config.Password),
	)
}
