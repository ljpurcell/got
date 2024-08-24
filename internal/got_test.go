package got

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ljpurcell/got/tests"
)

func TestInit(t *testing.T) {
	td, err := tests.GetTestDataDirectory(t)
	if err != nil {
		t.Fatalf("could not get testdata dir: %s", err)
	}

	if err = Init(td); err != nil {
		t.Fatalf("could not initialise repository: %s", err)
	}

	os.RemoveAll(filepath.Join(td, ".got"))
}
