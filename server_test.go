package automergendjsonsync

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/automerge/automerge-go"
)

func TestServe_empty_request_body(t *testing.T) {
	t.Parallel()
	sd := NewSharedDoc(automerge.New())
	rw := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/", nil)
	assertErrorEqual(t, sd.ServeChanges(rw, req), "request closed with no messages received")
	assertEqual(t, rw.Result().StatusCode, http.StatusOK)
	assertEqual(t, rw.Result().Header, map[string][]string{
		"Content-Type":           {ContentTypeWithCharset},
		"X-Content-Type-Options": {"nosniff"},
	})
	assertEqual(t, rw.Body.String(), "{\"event\":\"sync\",\"data\":\"QgAAAQAAAA==\"}\n")
}

type wrappedRw struct {
	http.ResponseWriter
	lineWaiter *sync.WaitGroup
}

func (w *wrappedRw) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	for _, b := range data {
		if b == '\n' {
			w.lineWaiter.Done()
		}
	}
	return n, err
}

func (w *wrappedRw) Flush() {
	w.ResponseWriter.(http.Flusher).Flush()
}

var _ http.ResponseWriter = (*wrappedRw)(nil)
var _ http.Flusher = (*wrappedRw)(nil)

func TestServe_exchange(t *testing.T) {
	t.Parallel()
	sd := NewSharedDoc(automerge.New())

	// create some fake data
	assertEqual(t, sd.Doc().RootMap().Set("a", "b"), nil)
	_, _ = sd.Doc().Commit("change")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// The server will keep the connection open continuously writing whatever it may get in the future. So for the purpose of our test
	// we need to know when we should cancel it once the required lines have been sent. We expect only 2 messages to be sent.
	lineWaiter := new(sync.WaitGroup)
	lineWaiter.Add(2)
	go func() {
		lineWaiter.Wait()
		cancel()
	}()
	req := httptest.NewRequestWithContext(ctx, http.MethodPut, "/", io.NopCloser(strings.NewReader("{\"event\":\"sync\",\"data\":\"QgAAAQAAAA==\"}\n")))
	rw := httptest.NewRecorder()
	assertEqual(t, sd.ServeChanges(&wrappedRw{lineWaiter: lineWaiter, ResponseWriter: rw}, req), nil)

	assertEqual(t, rw.Result().StatusCode, http.StatusOK)
	assertEqual(t, rw.Result().Header, map[string][]string{
		"Content-Type":           {ContentTypeWithCharset},
		"X-Content-Type-Options": {"nosniff"},
	})
	lines := strings.Split(strings.TrimSpace(rw.Body.String()), "\n")
	assertEqual(t, len(lines), 2)
}
