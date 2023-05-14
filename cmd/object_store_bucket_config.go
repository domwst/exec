package cmd

type ObjectStoreBucketConfig struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Replicas    int    `json:"replicas"`
}
