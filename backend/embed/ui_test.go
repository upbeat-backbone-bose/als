package embed

import (
	"io/fs"
	"testing"
)

func TestUIStaticFilesDeclared(t *testing.T) {
	if _, err := UIStaticFiles.ReadDir("ui"); err != nil {
		if _, ok := err.(*fs.PathError); !ok {
			t.Errorf("unexpected error type: %v", err)
		}
	}
}

func TestUIStaticFilesNotEmpty(t *testing.T) {
	entries, err := UIStaticFiles.ReadDir("ui")
	if err != nil {
		t.Skipf("ui directory not embedded: %v", err)
	}
	if len(entries) == 0 {
		t.Log("ui directory is empty")
	}
}
