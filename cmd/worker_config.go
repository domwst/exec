package cmd

type WorkerConfig struct {
	WorkerThreads                 int                     `json:"worker-threads"`
	PathToTools                   string                  `json:"path-to-tools"`
	ConsumerConfig                ConsumerConfig          `json:"consumer-config"`
	ConnectionConfig              ConnectionConfig        `json:"connection-config"`
	SourceObjectStoreBucketConfig ObjectStoreBucketConfig `json:"source-object-store-bucket-config"`
	ResultObjectStoreBucketConfig ObjectStoreBucketConfig `json:"result-object-store-bucket-config"`
	KeyValueBucketConfig          KeyValueBucketConfig    `json:"key-value-bucket-config"`
}
