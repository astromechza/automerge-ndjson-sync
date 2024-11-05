package automergendjsonsync

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestLog(t *testing.T) {
	buff := new(bytes.Buffer)
	ctx := SetContextLogger(context.Background(), slog.New(slog.NewTextHandler(buff, &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == "time" {
				a.Value = slog.TimeValue(time.Unix(0, 0).UTC())
			}
			return a
		},
	})))
	log := Logger(ctx)
	log.InfoContext(context.TODO(), "hello")
	log.DebugContext(context.TODO(), "world")
	assertEqual(t, buff.String(), "time=1970-01-01T00:00:00.000Z level=INFO msg=hello\n")
}
