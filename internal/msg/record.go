package msg

// Record represents a consumed Kafka record
type Record struct {
	Topic     string
	Key       string
	Value     []byte
	Partition int32
	Offset    int64
	Timestamp int64
}

