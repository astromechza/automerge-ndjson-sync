package automergendjsonsync

import "github.com/automerge/automerge-go"

// A ReadPredicate is used to filter messages before receiving them in the doc.
// The function should return a bool (true=include, false=exclude/skip).
// An error will cause the sync to abort.
type ReadPredicate func(doc *automerge.Doc, msg *automerge.SyncMessage) (bool, error)

// NoReadPredicate is a ReadPredicate which includes all messages
func NoReadPredicate(doc *automerge.Doc, msg *automerge.SyncMessage) (bool, error) {
	return true, nil
}

// SkipChangesReadPredicate is a ReadPredicate which skips any messages that contain changes. Effectively turning the
// doc read only since it does not accept incoming changes but is happy to doll them out.
func SkipChangesReadPredicate(doc *automerge.Doc, msg *automerge.SyncMessage) (bool, error) {
	return len(msg.Changes()) == 0, nil
}
