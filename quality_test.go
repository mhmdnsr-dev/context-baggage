package contextbaggage

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// maxHandwrittenGoLines is the hard upper bound on physical lines for a
// handwritten Go source file. It is a guardrail against files that accumulate
// unrelated responsibilities; it is not a target to approach.
const maxHandwrittenGoLines = 500

// TestHandwrittenGoFilesStayWithinSizeLimit fails when a handwritten Go source
// file exceeds the repository's hard file-size limit. Generated files are
// skipped using Go's standard generated-code convention. The test runs under
// `go test ./...` and therefore under normal CI, so no separate tool is needed.
func TestHandwrittenGoFilesStayWithinSizeLimit(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository root")
	}
	root := filepath.Dir(file)
	scanned := false
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "bin", "dist":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if isGeneratedGoFile(path) {
			return nil
		}
		scanned = true
		if n := countPhysicalLines(path); n > maxHandwrittenGoLines {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				rel = path
			}
			t.Errorf("%s has %d lines; handwritten Go files must not exceed %d", filepath.ToSlash(rel), n, maxHandwrittenGoLines)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !scanned {
		t.Fatal("no Go source files scanned")
	}
}

// isGeneratedGoFile reports whether a Go file is generated using Go's standard
// generated-code convention and is therefore excluded from the size gate.
func isGeneratedGoFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() {
		// Read-only close is best-effort; the generated-code detection has
		// already read the file header.
		_ = f.Close()
	}()
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	return strings.Contains(string(buf[:n]), "Code generated ")
}

// countPhysicalLines returns the number of newline-delimited physical lines in
// a file. It intentionally counts lines, not logical statements, so the rule
// stays understandable and does not encourage compressing useful code.
func countPhysicalLines(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	return len(strings.Split(string(data), "\n"))
}
