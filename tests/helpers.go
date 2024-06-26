package tests

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
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

func (b bufferReadWriteTruncate) Truncate(n int64) error {
	if n < 0 {
		return errors.New("cannot truncate buffer to less than 0")
	}

	buff := bytes.Buffer(b)
	if buff.Len() < int(n) {
		return fmt.Errorf("cannot truncate buffer of size %d to %d", buff.Len(), n)
	}

	buff.Truncate(0)
	return nil
}

func getTestDataDirectory(t *testing.T) (path string, err error) {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("could not get working directory: %v", err)
	}

	projectDir := filepath.Dir(dir)
	return filepath.Join(projectDir, "testdata"), nil
}
