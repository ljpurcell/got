package got_test

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

	"github.com/ljpurcell/got/internal"
	cfg "github.com/ljpurcell/got/internal/config"
	"github.com/ljpurcell/got/internal/utils"
)

func TestHashObjectForBlob(t *testing.T) {
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
	expectedId := hex.EncodeToString(hash.Sum(nil))
	expectedType := "blob"

	id, objectType := got.HashObject(file.Name())

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
}

func cleanUpTempFile(file *os.File) {
	file.Close()
	os.Remove(file.Name())
}
