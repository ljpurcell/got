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

const text string = "These are some bytes to be written"

func TestHashObjectForBlob(t *testing.T) {
	file, err := createTempFileWithText(text)
	defer cleanUpTempFile(file)
	info, err := os.Stat(file.Name())

	toHash := fmt.Sprintf("blob %d\u0000%v", info.Size(), text)
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

func TestIndex(t *testing.T) {
	index := got.Index{}

	if index.Length() != 0 {
		t.Errorf("Index length should be zero")
	}

	file, err := createTempFileWithText(text)
	if err != nil {
		t.Fatalf("Could not create temp tile in TestIndex: %v", err)
	}

	index.UpdateOrAddFromFile(file.Name())

	if index.Length() != 1 {
		t.Errorf("Index length should be one")
	}

	file.WriteString("This is some different text")

	index.UpdateOrAddFromFile(file.Name())

	if index.Length() != 1 {
		t.Errorf("Index length should be one")
	}

	oldName, newName := file.Name(), "NEW_NAME"
	if err := os.Rename(oldName, newName); err != nil {
		t.Fatalf("Could not rename %q to %q: %v", file.Name(), "NEW_NAME", err)
	}

	index.UpdateOrAddFromFile(newName)

	if index.Length() != 2 {
		t.Errorf("Index length should be two")
	}

	index.RemoveFile(oldName)

	if index.Length() != 1 {
		t.Errorf("Index length should be two")
	}


}

func createTempFileWithText(text string) (*os.File, error) {
	file, err := os.CreateTemp(".", "temp_file_for_testing")
	if err != nil {
		return nil, fmt.Errorf("Could not create temp file: %v", err)
	}

	data := []byte(text)
	file.Write(data)

	return file, nil
}

func cleanUpTempFile(file *os.File) {
	file.Close()
	os.Remove(file.Name())
}
