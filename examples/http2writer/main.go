package main

import (
	"context"
	"crypto/tls"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"time"

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
	automergendjsonsync.SetLog(slog.Default())

	randomDocId := strconv.Itoa(rand.Int())
	if v := os.Getenv("DOC_ID"); v != "" {
		randomDocId = v
	}
	slog.Info("random doc id decided", slog.String("id", randomDocId))

	doc := automerge.New()
	a := automergendjsonsync.NewSharedDoc(doc)

	go func() {
		t := time.NewTicker(time.Second)
		for range t.C {
			slog.Debug("mutating")
			if err := doc.RootMap().Set("foo", strconv.Itoa(rand.Int())); err != nil {
				panic(err)
			} else if _, err := doc.Commit("commit"); err != nil {
				panic(err)
			}
			a.NotifyPossibleChanges()
		}
	}()

	hc := &http.Client{
		Transport: &http.Transport{
			ForceAttemptHTTP2: true,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	return a.HttpPushPullChanges(context.TODO(), "https://localhost:8080/"+randomDocId, automergendjsonsync.WithHttpClient(hc), automergendjsonsync.WithTerminationCheck(automergendjsonsync.NoTerminationCheck))
}
