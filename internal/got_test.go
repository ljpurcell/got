package got

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	cfg "github.com/ljpurcell/got/internal/config"
	"github.com/ljpurcell/got/internal/utils"
)

const text string = "These are some bytes to be written"

func TestHashObjectForBlob(t *testing.T) {
	tempDir := t.TempDir()
	file, err := os.CreateTemp(tempDir, "temp_file_for_testing")
	if err != nil {
		t.Fatalf("Could not create temp file: %v", err)
	}

	file.Write([]byte(text))
	info, err := os.Stat(file.Name())

	expectedType := BLOB
	toHash := fmt.Sprintf("%v %d %v", expectedType, info.Size(), text)
	hash := sha1.New()
	hash.Write([]byte(toHash))
	expectedId := hex.EncodeToString(hash.Sum(nil))

	id, objectType := HashObject(file.Name())

	if id != expectedId {
		t.Errorf("\nExp: %v. Actual: %v\n", expectedId, id)
	}

	if objectType != expectedType {
		t.Errorf("\nExp: %v. Actual: %v\n", expectedType, objectType)
	}

	blobDir := filepath.Join(cfg.GOT_REPO, "objects", id[:2])
	blobFile := filepath.Join(blobDir, id[2:])

	if !utils.Exists(blobDir) || !utils.IsDir(blobDir) {
		t.Errorf("No directory for blob %v at %v", id, blobDir)
	}

	if !utils.Exists(blobFile) && !utils.IsDir(blobDir) {
		t.Errorf("No file for blob %v at %v in %v", id, blobFile, blobDir)
	}

	fileContents, err := os.ReadFile(blobFile)
	if err != nil {
		t.Fatalf("Could not read contents from %v for decompression", blobFile)
	}

	data := bytes.NewReader(fileContents)
	dec, err := zlib.NewReader(data)
	if err != nil {
		t.Fatalf("Could not create ZLIB reader: %v", err)
	}
	dec.Close()

	out, err := io.ReadAll(dec)

	if string(out) != string(text) {
		t.Errorf("Exp: %v. Actual: %v", string(text), string(out))
	}

	t.Cleanup(func() {
		file.Close()
		os.Remove(file.Name())
	})
}

func TestHashObjectForTree(t *testing.T) {
	tempDir := t.TempDir()
	file, err := os.CreateTemp(tempDir, "temp_file_for_testing")
	if err != nil {
		t.Fatalf("Could not create temp file: %v", err)
	}

	file.Write([]byte(text))

	HashObject(file.Name())

	files, err := os.ReadDir(tempDir)

	tree := ""
	for _, f := range files {
		filePath := filepath.Join(tempDir, f.Name())
		if f.IsDir() {
			treeId := writeTree(filePath)
			tree += fmt.Sprintf("%v tree %v %v\n", 100644, treeId, f.Name())
		} else {
			blobId := writeBlob(filePath)
			tree += fmt.Sprintf("%v blob %v %v\n", 100644, blobId, f.Name())
		}
	}

	expectedType := TREE
	size := len(tree)
	objString := fmt.Sprintf("%v %d %v", expectedType, size, tree)
	hasher := sha1.New()
	hasher.Write([]byte(objString))
	expectedId := hex.EncodeToString(hasher.Sum(nil))

	id, objectType := HashObject(tempDir)

	if id != expectedId {
		t.Errorf("\nExp: %v. Actual: %v\n", expectedId, id)
	}

	if objectType != expectedType {
		t.Errorf("\nExp: %v. Actual: %v\n", expectedType, objectType)
	}

	treeDir := filepath.Join(cfg.GOT_REPO, "objects", id[:2])
	treeFile := filepath.Join(treeDir, id[2:])

	if !utils.Exists(treeDir) || !utils.IsDir(treeDir) {
		t.Errorf("No directory for tree %v at %v", id, treeDir)
	}

	if !utils.Exists(treeFile) && !utils.IsDir(treeDir) {
		t.Errorf("No file for tree %v at %v in %v", id, treeFile, treeDir)
	}

	fileContents, err := os.ReadFile(treeFile)
	if err != nil {
		t.Fatalf("Could not read contents from %v for decompression", treeFile)
	}

	data := bytes.NewReader(fileContents)
	dec, err := zlib.NewReader(data)
	if err != nil {
		t.Fatalf("Could not create ZLIB reader: %v", err)
	}
	dec.Close()

	out, err := io.ReadAll(dec)

	if string(out) != string(text) {
		// TODO
	}

	t.Cleanup(func() {
		file.Close()
		os.Remove(file.Name())
	})
}

func TestIndex(t *testing.T) {
	os.Remove(cfg.INDEX_FILE)
	index := GetIndex()

	if index.Length() != 0 {
		t.Errorf("Index length should be zero")
	}

	tempDir := t.TempDir()
	file, err := os.CreateTemp(tempDir, "temp_file_for_testing")
	if err != nil {
		t.Fatalf("Could not create temp file: %v", err)
	}

	file.Write([]byte(text))

	index.UpdateOrAddEntry(file.Name())

	if index.Length() != 1 {
		t.Errorf("Index length should be one")
	}

	file.WriteString("This is some different text")

	index.UpdateOrAddEntry(file.Name())

	if index.Length() != 1 {
		t.Errorf("Index length should be one")
	}

	oldName, newName := file.Name(), "NEW_NAME"
	if err := os.Rename(oldName, newName); err != nil {
		t.Fatalf("Could not rename %q to %q: %v", oldName, newName, err)
	}

	index.UpdateOrAddEntry(newName)

	if index.Length() != 2 {
		t.Errorf("Index length should be two")
	}

	index.RemoveFile(oldName)

	if index.Length() != 1 {
		t.Errorf("Index length should be two")
	}

	if utils.Exists(cfg.INDEX_FILE) {
		os.Remove(cfg.INDEX_FILE)
	}

	index.Save()

	if !utils.Exists(cfg.INDEX_FILE) {
		t.Fatalf("Index file does not exist after calling Save method")
	}

	t.Cleanup(func() {
		file.Close()
		os.Remove(file.Name())
	})
}

func BenchmarkHashBlob(b *testing.B) {
	pwd, err := os.Getwd()
	if err != nil {
		b.Fatalf("Could not get current directory: %v", err)
	}

	parent := filepath.Dir(pwd)
	tFile := filepath.Join(parent, "testdata", "test_file.txt")

	for i := 0; i < b.N; i++ {
		writeBlob(tFile)
	}
}

func BenchmarkHashTree(b *testing.B) {
	pwd, err := os.Getwd()
	if err != nil {
		b.Fatalf("Could not get current directory: %v", err)
	}

	parent := filepath.Dir(pwd)
	testdata := filepath.Join(parent, "testdata")

	for i := 0; i < b.N; i++ {
		writeTree(testdata)
	}
}
