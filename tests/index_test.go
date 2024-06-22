package tests

import (
	"bytes"
	"testing"

	got "github.com/ljpurcell/got/internal"
)

func TestGetIndex(t *testing.T) {
	_, err := got.GetIndex()
	if err == nil {
		t.Fatal("index has not been initialised, should return error")
	}

	var b bytes.Buffer
	got.InitIndex(&b)
	_, err = got.GetIndex()
	if err != nil {
		t.Fatalf("could not get index: %s", err)
	}
}

// func TestClearIndex(t *testing.T) {
// }
//
// func TestSaveIndex(t *testing.T) {
// }
//
// func TestUpdateIndex(t *testing.T) {
// }
//
// func TestAddToIndex(t *testing.T) {
// }
//
// func TestIndexIncludes(t *testing.T) {
// }
//
// func TestRemoveFromIndex(t *testing.T) {
// }
