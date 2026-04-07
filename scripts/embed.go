package scripts

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

//go:embed powershell/*.ps1
var powerShellScripts embed.FS

var (
	extractOnce  sync.Once
	extractedDir string
	extractErr   error
)

func Extract() (string, error) {
	extractOnce.Do(func() {
		extractedDir, extractErr = os.MkdirTemp("", "toolbox-scripts-*")
		if extractErr != nil {
			return
		}

		extractErr = fs.WalkDir(powerShellScripts, ".", func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if entry.IsDir() {
				return nil
			}

			content, err := powerShellScripts.ReadFile(path)
			if err != nil {
				return err
			}

			targetPath := filepath.Join(extractedDir, "scripts", filepath.FromSlash(path))
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}

			return os.WriteFile(targetPath, content, 0o644)
		})

		if extractErr != nil {
			_ = os.RemoveAll(extractedDir)
			extractedDir = ""
		}
	})

	return extractedDir, extractErr
}

func Cleanup() error {
	if extractedDir == "" {
		return nil
	}

	return os.RemoveAll(extractedDir)
}
