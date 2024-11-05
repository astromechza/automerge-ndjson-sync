package automergendjsonsync

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"github.com/automerge/automerge-go"
)

// NotifyReceivedChanges should be called after the shared doc has "received" a message. This allows any goroutines that
// are generating messages to be preempted and know that new messaged may be available. This is a broadcast because
// any number of goroutines may be writing changes to the doc to their client.
func (b *SharedDoc) NotifyReceivedChanges() {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	for _, channel := range b.channels {
		select {
		case channel <- true:
		default:
		}
	}
}

func (b *SharedDoc) consumeMessagesFromReader(ctx context.Context, state *automerge.SyncState, reader io.Reader, readPredicate ReadPredicate, terminationCheck TerminationCheck) (int, error) {
	received, receivedBytes, receivedChanges := 0, 0, 0
	defer func() {
		log.InfoContext(ctx, "finished receiving sync messages", slog.Int("received-messages", received), slog.Int("received-changes", receivedChanges), slog.Int("received-bytes", receivedBytes))
	}()

	sc := bufio.NewScanner(reader)
	for sc.Scan() {
		e := &NdJson{}
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			return received, fmt.Errorf("failed to unmarshal message %d: %w", received+1, err)
		} else if e.Event == EventSync {
			if m, err := automerge.LoadSyncMessage(e.Data); err != nil {
				return received, fmt.Errorf("failed to load message %d: %w", received+1, err)
			} else if ok, err := readPredicate(state.Doc, m); err != nil {
				return received, fmt.Errorf("failed to run read predicate on message %d: %w", received+1, err)
			} else if !ok {
				log.DebugContext(ctx, "skipping message", slog.Int("changes", len(m.Changes())), slog.Int("bytes", len(sc.Bytes())), slog.Any("heads", LoggableChangeHashes(m.Heads())))
			} else if _, err := state.ReceiveMessage(e.Data); err != nil {
				return received, fmt.Errorf("failed to receive message %d: %w", received+1, err)
			} else {
				log.DebugContext(ctx, "received message", slog.Int("changes", len(m.Changes())), slog.Int("bytes", len(sc.Bytes())), slog.Any("heads", LoggableChangeHashes(m.Heads())))
				received += 1
				receivedChanges += len(m.Changes())
				receivedBytes += len(sc.Bytes()) + 1
				b.NotifyReceivedChanges()

				if terminationCheck(state.Doc, m) {
					log.InfoContext(ctx, "termination check met")
					return received, nil
				}
			}
		}
	}
	if sc.Err() != nil {
		return received, fmt.Errorf("failed while scanning message %d: %w", received+1, sc.Err())
	}
	return received, nil
}
