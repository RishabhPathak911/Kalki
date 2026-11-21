package telemetry

type Batch struct {
	Tenant      string   `json:"tenant"`
	CollectorID string   `json:"collector_id"`
	Region      string   `json:"region"`
	Seq         uint64   `json:"seq"`
	BatchID     string   `json:"batch_id"`
	SentAt      string   `json:"sent_at"`
	Records     []Record `json:"records"`
}

type Ack struct {
	Seq                uint64   `json:"seq"`
	HighestContiguous  uint64   `json:"highest_contiguous_seq"`
	Gaps               []uint64 `json:"gaps"`
	PayloadSHA256      string   `json:"payload_sha256"`
	ServerID           string   `json:"server_id"`
	ReceivedAt         string   `json:"received_at"`
	Duplicate          bool     `json:"duplicate"`
	DuplicatesRejected uint64   `json:"duplicates_rejected"`
}
