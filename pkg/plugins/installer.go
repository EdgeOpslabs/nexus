package plugins

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func Install(source, destDir string) (string, error) {
	if source == "" {
		return "", fmt.Errorf("source is required")
	}
	if destDir == "" {
		return "", fmt.Errorf("plugins directory is required")
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", err
	}

	localPath, cleanup, err := resolveSource(source)
	if err != nil {
		return "", err
	}
	if cleanup != nil {
		defer cleanup()
	}

	info, err := os.Stat(localPath)
	if err != nil {
		return "", err
	}

	if info.IsDir() {
		target := filepath.Join(destDir, filepath.Base(localPath))
		if err := copyDir(localPath, target); err != nil {
			return "", err
		}
		return target, nil
	}

	ext := strings.ToLower(filepath.Ext(localPath))
	switch ext {
	case ".zip":
		target := filepath.Join(destDir, trimExt(filepath.Base(localPath)))
		if err := unzip(localPath, target); err != nil {
			return "", err
		}
		return target, nil
	case ".gz":
		if strings.HasSuffix(localPath, ".tar.gz") || strings.HasSuffix(localPath, ".tgz") {
			target := filepath.Join(destDir, trimExt(trimExt(filepath.Base(localPath))))
			if err := untarGz(localPath, target); err != nil {
				return "", err
			}
			return target, nil
		}
	}

	target := filepath.Join(destDir, filepath.Base(localPath))
	if err := copyFile(localPath, target); err != nil {
		return "", err
	}
	return target, nil
}

func resolveSource(source string) (string, func(), error) {
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return download(source)
	}
	return source, nil, nil
}

func download(url string) (string, func(), error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", nil, fmt.Errorf("download failed: %s", resp.Status)
	}

	tmpFile, err := os.CreateTemp("", "nexus-plugin-*")
	if err != nil {
		return "", nil, err
	}
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return "", nil, err
	}

	cleanup := func() { _ = os.Remove(tmpFile.Name()) }
	return tmpFile.Name(), cleanup, nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func unzip(src, dst string) error {
	reader, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		target := filepath.Join(dst, file.Name)
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		in, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			in.Close()
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			in.Close()
			out.Close()
			return err
		}
		in.Close()
		out.Close()
	}
	return nil
}

func untarGz(src, dst string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dst, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			out, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
	return nil
}

func trimExt(name string) string {
	return strings.TrimSuffix(name, filepath.Ext(name))
}
