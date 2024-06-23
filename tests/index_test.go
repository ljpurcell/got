package tests

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	got "github.com/ljpurcell/got/internal"
)

type bufferReadWriteTruncate bytes.Buffer

func (b bufferReadWriteTruncate) Read(p []byte) (int, error) {
	buff := bytes.Buffer(b)
	return buff.Read(p)
}

func (b bufferReadWriteTruncate) Write(p []byte) (int, error) {
	buff := bytes.Buffer(b)
	return buff.Write(p)
}

func (b bufferReadWriteTruncate) Truncate(n int) error {
	if n < 0 {
		return errors.New("cannot truncate buffer to less than 0")
	}

	buff := bytes.Buffer(b)
	if buff.Len() < n {
		return fmt.Errorf("cannot truncate buffer of size %d to %d", buff.Len(), n)
	}

	buff.Truncate(0)
	return nil
}

func TestGetIndex(t *testing.T) {
	_, err := got.GetIndex()
	if err == nil {
		t.Fatal("index has not been initialised, should return error")
	}

	var b bufferReadWriteTruncate
	got.InitIndex(b)

	_, err = got.GetIndex()
	if err != nil {
		t.Fatalf("could not get index: %s", err)
	}
}

func TestUpdateIndex(t *testing.T) {
	var b bufferReadWriteTruncate
	got.InitIndex(b)

	index, err := got.GetIndex()
	if err != nil {
		t.Fatalf("could not get index: %s", err)
	}

	if len(index.Entries()) != 0 {
		t.Fatalf("index entries should be empty but contains: %v", index.Entries())
	}

	testdata, err := getTestDataDirectory(t)

	file, err := os.CreateTemp(testdata, "test_update_index_")
	if err != nil {
		t.Fatalf("could not create temp file: %s", err)
	}
	defer os.Remove(file.Name())

	if _, err = file.Write([]byte("Hello")); err != nil {
		t.Fatalf("could not write to temp file: %s", err)
	}

	if err = index.UpdateOrAddEntry(file.Name()); err != nil {
		t.Fatalf("could not update index: %s", err)
	}

	if len(index.Entries()) != 1 {
		t.Fatalf("index entries should have one item but contains: %v", index.Entries())
	}

	if ok, _ := index.IncludesFile(file.Name()); !ok {
		t.Fatalf("index entries should include %s but does not", file.Name())
	}
}

func TestClearIndex(t *testing.T) {
	var b bufferReadWriteTruncate
	got.InitIndex(b)

	index, err := got.GetIndex()
	if err != nil {
		t.Fatalf("could not get index: %s", err)
	}

	testdata, err := getTestDataDirectory(t)

	file, err := os.CreateTemp(testdata, "test_clear_index_")
	if err != nil {
		t.Fatalf("could not create temp file: %s", err)
	}
	defer os.Remove(file.Name())

	if _, err = file.Write([]byte("Hello")); err != nil {
		t.Fatalf("could not write to temp file: %s", err)
	}

	if err = index.UpdateOrAddEntry(file.Name()); err != nil {
		t.Fatalf("could not update index: %s", err)
	}

	if err = index.Clear(); err != nil {
		t.Fatalf("could not clear index: %s", err)
	}

	if ok, _ := index.IncludesFile(file.Name()); ok {
		t.Fatalf("index entries should not include %s but does", file.Name())
	}
	if len(index.Entries()) != 0 {
		t.Fatalf("index entries should be empty but contains: %v", index.Entries())
	}
}

// func TestSaveIndex(t *testing.T) {
// }
//
//
// func TestAddToIndex(t *testing.T) {
// }
//
// func TestIndexIncludes(t *testing.T) {
// }
//
// func TestRemoveFromIndex(t *testing.T) {
// }

func getTestDataDirectory(t *testing.T) (path string, err error) {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("could not get working directory: %v", err)
	}

	projectDir := filepath.Dir(dir)
	return filepath.Join(projectDir, "testdata"), nil
}
