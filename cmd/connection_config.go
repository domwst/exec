package cmd

import "github.com/nats-io/nats.go"

type ConnectionConfig struct {
	User     string `json:"user"`
	Password string `json:"password"`
	NatsUrls string `json:"nats-urls"`
}

func (config *ConnectionConfig) Connect() (*nats.Conn, error) {
	return nats.Connect(
		config.NatsUrls,
		nats.UserInfo(config.User, config.Password),
	)
}
