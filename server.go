package automergendjsonsync

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/automerge/automerge-go"
)

type SharedDoc struct {
	doc      *automerge.Doc
	mutex    sync.Mutex
	channels []chan bool
}

func NewSharedDoc(doc *automerge.Doc) *SharedDoc {
	return &SharedDoc{doc: doc}
}

func (b *SharedDoc) Doc() *automerge.Doc {
	return b.doc
}

type serverOptions struct {
	state *automerge.SyncState
}

type ServerOption func(*serverOptions)

func WithServerSyncState(state *automerge.SyncState) ServerOption {
	return func(o *serverOptions) {
		o.state = state
	}
}

func (b *SharedDoc) ServeChanges(rw http.ResponseWriter, req *http.Request, opts ...ServerOption) (finalErr error) {
	options := &serverOptions{}
	for _, opt := range opts {
		opt(options)
	}
	if options.state == nil {
		options.state = automerge.NewSyncState(b.Doc())
	}

	ctx := req.Context()

	// Because the request body is relatively expensive to produce, the client may only want to produce it when the request has been accepted.
	// So it may send an Expect=100-continue header and expect us to honor it.
	if req.Header.Get("Expect") == "100-continue" {
		rw.WriteHeader(http.StatusContinue)
	}

	log.InfoContext(ctx, "sending http sync response", slog.String("proto", req.Proto), slog.String("target", fmt.Sprintf("%s %s", req.Method, req.URL)), slog.Int("status", http.StatusOK))
	rw.Header().Set("Content-Type", ContentType)
	rw.Header().Set("Transfer-Encoding", "chunked")
	rw.WriteHeader(http.StatusOK)
	// Flush the header, this should ensure the client can begin reacting to our sync messages while still producing the body content.
	if v, ok := rw.(http.Flusher); ok {
		v.Flush()
	}

	// We produce the messages in a goroutine which must be shut down on exit.
	wg := new(sync.WaitGroup)
	defer wg.Wait()

	sub, fin := b.subscribeToReceivedChanges()
	defer fin()

	// We piggy back on the context and ensure we cancel it before waiting for the wait group.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	log.DebugContext(ctx, "starting to read messages from request body in the background")
	wg.Add(1)
	go func() {
		defer wg.Done()
		if received, err := b.consumeMessagesFromReader(ctx, options.state, req.Body, NoTerminationCheck); err != nil {
			if req.Context().Err() == nil {
				finalErr = err
				cancel()
			}
		} else if received == 0 {
			finalErr = fmt.Errorf("request closed with no messages received")
			cancel()
		}
	}()

	log.DebugContext(ctx, "writing messages to response body")
	if err := generateMessagesToWriter(ctx, options.state, sub, rw, false); err != nil && req.Context().Err() == nil {
		if finalErr != nil {
			return finalErr
		}
		return err
	}
	return finalErr
}
