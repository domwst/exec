package common

type ConnectionConfig struct {
	User     string `json:"user"`
	Password string `json:"password"`
	NatsUrl  string `json:"nats-url"`
}
