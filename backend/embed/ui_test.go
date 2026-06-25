package embed

import (
	"io/fs"
	"testing"
)

// TestUIStaticFilesDeclared verifies that the embed.FS is reachable
// and the "ui" directory is queryable. The embed directive
// `//go:embed ui` always produces a non-nil FS, so the real assertion
// is that the directory contents are readable.
func TestUIStaticFilesDeclared(t *testing.T) {
	entries, err := UIStaticFiles.ReadDir("ui")
	if err != nil {
		// A non-PathError is a real failure; PathError is acceptable
		// only if the embed directive was not run (unlikely since the
		// build succeeded).
		if _, ok := err.(*fs.PathError); !ok {
			t.Fatalf("ReadDir(ui) unexpected error: %v", err)
		}
		t.Fatalf("ReadDir(ui) PathError: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("ui directory is empty; expected embedded assets")
	}
}

// TestUIStaticFilesHaveCoreAssets verifies the four files that the
// router relies on (index.html, favicon.ico, speedtest_worker.js,
// placeholder.html) are all embedded. A regression that drops one
// of these would silently break the UI.
func TestUIStaticFilesHaveCoreAssets(t *testing.T) {
	required := []string{
		"ui/index.html",
		"ui/favicon.ico",
		"ui/speedtest_worker.js",
		"ui/placeholder.html",
	}
	for _, name := range required {
		if _, err := fs.ReadFile(UIStaticFiles, name); err != nil {
			t.Errorf("missing embedded asset %q: %v", name, err)
		}
	}
}
