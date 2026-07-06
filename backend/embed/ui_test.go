package embed

import (
	"errors"
	"io/fs"
	"strings"
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
		var pathErr *fs.PathError
		if !errors.As(err, &pathErr) {
			t.Fatalf("ReadDir(ui) unexpected error: %v", err)
		}
		t.Fatalf("ReadDir(ui) PathError: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("ui directory is empty; expected embedded assets")
	}
}

// TestUIStaticFilesHaveCoreAssets verifies the files the runtime
// depends on. Three are loaded by the router (index.html,
// favicon.ico, speedtest_worker.js); placeholder.html is a
// compile-time fixture, not a runtime asset, but its absence
// breaks //go:embed and thus the entire build. We assert on it
// here so a regression that deletes it is caught at unit-test
// time instead of at the next CI build.
//
// This test cannot distinguish between the placeholder and a
// real UI artifact -- both filenames are identical. The real
// guard is in the build pipeline (CI runs build-ui before
// backend-lint), not in unit tests.
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

// TestPlaceholderHTMLIsMarkerAsset locks the contract that
// placeholder.html is the compile-time marker file. We assert on
// the size and content fingerprint so a future contributor who
// rewrites the placeholder to ship a half-built UI by accident
// gets caught here. If the placeholder is intentionally replaced
// with a different marker, update both this test and the comment
// in commit 77bae70 that introduced the file.
//
// Line endings are normalised before comparison: the repository
// ships the file with LF, but on Windows checkouts with
// core.autocrlf=true Git rewrites it to CRLF before the Go
// embed directive captures it. The marker contract is "a minimal
// stub", not "LF exactly", so we accept any of LF, CRLF, or CR.
func TestPlaceholderHTMLIsMarkerAsset(t *testing.T) {
	data, err := fs.ReadFile(UIStaticFiles, "ui/placeholder.html")
	if err != nil {
		t.Fatalf("placeholder.html missing: %v", err)
	}
	const want = "<!doctype html>\n<meta charset=utf-8>\n<title>ALS</title>\n<p>UI not built — run the frontend build (npm run build in ui/) in CI.</p>\n"
	if normaliseEOL(string(data)) != want {
		t.Errorf("placeholder.html content drifted; the marker asset must remain a minimal stub so it does not masquerade as a real UI build. got %q want %q", string(data), want)
	}
}

// normaliseEOL collapses \r\n and bare \r to \n so byte-for-byte
// comparison is independent of platform-specific line endings.
func normaliseEOL(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}
