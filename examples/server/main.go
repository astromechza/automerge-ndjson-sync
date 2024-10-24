package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"os"
	"sync"
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

type CommitingResponseWriter struct {
	inner     http.ResponseWriter
	committed bool
}

func (c *CommitingResponseWriter) Header() http.Header {
	return c.inner.Header()
}

func (c *CommitingResponseWriter) Write(bytes []byte) (int, error) {
	return c.inner.Write(bytes)
}

func (c *CommitingResponseWriter) WriteHeader(statusCode int) {
	c.inner.WriteHeader(statusCode)

}

func (c *CommitingResponseWriter) Flush() {
	c.inner.(http.Flusher).Flush()
	c.committed = true
}

func (c *CommitingResponseWriter) Committed() bool {
	return c.committed
}

var _ http.Flusher = (*CommitingResponseWriter)(nil)
var _ http.ResponseWriter = (*CommitingResponseWriter)(nil)

func handlerWithErrors(inner func(writer http.ResponseWriter, request *http.Request) error) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		crw := &CommitingResponseWriter{inner: writer}
		if err := inner(crw, request); err != nil {
			// All errors are handled as internal server errors at this point but can be handled more specifically in
			// the future.
			committed := crw.Committed()
			slog.With(slog.Any("err", err), slog.Bool("committed", committed)).Error("error returned while handling request")
			if !committed {
				clear(crw.Header())
				crw.Header().Set("Content-Type", "text/plain; charset=utf-8")
				crw.WriteHeader(http.StatusInternalServerError)
				_, _ = crw.Write([]byte(fmt.Sprintf("internal server error: %s", err)))
			}
		}
	}
}

func mainInner() error {
	m := new(sync.Map)

	mux := http.NewServeMux()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	automergendjsonsync.SetLog(slog.Default())

	mux.HandleFunc("GET /{id}", handlerWithErrors(func(writer http.ResponseWriter, request *http.Request) error {
		docId := request.PathValue("id")
		docAny, ok := m.Load(docId)
		if !ok {
			return fmt.Errorf("document not found")
		}
		docShared := docAny.(*automergendjsonsync.SharedDoc)
		writer.Header().Set("Content-Type", "application/octet-stream")
		writer.WriteHeader(http.StatusOK)
		_, err := writer.Write(docShared.Doc().Save())
		return err
	}))

	mux.HandleFunc("PUT /{id}", handlerWithErrors(func(writer http.ResponseWriter, request *http.Request) error {
		docId := request.PathValue("id")
		docAny, _ := m.LoadOrStore(docId, automergendjsonsync.NewSharedDoc(automerge.New()))
		docShared := docAny.(*automergendjsonsync.SharedDoc)
		return docShared.ServeChanges(writer, request)
	}))

	// we want both http1 and http2 here. For http2 we need a tls cert.
	cert, err := generateSelfSignedCert()
	if err != nil {
		return err
	}
	server := &http.Server{Addr: ":8080", Handler: mux, TLSConfig: &tls.Config{
		Certificates: []tls.Certificate{*cert},
	}}
	return server.ListenAndServeTLS("", "")
}

func generateSelfSignedCert() (*tls.Certificate, error) {
	// Generate a new RSA private key
	pk, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	cert := &x509.Certificate{
		Subject: pkix.Name{
			Country:      []string{"GB"},
			Organization: []string{"Example Organization"},
			CommonName:   "localhost",
		},
		NotBefore:   time.Now(),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}
	cert.NotAfter = cert.NotBefore.Add(30 * 24 * time.Hour)
	if cert.SerialNumber, err = rand.Int(rand.Reader, big.NewInt(1<<62)); err != nil {
		return nil, err
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, cert, cert, &pk.PublicKey, pk)
	if err != nil {
		return nil, err
	}
	return &tls.Certificate{Certificate: [][]byte{derBytes}, PrivateKey: pk}, nil
}
