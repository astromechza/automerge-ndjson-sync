package automergendjsonsync

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"

	"github.com/automerge/automerge-go"
)

// The messageGenerator is a io.ReadCloser which runs generateMessagesToWriter in a goroutine once the first read is
// requested. This uses an IO pipe so messages are only generated at the rate that they are being read off the reader.
// Some extra context and synchronisation elements are stored in the struct in order to optimise things.
type messageGenerator struct {
	ctx    context.Context
	state  *automerge.SyncState
	hints  <-chan bool
	writer *io.PipeWriter
	reader *io.PipeReader
	wg     *sync.WaitGroup
	once   sync.Once
}

func newMessageGenerator(ctx context.Context, state *automerge.SyncState, hints <-chan bool, wg *sync.WaitGroup) *messageGenerator {
	return &messageGenerator{ctx: ctx, state: state, hints: hints, wg: wg}
}

func (mg *messageGenerator) background() {
	defer mg.wg.Done()
	if err := generateMessagesToWriter(mg.ctx, mg.state, mg.hints, mg.writer, false); err != nil && !errors.Is(err, context.Canceled) {
		_ = mg.writer.CloseWithError(err)
	} else {
		_ = mg.writer.Close()
	}
}

func (mg *messageGenerator) start() {
	mg.wg.Add(1)
	mg.reader, mg.writer = io.Pipe()
	go mg.background()
}

func (mg *messageGenerator) Close() error {
	mg.once.Do(func() {})
	if mg.reader == nil {
		return nil
	}
	return mg.reader.Close()
}

func (mg *messageGenerator) Read(p []byte) (int, error) {
	mg.once.Do(mg.start)
	if mg.reader == nil {
		return 0, io.ErrClosedPipe
	}
	return mg.reader.Read(p)
}

var _ io.ReadCloser = (*messageGenerator)(nil)

type HttpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type HttpDoerFunc func(*http.Request) (*http.Response, error)

func (f HttpDoerFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

type clientOptions struct {
	client           HttpDoer
	state            *automerge.SyncState
	terminationCheck TerminationCheck
	reqEditors       []func(r *http.Request)
}

type ClientOption func(*clientOptions)

func newClientOptions(opts ...ClientOption) *clientOptions {
	options := &clientOptions{client: http.DefaultClient, terminationCheck: NoTerminationCheck}
	for _, opt := range opts {
		opt(options)
	}
	return options
}

func WithHttpClient(client HttpDoer) ClientOption {
	return func(o *clientOptions) {
		o.client = client
	}
}

func WithClientSyncState(state *automerge.SyncState) ClientOption {
	return func(o *clientOptions) {
		o.state = state
	}
}

func WithClientTerminationCheck(check TerminationCheck) ClientOption {
	return func(o *clientOptions) {
		o.terminationCheck = check
	}
}

func WithClientRequestEditor(f func(r *http.Request)) ClientOption {
	return func(o *clientOptions) {
		o.reqEditors = append(o.reqEditors, f)
	}
}

// HttpPushPullChanges is the HTTP client function to synchronise a local document with a remote server. This uses either HTTP2 or HTTP1.1 depending on the
// remote server - HTTP2 is preferred since it has better understood bidirectional body capabilities.
func (b *SharedDoc) HttpPushPullChanges(ctx context.Context, url string, opts ...ClientOption) error {
	o := newClientOptions(opts...)
	if o.state == nil {
		o.state = automerge.NewSyncState(b.Doc())
	}

	// We use the PUT method here because we are modifying a document in place.
	r, err := http.NewRequestWithContext(ctx, http.MethodPut, url, nil)
	if err != nil {
		return fmt.Errorf("failed to setup request: %w", err)
	}
	r.Header.Set("Content-Type", ContentTypeWithCharset)
	r.Header.Set("Accept", ContentType)
	r.Header.Set("Cache-Control", "no-store")
	// We don't need to send the body content if the server will reject it, so we can notify that expect-continue is supported.
	r.Header.Set("Expect", "100-continue")
	for _, editor := range o.reqEditors {
		editor(r)
	}

	wg := new(sync.WaitGroup)
	defer wg.Wait()

	sub, fin := b.SubscribeToReceivedChanges()
	defer fin()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// We use a special body generator that runs in a goroutine on demand in order to generate new messages.
	r.Body = newMessageGenerator(ctx, o.state, sub, wg)
	r.GetBody = func() (io.ReadCloser, error) {
		return newMessageGenerator(ctx, o.state, sub, wg), nil
	}

	res, err := o.client.Do(r)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	log.InfoContext(ctx, "received http sync response", slog.String("proto", res.Proto), slog.String("target", fmt.Sprintf("%s %s", http.MethodPut, url)), slog.Int("status", res.StatusCode))
	if res.StatusCode != 200 {
		return fmt.Errorf("http request failed with status %d", res.StatusCode)
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			log.ErrorContext(ctx, "failed to close response body", slog.Any("err", err))
		}
	}()

	if v := res.Header.Get("Content-Type"); v != "" && isNotSuitableContentType(v) {
		return fmt.Errorf("http request returned a response with an unsuitable content type %s", v)
	}

	if _, err := b.consumeMessagesFromReader(ctx, o.state, res.Body, NoReadPredicate, o.terminationCheck); err != nil {
		return err
	}
	return nil
}
