package automergendjsonsync

const ContentType = "application/x-ndjson"
const ContentTypeWithCharset = ContentType + "; charset=utf-8"
const EventSync = "sync"

type NdJson struct {
	Event string `json:"event"`
	Data  []byte `json:"data,omitempty"`
}
