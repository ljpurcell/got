package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func getTestDataDirectory(t *testing.T) (path string, err error) {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("could not get working directory: %v", err)
	}

	return filepath.Join(filepath.Dir(dir), "testdata"), nil
}
