package main

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
)

func TestHashBlobIdGeneration(t *testing.T) {
	file, err := os.CreateTemp(".", "test_file_for_hash_blob")
	if err != nil {
		t.Fatalf("Could not create temp file: %v", err)
	}
	defer cleanUpTempFile(file)

	text := []byte("These are some bytes to be written")
	file.Write(text)
	info, err := os.Stat(file.Name())

	toHash := fmt.Sprintf("blob %d\u0000", info.Size())
	hash := sha1.New()
	hash.Write([]byte(toHash))
	expected := hex.EncodeToString(hash.Sum(nil))

	result := hashBlob(file.Name(), false)

	if result != expected {
		t.Errorf("\nExp: %v. Actual: %v\n", expected, result)
	}
}

func TestHashBlobCreatesObjectFile(t *testing.T) {
	file, err := os.CreateTemp(".", "test_file_for_hash_blob")
	if err != nil {
		t.Fatalf("Could not create temp file: %v", err)
	}
	defer cleanUpTempFile(file)

	text := []byte("These are some bytes to be written")
	file.Write(text)
	id := hashBlob(file.Name(), true)

	blobDir := filepath.Join(GOT_REPO, "objects", id[:2])
	blobFile := filepath.Join(blobDir, id[2:])

	if !exists(blobDir) || !isDir(blobDir) {
		t.Errorf("No directory for blob %v at %v", id, blobDir)
	}

	if !exists(blobFile) && !isDir(blobDir) {
		t.Errorf("No file for blob %v at %v in %v", id, blobFile, blobDir)
	}
}

func TestHashBlobCompressesFileContents(t *testing.T) {
	file, err := os.CreateTemp(".", "test_file_for_hash_blob")
	if err != nil {
		t.Fatalf("Could not create temp file: %v", err)
	}
	defer cleanUpTempFile(file)

	text := "These are some bytes to be written"
	file.Write([]byte(text))
	id := hashBlob(file.Name(), true)

	blobFile := filepath.Join(GOT_REPO, "objects", id[:2], id[2:])

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

	result := string(out)

	if result != text {
		t.Fatalf("Exp: %v. Actual: %v", text, result)
	}

}

func cleanUpTempFile(file *os.File) {
	file.Close()
	os.Remove(file.Name())
}
