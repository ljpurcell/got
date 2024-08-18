package tests

import (
	"os"
	"testing"

	got "github.com/ljpurcell/got/internal"
)

func TestGetIndex(t *testing.T) {
	_, err := got.GetIndex()
	if err == nil {
		t.Fatal("index has not been initialised, should return error")
	}

	_, err = got.GetIndex()
	if err != nil {
		t.Fatalf("could not get index: %s", err)
	}
}

func TestIndexIncludes(t *testing.T) {
	index, err := got.GetIndex()
	if err != nil {
		t.Fatalf("could not get index: %s", err)
	}

	testdata, err := getTestDataDirectory(t)

	file, err := os.CreateTemp(testdata, "test_index_includes_")
	if err != nil {
		t.Fatalf("could not create temp file: %s", err)
	}
	defer os.Remove(file.Name())

	if _, err = file.Write([]byte("Hello")); err != nil {
		t.Fatalf("could not write to temp file: %s", err)
	}

	if err = index.UpdateOrAddEntry(file.Name()); err != nil {
		t.Fatalf("could not add to index: %s", err)
	}

	if ok, _ := index.IncludesFile(file.Name()); !ok {
		t.Fatalf("index entries should include %s but does not", file.Name())
	}
}

func TestAddToIndex(t *testing.T) {
	index, err := got.GetIndex()
	if err != nil {
		t.Fatalf("could not get index: %s", err)
	}

	if len(index.Entries()) != 0 {
		t.Fatalf("index entries should be empty but contains: %v", index.Entries())
	}

	testdata, err := getTestDataDirectory(t)

	file, err := os.CreateTemp(testdata, "test_add_to_index_")
	if err != nil {
		t.Fatalf("could not create temp file: %s", err)
	}
	defer os.Remove(file.Name())

	if _, err = file.Write([]byte("Hello")); err != nil {
		t.Fatalf("could not write to temp file: %s", err)
	}

	if err = index.UpdateOrAddEntry(file.Name()); err != nil {
		t.Fatalf("could not add to index: %s", err)
	}

	if len(index.Entries()) != 1 {
		t.Fatalf("index entries should have one item but contains: %v", index.Entries())
	}

	entry := index.Entries()[0]

	if entry.Status != got.STATUS_ADD {
		t.Fatalf("%s should show status %s, instead showing %s", file.Name(), got.STATUS_MODIFY, entry.Status)
	}

	if ok, _ := index.IncludesFile(file.Name()); !ok {
		t.Fatalf("index entries should include %s but does not", entry.Name)
	}
}

func TestUpdateIndex(t *testing.T) {
	index, err := got.GetIndex()
	if err != nil {
		t.Fatalf("could not get index: %s", err)
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

	if _, err = file.Write([]byte("Goodbye")); err != nil {
		t.Fatalf("could not write to temp file: %s", err)
	}

	entry := index.Entries()[0]

	if entry.Status != got.STATUS_MODIFY {
		t.Fatalf("%s should show status %s, instead showing %s", entry.Name, got.STATUS_MODIFY, entry.Status)
	}

	if ok, _ := index.IncludesFile(file.Name()); !ok {
		t.Fatalf("index entries should include %s but does not", file.Name())
	}
}

func TestClearIndex(t *testing.T) {
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
// func TestRemoveFromIndex(t *testing.T) {
// }
