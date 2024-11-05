package automergendjsonsync

import (
	"bytes"
	"context"
	"log/slog"

	"github.com/automerge/automerge-go"
)

type logContextKeyType int

var logContextKey = logContextKeyType(0)

func SetContextLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, logContextKey, l)
}

func Logger(ctx context.Context) *slog.Logger {
	if v, ok := ctx.Value(logContextKey).(*slog.Logger); ok {
		return v
	}
	return slog.Default()
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
