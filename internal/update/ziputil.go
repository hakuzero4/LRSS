package update

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func extractNamedFromZip(zipPath, wantExt string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	outDir := filepath.Join(os.TempDir(), "lrss-unzip-bin")
	_ = os.MkdirAll(outDir, 0o755)

	wantExt = strings.ToLower(wantExt)
	var picked *zip.File
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		base := filepath.Base(f.Name)
		if strings.HasPrefix(strings.ToLower(base), "lrss") &&
			(wantExt == "" || strings.HasSuffix(strings.ToLower(base), wantExt)) {
			picked = f
			break
		}
	}
	if picked == nil {
		// any file with extension
		for _, f := range r.File {
			if f.FileInfo().IsDir() {
				continue
			}
			if wantExt == "" || strings.HasSuffix(strings.ToLower(f.Name), wantExt) {
				picked = f
				break
			}
		}
	}
	if picked == nil {
		return "", fmt.Errorf("file_not_in_zip")
	}
	dest := filepath.Join(outDir, filepath.Base(picked.Name))
	rc, err := picked.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	w, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return "", err
	}
	_, err = io.Copy(w, rc)
	cerr := w.Close()
	if err != nil {
		return "", err
	}
	return dest, cerr
}

func unzipAll(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		name := filepath.Clean(f.Name)
		if strings.Contains(name, "..") {
			continue
		}
		target := filepath.Join(destDir, name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		w, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}
		_, copyErr := io.Copy(w, rc)
		rc.Close()
		closeErr := w.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func findAppBundle(root string) (string, error) {
	var found string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".app") {
			found = path
			return io.EOF // stop walk
		}
		return nil
	})
	if found == "" {
		return "", fmt.Errorf("app_bundle_not_found")
	}
	return found, nil
}
