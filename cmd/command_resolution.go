package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func resolveExecutablePath(program string) (string, error) {
	if program == "" {
		return "", fmt.Errorf("실행할 프로그램 이름이 비어 있습니다")
	}

	if hasPathSeparator(program) {
		return ensureFileExists(program)
	}

	return searchExecutableInDirs(program, commandSearchDirs())
}

func commandSearchDirs() []string {
	searchDirs := make([]string, 0)

	if bundledDir, err := bundledBinDir(); err == nil && directoryExists(bundledDir) {
		searchDirs = append(searchDirs, bundledDir)
	}

	searchDirs = append(searchDirs, filepath.SplitList(os.Getenv(pathEnvKey()))...)
	return uniquePathEntries(searchDirs)
}

func commandEnvironment() []string {
	bundledDir, err := bundledBinDir()
	if err != nil || !directoryExists(bundledDir) {
		return os.Environ()
	}

	return prependPathEntry(os.Environ(), bundledDir)
}

func bundledBinDir() (string, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("실행 파일 경로를 확인할 수 없습니다: %w", err)
	}

	return filepath.Join(filepath.Dir(executablePath), "bin"), nil
}

func searchExecutableInDirs(program string, searchDirs []string) (string, error) {
	for _, dir := range uniquePathEntries(searchDirs) {
		for _, candidateName := range executableNameCandidates(program, os.Getenv("PATHEXT")) {
			candidatePath := filepath.Join(dir, candidateName)
			resolvedPath, err := ensureFileExists(candidatePath)
			if err == nil {
				return resolvedPath, nil
			}
		}
	}

	return "", fmt.Errorf("%q 실행 파일을 찾을 수 없습니다", program)
}

func executableNameCandidates(program string, pathExt string) []string {
	candidates := []string{program}

	if runtime.GOOS != "windows" || filepath.Ext(program) != "" {
		return candidates
	}

	if strings.TrimSpace(pathExt) == "" {
		pathExt = ".COM;.EXE;.BAT;.CMD"
	}

	seen := map[string]struct{}{
		strings.ToLower(program): {},
	}

	for _, extension := range strings.Split(pathExt, ";") {
		extension = strings.TrimSpace(extension)
		if extension == "" {
			continue
		}

		if !strings.HasPrefix(extension, ".") {
			extension = "." + extension
		}

		candidate := program + extension
		key := strings.ToLower(candidate)
		if _, exists := seen[key]; exists {
			continue
		}

		seen[key] = struct{}{}
		candidates = append(candidates, candidate)
	}

	return candidates
}

func prependPathEntry(environment []string, entry string) []string {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return environment
	}

	environmentCopy := append([]string(nil), environment...)

	pathIndex := -1
	pathKey := pathEnvKey()
	pathValue := ""

	for index, environmentEntry := range environmentCopy {
		key, value, found := strings.Cut(environmentEntry, "=")
		if !found {
			continue
		}

		if !equalEnvKey(key, pathEnvKey()) {
			continue
		}

		pathIndex = index
		pathKey = key
		pathValue = value
		break
	}

	updatedPath := strings.Join(uniquePathEntries(append([]string{entry}, filepath.SplitList(pathValue)...)), string(os.PathListSeparator))
	if pathIndex == -1 {
		return append(environmentCopy, pathKey+"="+updatedPath)
	}

	environmentCopy[pathIndex] = pathKey + "=" + updatedPath
	return environmentCopy
}

func uniquePathEntries(entries []string) []string {
	uniqueEntries := make([]string, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))

	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		normalizedEntry := normalizePathForComparison(entry)
		if _, exists := seen[normalizedEntry]; exists {
			continue
		}

		seen[normalizedEntry] = struct{}{}
		uniqueEntries = append(uniqueEntries, entry)
	}

	return uniqueEntries
}

func normalizePathForComparison(path string) string {
	normalizedPath := filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(normalizedPath)
	}

	return normalizedPath
}

func equalEnvKey(left string, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}

	return left == right
}

func hasPathSeparator(path string) bool {
	return strings.ContainsRune(path, '/') || strings.ContainsRune(path, '\\')
}

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return info.IsDir()
}

func pathEnvKey() string {
	return "PATH"
}
