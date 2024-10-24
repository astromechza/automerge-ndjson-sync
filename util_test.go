package automergendjsonsync

import (
	"reflect"
	"testing"
)

func assertEqual[k any](t *testing.T, actual, expected k) bool {
	t.Helper()
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("actual = %v, expected = %v", actual, expected)
		return false
	}
	return true
}

func assertErrorEqual(t *testing.T, actual error, expected string) bool {
	t.Helper()
	if actual == nil {
		t.Errorf("expected an error, got nil")
		return false
	} else if actual.Error() != expected {
		t.Errorf("expected error: %v to equal %s", actual.Error(), expected)
		return false
	}
	return true
}
