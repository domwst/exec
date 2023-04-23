package cmd

type TaskMsg struct {
	ObjectStoreKey string
	RequestId      string
}

type RunStatus uint8

const (
	Enqueued RunStatus = iota
	Processing
	Processed
)

type RunResult struct {
}
