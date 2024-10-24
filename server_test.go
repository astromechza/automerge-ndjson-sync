package automergendjsonsync

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
		"Content-Type":      {ContentType},
		"Transfer-Encoding": {"chunked"},
	})
	assertEqual(t, rw.Body.String(), "{\"event\":\"sync\",\"data\":\"QgAAAQAAAA==\"}\n")
}

func TestServe_exchange(t *testing.T) {
	t.Parallel()
	sd := NewSharedDoc(automerge.New())

	// create some fake data
	assertEqual(t, sd.Doc().RootMap().Set("a", "b"), nil)
	_, _ = sd.Doc().Commit("change")

	rw := httptest.NewRecorder()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	req := httptest.NewRequestWithContext(ctx, http.MethodPut, "/", io.NopCloser(strings.NewReader("{\"event\":\"sync\",\"data\":\"QgAAAQAAAA==\"}\n")))
	assertEqual(t, sd.ServeChanges(rw, req), nil)
	assertEqual(t, rw.Result().StatusCode, http.StatusOK)
	assertEqual(t, rw.Result().Header, map[string][]string{
		"Content-Type":      {ContentType},
		"Transfer-Encoding": {"chunked"},
	})
	lines := strings.Split(strings.TrimSpace(rw.Body.String()), "\n")
	assertEqual(t, len(lines), 2)
}
