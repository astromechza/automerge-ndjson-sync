package automergendjsonsync

import (
	"bytes"
	"io"
	"log/slog"

	"github.com/automerge/automerge-go"
)

// log is a structured logger used by anything within this package.
var log *slog.Logger

// init calls ResetLog to ensure that we always have the default pkg logging behavior.
func init() {
	ResetLog()
}

// SetLog will set the logger used by this package. By default, it is disabled and will not log any messages.
func SetLog(l *slog.Logger) {
	if l == nil {
		panic("nil logger is unacceptable")
	}
	log = l
}

// ResetLog	will set the logger back to its default value - disabled.
func ResetLog() {
	SetLog(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError})))
}

// LoggableChangeHashes is a type alias that allows the change hash array that represents the document heads to be
// easily attached to log messages via the slog.LogValuer interface.
type LoggableChangeHashes []automerge.ChangeHash

func (l LoggableChangeHashes) LogValue() slog.Value {
	buff := new(bytes.Buffer)
	for i, head := range l {
		if i > 0 {
			_ = buff.WriteByte(',')
		}
		_, _ = buff.WriteString(head.String())
	}
	return slog.StringValue(buff.String())
}

var _ slog.LogValuer = LoggableChangeHashes{}
