package automergendjsonsync

import (
	"testing"

	"github.com/automerge/automerge-go"
)

func TestCompareHeads(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		x, y := CompareHeads([]automerge.ChangeHash{}, []automerge.ChangeHash{})
		assertEqual(t, x, 0)
		assertEqual(t, y, 0)
	})

	t.Run("empty_left", func(t *testing.T) {
		x, y := CompareHeads([]automerge.ChangeHash{}, []automerge.ChangeHash{{}})
		assertEqual(t, x, 1)
		assertEqual(t, y, 0)
	})

	t.Run("empty_right", func(t *testing.T) {
		x, y := CompareHeads([]automerge.ChangeHash{{}}, []automerge.ChangeHash{})
		assertEqual(t, x, 0)
		assertEqual(t, y, 1)
	})

	t.Run("nominal", func(t *testing.T) {
		x, y := CompareHeads([]automerge.ChangeHash{{0}, {1}}, []automerge.ChangeHash{{1}, {0}})
		assertEqual(t, x, 0)
		assertEqual(t, y, 0)
	})
}

func TestHeadsEqualCheck(t *testing.T) {
	t.Parallel()

	t.Run("true", func(t *testing.T) {
		doc := automerge.New()
		_ = doc.RootMap().Set("a", "b")
		_, _ = doc.Commit("done")
		ss := automerge.NewSyncState(doc)
		m, _ := ss.GenerateMessage()
		assertEqual(t, HeadsEqualCheck(doc, m), true)
	})

	t.Run("false", func(t *testing.T) {
		doc := automerge.New()
		_ = doc.RootMap().Set("a", "b")
		_, _ = doc.Commit("done")
		ss := automerge.NewSyncState(doc)
		m, _ := ss.GenerateMessage()
		_ = doc.RootMap().Set("a", "c")
		_, _ = doc.Commit("done")
		assertEqual(t, HeadsEqualCheck(doc, m), false)
	})
}
