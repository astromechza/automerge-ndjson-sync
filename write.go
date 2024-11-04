package automergendjsonsync

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"

	"github.com/automerge/automerge-go"
)

// SubscribeToReceivedChanges allows the caller to subscribe to changes received by the doc. Call the finish function
// to clean up.
func (b *SharedDoc) SubscribeToReceivedChanges() (chan bool, func()) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if b.channels == nil {
		b.channels = make([]chan bool, 0, 1)
	}
	nc := make(chan bool, 1)
	b.channels = append(b.channels, nc)
	return nc, func() {
		b.mutex.Lock()
		defer b.mutex.Unlock()
		if i := slices.Index(b.channels, nc); i >= 0 {
			c := b.channels[i]
			b.channels = slices.Delete(b.channels, i, i+1)
			close(c)
		}
	}
}

func generateMessagesToWriter(ctx context.Context, state *automerge.SyncState, hintChannel <-chan bool, writer io.Writer, immediate bool) error {
	sent, sentBytes, sentChanges := 0, 0, 0
	defer func() {
		log.InfoContext(ctx, "finished writing sync messages", slog.Int("sent-messages", sent), slog.Int("sent-changes", sentChanges), slog.Int("sent-bytes", sentBytes))
	}()

	for {
		for {
			if m, ok := state.GenerateMessage(); !ok {
				break
			} else {
				r, _ := json.Marshal(&NdJson{Event: EventSync, Data: m.Bytes()})
				r = append(r, '\n')
				if n, err := writer.Write(r); err != nil {
					return fmt.Errorf("failed to marshal: %w", err)
				} else {
					sent += 1
					sentBytes += n
					sentChanges += len(m.Changes())
					log.DebugContext(ctx, "wrote message", slog.Int("changes", len(m.Changes())), slog.Int("bytes", n), slog.Any("heads", LoggableChangeHashes(m.Heads())))
				}
				if f, ok := writer.(http.Flusher); ok {
					f.Flush()
				}
			}
		}
		if immediate {
			break
		}
		select {
		case <-hintChannel:
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if v, ok := writer.(io.Closer); ok {
		if err := v.Close(); err != nil {
			return fmt.Errorf("failed to close writer: %w", err)
		}
	}
	return nil
}
