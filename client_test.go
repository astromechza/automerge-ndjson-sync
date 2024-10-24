package automergendjsonsync

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/automerge/automerge-go"
)

func TestMessageGenerator(t *testing.T) {
	t.Parallel()

	t.Run("can be cancelled immediately", func(t *testing.T) {
		t.Parallel()
		doc := automerge.New()
		state := automerge.NewSyncState(doc)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		hints := make(chan bool)
		wg := new(sync.WaitGroup)
		wg.Add(1)
		mg := newMessageGenerator(ctx, state, hints, wg)
		cancel()
		buff := new(bytes.Buffer)
		_, err := io.Copy(buff, mg)
		assertEqual(t, err, nil)
		assertEqual(t, len(strings.Split(strings.TrimSpace(buff.String()), "\n")), 1)
	})

	t.Run("can be closed by the reader", func(t *testing.T) {
		t.Parallel()
		doc := automerge.New()
		state := automerge.NewSyncState(doc)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		hints := make(chan bool)
		wg := new(sync.WaitGroup)
		wg.Add(1)
		mg := newMessageGenerator(ctx, state, hints, wg)
		assertEqual(t, mg.Close(), nil)
		buff := new(bytes.Buffer)
		_, err := io.Copy(buff, mg)
		assertEqual(t, err, io.ErrClosedPipe)
		assertEqual(t, buff.Len(), 0)
	})

	for _, hasPeer := range []bool{true, false} {
		t.Run(fmt.Sprintf("nominal hasPeer=%v", hasPeer), func(t *testing.T) {
			t.Parallel()

			doc := automerge.New()
			state := automerge.NewSyncState(doc)

			doc2 := automerge.New()
			state2 := automerge.NewSyncState(doc2)
			if hasPeer {
				v, _ := state2.GenerateMessage()
				_, _ = state.ReceiveMessage(v.Bytes())
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			hints := make(chan bool)
			wg := new(sync.WaitGroup)
			wg.Add(1)
			mg := newMessageGenerator(ctx, state, hints, wg)

			go func() {
				for i := 0; i < 10; i++ {
					_ = doc.RootMap().Set(strconv.Itoa(i), "hello")
					_, _ = doc.Commit("committed")
					hints <- true
				}
				cancel()
			}()

			changeCount := 0
			var lastHeads []automerge.ChangeHash
			sc := bufio.NewScanner(mg)
			for sc.Scan() {
				e := NdJson{}
				assertEqual(t, json.Unmarshal(sc.Bytes(), &e), nil)
				m, err := state2.ReceiveMessage(e.Data)
				assertEqual(t, err, nil)
				changeCount += len(m.Changes())
				lastHeads = m.Heads()
			}
			assertEqual(t, lastHeads, doc.Heads())
			if hasPeer {
				assertEqual(t, changeCount, 10)
			} else {
				assertEqual(t, changeCount, 0)
			}
		})
	}

}

func TestHttpPushPullChanges(t *testing.T) {
	t.Parallel()

	t.Run("non 200 response", func(t *testing.T) {
		doc := automerge.New()
		sd := NewSharedDoc(doc)
		assertErrorEqual(t, sd.HttpPushPullChanges(context.Background(), "https://localhost", WithHttpClient(HttpDoerFunc(func(request *http.Request) (*http.Response, error) {
			r := &http.Response{StatusCode: http.StatusBadRequest}
			return r, nil
		}))), "http request failed with status 400")
	})

	t.Run("err response", func(t *testing.T) {
		doc := automerge.New()
		sd := NewSharedDoc(doc)
		assertErrorEqual(t, sd.HttpPushPullChanges(context.Background(), "https://localhost", WithHttpClient(HttpDoerFunc(func(request *http.Request) (*http.Response, error) {
			return nil, &url.Error{Op: "thing", URL: "https://localhost", Err: net.ErrClosed}
		}))), "http request failed: thing \"https://localhost\": use of closed network connection")
	})

	t.Run("read until server close", func(t *testing.T) {
		doc := automerge.New()
		sd := NewSharedDoc(doc)
		assertEqual(t, sd.HttpPushPullChanges(context.Background(), "https://localhost", WithHttpClient(HttpDoerFunc(func(request *http.Request) (*http.Response, error) {
			assertEqual(t, request.URL.String(), "https://localhost")
			assertEqual(t, request.Header, map[string][]string{
				"Accept":            {ContentType},
				"Content-Type":      {ContentType},
				"Expect":            {"100-continue"},
				"Transfer-Encoding": {"chunked"},
			})

			sc := bufio.NewScanner(request.Body)
			// we should get at least the head line in the request body
			assertEqual(t, sc.Scan(), true)

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}))), nil)
	})

	t.Run("read until check passes", func(t *testing.T) {
		doc := automerge.New()
		sd := NewSharedDoc(doc)
		checkCalled := false
		assertEqual(t, sd.HttpPushPullChanges(context.Background(), "https://localhost", WithHttpClient(HttpDoerFunc(func(request *http.Request) (*http.Response, error) {
			sc := bufio.NewScanner(request.Body)
			// we should get at least the first message line in the request body
			assertEqual(t, sc.Scan(), true)

			doc2 := automerge.New()
			wg := new(sync.WaitGroup)
			wg.Add(1)
			mg2 := newMessageGenerator(context.Background(), automerge.NewSyncState(doc2), make(<-chan bool), wg)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       mg2,
			}, nil
		})), WithTerminationCheck(func(doc *automerge.Doc, m *automerge.SyncMessage) bool {
			checkCalled = true
			return true
		})), nil)
		assertEqual(t, checkCalled, true)
	})
}
