package automergendjsonsync

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"
	"testing/iotest"
	"time"

	"github.com/automerge/automerge-go"
)

func TestNotifyPossibleChanges_no_subs(t *testing.T) {
	t.Parallel()
	a := NewSharedDoc(automerge.New())
	a.NotifyReceivedChanges()
}

func TestNotifyPossibleChanges_with_sub(t *testing.T) {
	t.Parallel()

	a := NewSharedDoc(automerge.New())
	sub, fin := a.SubscribeToReceivedChanges()

	t.Run("no message available by default", func(t *testing.T) {
		select {
		case <-sub:
			t.Fatal("no message should be available")
		default:
		}
	})

	t.Run("message available after notify", func(t *testing.T) {
		a.NotifyReceivedChanges()

		select {
		case v := <-sub:
			assertEqual(t, true, v)
		case <-time.After(time.Second * 10):
			t.Fatal("timed out waiting for notification")
		}
	})

	t.Run("closer function close channel", func(t *testing.T) {
		fin()
		a.NotifyReceivedChanges()
		_, ok := <-sub
		assertEqual(t, false, ok)
	})
}

func TestConsumeMessagesFromReader_empty(t *testing.T) {
	t.Parallel()
	sd := NewSharedDoc(automerge.New())
	n, err := sd.consumeMessagesFromReader(context.Background(), automerge.NewSyncState(sd.Doc()), new(bytes.Buffer), NoTerminationCheck)
	assertEqual(t, err, nil)
	assertEqual(t, n, 0)
}

func TestConsumeMessagesFromReader_ping_message(t *testing.T) {
	t.Parallel()
	sd := NewSharedDoc(automerge.New())
	buff := bytes.NewBuffer([]byte(`{"event": "ping"}
`))
	n, err := sd.consumeMessagesFromReader(context.Background(), automerge.NewSyncState(sd.Doc()), buff, NoTerminationCheck)
	assertEqual(t, err, nil)
	assertEqual(t, n, 0)
	assertEqual(t, buff.Len(), 0)
}

func TestConsumeMessagesFromReader_read_err(t *testing.T) {
	t.Parallel()
	sd := NewSharedDoc(automerge.New())
	buff := iotest.ErrReader(io.ErrUnexpectedEOF)
	n, err := sd.consumeMessagesFromReader(context.Background(), automerge.NewSyncState(sd.Doc()), buff, NoTerminationCheck)
	assertErrorEqual(t, err, "failed while scanning message 1: unexpected EOF")
	assertEqual(t, n, 0)
}

func TestConsumeMessagesFromReader_not_json(t *testing.T) {
	t.Parallel()
	sd := NewSharedDoc(automerge.New())
	buff := bytes.NewBuffer([]byte(`bad`))
	n, err := sd.consumeMessagesFromReader(context.Background(), automerge.NewSyncState(sd.Doc()), buff, NoTerminationCheck)
	assertErrorEqual(t, err, "failed to unmarshal message 1: invalid character 'b' looking for beginning of value")
	assertEqual(t, n, 0)
}

func TestConsumeMessagesFromReader_bad_data(t *testing.T) {
	t.Parallel()
	sd := NewSharedDoc(automerge.New())
	buff := bytes.NewBuffer([]byte(`{"event":"sync"}`))
	n, err := sd.consumeMessagesFromReader(context.Background(), automerge.NewSyncState(sd.Doc()), buff, NoTerminationCheck)
	assertErrorEqual(t, err, "failed to receive message 1: not enough input")
	assertEqual(t, n, 0)
}

func TestConsumeMessagesFromReader_termination_check(t *testing.T) {
	t.Parallel()

	sd := NewSharedDoc(automerge.New())
	buff := new(bytes.Buffer)

	{
		ss := automerge.NewSyncState(sd.Doc())
		m, _ := ss.GenerateMessage()
		raw, _ := json.Marshal(&NdJson{Event: EventSync, Data: m.Bytes()})
		raw = append(raw, '\n')
		for i := 0; i < 10; i++ {
			_, _ = buff.Write(raw)
		}
	}
	called := 0
	n, err := sd.consumeMessagesFromReader(context.Background(), automerge.NewSyncState(sd.Doc()), buff, func(doc *automerge.Doc, m *automerge.SyncMessage) bool {
		called += 1
		return called >= 2
	})
	assertEqual(t, err, nil)
	assertEqual(t, called, 2)
	assertEqual(t, n, 2)
}
