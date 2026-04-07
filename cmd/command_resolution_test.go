package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSearchExecutableInDirsPrefersEarlierDirectory(t *testing.T) {
	firstDir := t.TempDir()
	secondDir := t.TempDir()

	executableName := platformExecutableName("demo")
	firstPath := filepath.Join(firstDir, executableName)
	secondPath := filepath.Join(secondDir, executableName)

	if err := os.WriteFile(firstPath, []byte("first"), 0o644); err != nil {
		t.Fatalf("failed to create first executable: %v", err)
	}

	if err := os.WriteFile(secondPath, []byte("second"), 0o644); err != nil {
		t.Fatalf("failed to create second executable: %v", err)
	}

	resolvedPath, err := searchExecutableInDirs("demo", []string{firstDir, secondDir})
	if err != nil {
		t.Fatalf("expected executable to resolve: %v", err)
	}

	if normalizePathForComparison(resolvedPath) != normalizePathForComparison(firstPath) {
		t.Fatalf("expected first directory to win, got %q", resolvedPath)
	}
}

func TestExecutableNameCandidatesUsesPathExtOnWindows(t *testing.T) {
	candidates := executableNameCandidates("toolbox", ".EXE;.CMD")

	if runtime.GOOS != "windows" {
		if len(candidates) != 1 || candidates[0] != "toolbox" {
			t.Fatalf("expected non-Windows candidate list to contain only the original name, got %v", candidates)
		}

		return
	}

	expected := []string{"toolbox", "toolbox.EXE", "toolbox.CMD"}
	if strings.Join(candidates, "|") != strings.Join(expected, "|") {
		t.Fatalf("unexpected candidates: %v", candidates)
	}
}

func TestPrependPathEntryAddsBundledBinFirstOnce(t *testing.T) {
	bundledDir := filepath.Join(t.TempDir(), "bin")
	environment := []string{
		"Path=" + filepath.Join(t.TempDir(), "system"),
		"TERM=xterm-256color",
	}

	updatedEnvironment := prependPathEntry(environment, bundledDir)

	var pathEntry string
	for _, environmentEntry := range updatedEnvironment {
		if strings.HasPrefix(strings.ToLower(environmentEntry), "path=") {
			pathEntry = environmentEntry
			break
		}
	}

	if pathEntry == "" {
		t.Fatal("expected PATH entry to exist")
	}

	pathValue := strings.SplitN(pathEntry, "=", 2)[1]
	pathParts := filepath.SplitList(pathValue)
	if len(pathParts) < 2 {
		t.Fatalf("expected bundled directory and original PATH entries, got %v", pathParts)
	}

	if normalizePathForComparison(pathParts[0]) != normalizePathForComparison(bundledDir) {
		t.Fatalf("expected bundled directory first, got %v", pathParts)
	}

	updatedEnvironment = prependPathEntry(updatedEnvironment, bundledDir)

	for _, environmentEntry := range updatedEnvironment {
		if !strings.HasPrefix(strings.ToLower(environmentEntry), "path=") {
			continue
		}

		pathParts = filepath.SplitList(strings.SplitN(environmentEntry, "=", 2)[1])
		break
	}

	count := 0
	for _, pathPart := range pathParts {
		if normalizePathForComparison(pathPart) == normalizePathForComparison(bundledDir) {
			count++
		}
	}

	if count != 1 {
		t.Fatalf("expected bundled directory to appear once, got %v", pathParts)
	}
}

func platformExecutableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}

	return name
}
