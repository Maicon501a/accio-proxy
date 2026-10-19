//go:build windows

package sgbridge

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

//go:embed security-guard.zip
var embeddedBundleZip []byte

// extractEmbeddedBundle unzips the embedded security-guard bundle into dstDir.
func extractEmbeddedBundle(dstDir string) error {
	if len(embeddedBundleZip) == 0 {
		return fmt.Errorf("embedded bundle is empty")
	}

	r, err := zip.NewReader(bytes.NewReader(embeddedBundleZip), int64(len(embeddedBundleZip)))
	if err != nil {
		return fmt.Errorf("open embedded zip: %w", err)
	}

	cleanDst := filepath.Clean(dstDir)

	for _, f := range r.File {
		path := filepath.Join(dstDir, f.Name)
		cleanPath := filepath.Clean(path)
		if !strings.HasPrefix(cleanPath, cleanDst) {
			continue
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0o755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func hasEmbeddedBundle() bool {
	return len(embeddedBundleZip) > 0
}
