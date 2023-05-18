package cmd

type KeyValueBucketConfig struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Replicas    int    `json:"replicas"`
}
