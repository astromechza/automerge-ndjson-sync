package automergendjsonsync

import "github.com/automerge/automerge-go"

// TerminationCheck can be used on a message reader to stop reading messages when the local document and remote document
// are suitably in-sync. NoTerminationCheck will never stop reading.
type TerminationCheck func(doc *automerge.Doc, m *automerge.SyncMessage) bool

// NoTerminationCheck will continue reading all messages and not stop.
func NoTerminationCheck(doc *automerge.Doc, m *automerge.SyncMessage) bool {
	return false
}

var _ TerminationCheck = NoTerminationCheck

// CompareHeads is a utility function that compares change hash arrays and returns values indicating the intersection
// or overlap.
func CompareHeads(a []automerge.ChangeHash, b []automerge.ChangeHash) (missingInA int, missingInB int) {
	seen := make(map[string]bool, len(a))
	for _, head := range a {
		seen[head.String()] = true
	}
	for _, hash := range b {
		if _, ok := seen[hash.String()]; !ok {
			missingInA++
		} else {
			delete(seen, hash.String())
		}
	}
	missingInB = len(seen)
	return
}

// HeadsEqualCheck will continue accepting messages until both the local doc and remote doc have the same heads.
func HeadsEqualCheck(doc *automerge.Doc, m *automerge.SyncMessage) bool {
	a, b := CompareHeads(doc.Heads(), m.Heads())
	return a == 0 && b == 0
}

var _ TerminationCheck = HeadsEqualCheck

// HasAllRemoteHeads will continue accepting messages until it confirms that the local doc contains all the remote heads.
// But the opposite may not be true.
func HasAllRemoteHeads(doc *automerge.Doc, m *automerge.SyncMessage) bool {
	missingInLocal, _ := CompareHeads(doc.Heads(), m.Heads())
	return missingInLocal == 0
}

var _ TerminationCheck = HasAllRemoteHeads
