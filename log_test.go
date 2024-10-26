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
	SetLog(slog.New(slog.NewTextHandler(buff, &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == "time" {
				a.Value = slog.TimeValue(time.Unix(0, 0))
			}
			return a
		},
	})))
	log.InfoContext(context.TODO(), "hello")
	log.DebugContext(context.TODO(), "world")
	ResetLog()
	log.InfoContext(context.TODO(), "other")
	assertEqual(t, buff.String(), "time=1970-01-01T01:00:00.000+01:00 level=INFO msg=hello\n")
}
