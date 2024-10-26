package automergendjsonsync

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"testing"

	"github.com/automerge/automerge-go"
)

// A key integration test that checks that we can synchronise 2 clients and a server together
func Test_sync3(t *testing.T) {
	t.Parallel()

	// Create a starting server side doc that has some existing content.
	sd := NewSharedDoc(automerge.New())
	assertEqual(t, sd.Doc().RootMap().Set("a", "b"), nil)
	_, _ = sd.Doc().Commit("change")

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if err := sd.ServeChanges(w, r); err != nil {
			t.Fatal(err)
		}
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	assertEqual(t, err, nil)
	server := &http.Server{Handler: mux}
	defer server.Close()
	goErrors := make(chan error, 3)
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			panic(err)
		}
	}()

	peerDocs := make([]*SharedDoc, 0)
	wg := new(sync.WaitGroup)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		// Each peer also has a starting doc that has some existing content.
		peerDoc := NewSharedDoc(automerge.New())
		peerDocs = append(peerDocs, peerDoc)
		assertEqual(t, peerDoc.Doc().RootMap().Set(fmt.Sprintf("peer-%d", i), int64(i)), nil)
		_, _ = peerDoc.Doc().Commit("change")
		go func() {
			defer wg.Done()
			if err := peerDoc.HttpPushPullChanges(context.Background(), "http://"+listener.Addr().String(), WithClientTerminationCheck(func(doc *automerge.Doc, m *automerge.SyncMessage) bool {
				return len(doc.Heads()) == 3
			})); err != nil {
				goErrors <- err
			}
		}()
	}

	wg.Wait()

	close(goErrors)
	for err2 := range goErrors {
		t.Error(err2.Error())
	}

	t.Log(LoggableChangeHashes(sd.Doc().Heads()).LogValue().String())
	t.Log(LoggableChangeHashes(peerDocs[0].Doc().Heads()).LogValue().String())
	t.Log(LoggableChangeHashes(peerDocs[1].Doc().Heads()).LogValue().String())
	assertEqual(t, sd.Doc().Heads(), peerDocs[0].Doc().Heads())
	assertEqual(t, sd.Doc().Heads(), peerDocs[1].Doc().Heads())
	assertEqual(t, sd.Doc().RootMap().Len(), 3)
	values, _ := sd.Doc().RootMap().Values()
	assertEqual(t, values["a"].Str(), "b")
	assertEqual(t, values["peer-0"].Int64(), 0)
	assertEqual(t, values["peer-1"].Int64(), 1)
}
