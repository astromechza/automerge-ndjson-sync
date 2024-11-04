package automergendjsonsync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"sync"

	"github.com/automerge/automerge-go"
)

// SharedDoc encapsulates a doc with a signalling mechanism that broadcasts an event when new messages changes have
// been synced into the doc. This event is generally used to wake up other goroutines for generating sync messages to
// other clients or servers but can also be used to driver other mechanisms like backups or transformers.
type SharedDoc struct {
	doc      *automerge.Doc
	mutex    sync.Mutex
	channels []chan bool
}

// NewSharedDoc returns a new SharedDoc
func NewSharedDoc(doc *automerge.Doc) *SharedDoc {
	return &SharedDoc{doc: doc}
}

// Doc returns the document held by this SharedDoc.
func (b *SharedDoc) Doc() *automerge.Doc {
	return b.doc
}

type serverOptions struct {
	state         *automerge.SyncState
	headerEditors []func(rw http.Header)
}

type ServerOption func(*serverOptions)

func newServerOptions(opts ...ServerOption) *serverOptions {
	options := &serverOptions{}
	for _, opt := range opts {
		opt(options)
	}
	return options
}

func WithServerSyncState(state *automerge.SyncState) ServerOption {
	return func(o *serverOptions) {
		o.state = state
	}
}

func WithServerHeaderEditor(f func(headers http.Header)) ServerOption {
	return func(o *serverOptions) {
		o.headerEditors = append(o.headerEditors, f)
	}
}

func isNotSuitableContentType(in string) bool {
	mt, p, err := mime.ParseMediaType(in)
	log.Info(fmt.Sprintf("%v %v %v", mt, p, err))
	return err != nil || mt != ContentType || (p["charset"] != "" && p["charset"] != "utf-8")
}

func (b *SharedDoc) ServeChanges(rw http.ResponseWriter, req *http.Request, opts ...ServerOption) (finalErr error) {
	options := newServerOptions(opts...)
	if options.state == nil {
		options.state = automerge.NewSyncState(b.Doc())
	}

	// If there is an accept header, then ensure it's compatible.
	if v := req.Header.Get("Accept"); v != "" && isNotSuitableContentType(v) {
		rw.WriteHeader(http.StatusNotAcceptable)
		return nil
	}
	// If there is a content-type header, then ensure it's what we expect
	if v := req.Header.Get("Content-Type"); v != "" && isNotSuitableContentType(v) {
		rw.WriteHeader(http.StatusUnsupportedMediaType)
		return nil
	}

	ctx := req.Context()

	// Because the request body is relatively expensive to produce, the client may only want to produce it when the request has been accepted.
	// So it may send an Expect=100-continue header and expect us to honor it.
	if req.Header.Get("Expect") == "100-continue" {
		rw.WriteHeader(http.StatusContinue)
	}

	log.InfoContext(ctx, "sending http sync response", slog.String("proto", req.Proto), slog.String("target", fmt.Sprintf("%s %s", req.Method, req.URL)), slog.Int("status", http.StatusOK))
	rw.Header().Set("Content-Type", ContentTypeWithCharset)
	rw.Header().Set("X-Content-Type-Options", "nosniff")
	rw.Header().Set("Cache-Control", "no-store")
	for _, he := range options.headerEditors {
		he(rw.Header())
	}
	rw.WriteHeader(http.StatusOK)

	// Flush the header, this should ensure the client can begin reacting to our sync messages while still producing the body content.
	if v, ok := rw.(http.Flusher); ok {
		v.Flush()
	}

	// We produce the messages in a goroutine which must be shut down on exit.
	wg := new(sync.WaitGroup)
	defer wg.Wait()

	sub, fin := b.SubscribeToReceivedChanges()
	defer fin()

	// We piggyback on the context and ensure we cancel it before waiting for the wait group.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	log.DebugContext(ctx, "starting to read messages from request body in the background")
	wg.Add(1)
	go func() {
		defer wg.Done()
		if received, err := b.consumeMessagesFromReader(ctx, options.state, req.Body, NoTerminationCheck); err != nil {
			// If we've finished and the request context is closed (indicating that the client disconnected), then this
			// isn't really an error. For anything else, set the final error and cancel the context. The cancellation
			// should stop the writer from producing messages and lead to closing the response.
			if req.Context().Err() != nil {
				log.DebugContext(ctx, "client context closed")
			} else if errors.Is(err, http.ErrBodyReadAfterClose) {
				log.DebugContext(ctx, "read after close")
			} else {
				finalErr = err
				cancel()
			}
		} else if received == 0 {
			// It's bad if the request reached EOF without any sync messages since our writer can't really do anything
			// in response. So we set an error and cancel.
			finalErr = fmt.Errorf("request closed with no messages received")
			cancel()
		}
	}()

	log.DebugContext(ctx, "writing messages to response body")
	if err := generateMessagesToWriter(ctx, options.state, sub, rw, false); err != nil {
		// If we close and the request context is closed then there's no particular error unless finalErr has been set
		// from the reading routine.
		if ctx.Err() != nil {
			return
		}
		return errors.Join(err, finalErr)
	}
	return
}
