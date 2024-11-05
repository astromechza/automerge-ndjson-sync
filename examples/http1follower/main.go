package main

import (
	"context"
	"crypto/tls"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"

	"github.com/automerge/automerge-go"

	"github.com/astromechza/automerge-ndjson-sync"
)

func main() {
	if err := mainInner(); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func mainInner() error {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))

	randomDocId := strconv.Itoa(rand.Int())
	if v := os.Getenv("DOC_ID"); v != "" {
		randomDocId = v
	}
	slog.Info("random doc id decided", slog.String("id", randomDocId))

	doc := automerge.New()
	for i := 0; i < 10; i++ {
		if err := doc.RootMap().Set("foo", strconv.Itoa(rand.Int())); err != nil {
			panic(err)
		} else if _, err := doc.Commit("commit"); err != nil {
			panic(err)
		}
	}

	a := automergendjsonsync.NewSharedDoc(doc)

	hc := &http.Client{
		Transport: &http.Transport{
			ForceAttemptHTTP2: false,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	ctx := automergendjsonsync.SetContextLogger(context.TODO(), slog.Default())
	return a.HttpPushPullChanges(ctx, "https://localhost:8080/"+randomDocId, automergendjsonsync.WithHttpClient(hc), automergendjsonsync.WithClientTerminationCheck(automergendjsonsync.HeadsEqualCheck))
}
