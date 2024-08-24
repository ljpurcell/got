package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func GetTestDataDirectory(t *testing.T) (path string, err error) {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("could not get working directory: %v", err)
	}

	return filepath.Join(filepath.Dir(dir), "testdata"), nil
}

func InitialiseRepoInTestDirectory(t *testing.T) (repo string, cleanup func(), err error) {
	t.Helper()
	td, err := GetTestDataDirectory(t)
	if err != nil {
		t.Fatalf("can not intialise repo in test directory: %s", err)
	}

	repo = filepath.Join(td, ".got")

	return repo, func() {
		err = os.RemoveAll(repo)
		t.Fatalf("could not remove repo in test directory: %s", err)
	}, nil
}
