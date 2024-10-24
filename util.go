package automergendjsonsync

const ContentType = "application/x-ndjson; charset=utf-8"
const EventSync = "sync"

type NdJson struct {
	Event string `json:"event"`
	Data  []byte `json:"data,omitempty"`
}
