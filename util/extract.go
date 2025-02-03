package util

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nektos/act/pkg/filecollector"
)

type Logger interface {
	Infof(format string, args ...interface{})
}

func ExtractTar(in io.Reader, dst string) error {
	os.RemoveAll(dst)
	tr := tar.NewReader(in)
	cp := &filecollector.CopyCollector{
		DstDir: dst,
	}
	for {
		ti, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return err
		}
		pc := strings.SplitN(ti.Name, "/", 2)
		if ti.FileInfo().IsDir() || len(pc) < 2 {
			continue
		}
		_ = cp.WriteFile(pc[1], ti.FileInfo(), ti.Linkname, tr)
	}
}

func ExtractZip(in io.ReaderAt, size int64, dst string) error {
	os.RemoveAll(dst)
	tr, err := zip.NewReader(in, size)
	if err != nil {
		return err
	}
	cp := &filecollector.CopyCollector{
		DstDir: dst,
	}
	for _, ti := range tr.File {
		if ti.FileInfo().IsDir() {
			continue
		}
		fs, _ := ti.Open()
		defer fs.Close()
		_ = cp.WriteFile(ti.Name, ti.FileInfo(), "", fs)
	}
	return nil
}

func ExtractTarGz(reader io.Reader, dir string) error {
	gzr, err := gzip.NewReader(reader)
	if err != nil {
		return err
	}
	defer gzr.Close()
	return ExtractTar(gzr, dir)
}

func DownloadTool(ctx context.Context, logger Logger, url, dest string) error {
	token := ""
	httpClient := http.DefaultClient
	randBytes := make([]byte, 16)
	_, _ = rand.Read(randBytes)
	cachedTar := filepath.Join(dest, "..", hex.EncodeToString(randBytes)+".tmp")
	defer os.Remove(cachedTar)
	var tarstream io.Reader
	if logger != nil {
		logger.Infof("Downloading %s to %s", url, dest)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Add("Authorization", "token "+token)
	}
	req.Header.Add("User-Agent", "github-act-runner/1.0.0")
	req.Header.Add("Accept", "*/*")
	rsp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != 200 {
		buf := &bytes.Buffer{}
		_, _ = io.Copy(buf, rsp.Body)
		return fmt.Errorf("failed to download action from %v response %v", url, buf.String())
	}
	fo, err := os.Create(cachedTar)
	if err != nil {
		return err
	}
	defer fo.Close()
	ch := make(chan error)
	go func() {
		for {
			select {
			case <-ch:
				return
			case <-time.After(time.Second * 10):
				off, _ := fo.Seek(0, 1)
				percent := off * 100 / rsp.ContentLength
				logger.Infof("Downloading... %d%%\n", percent)
			}
		}
	}()
	l, err := io.Copy(fo, rsp.Body)
	close(ch)
	if err != nil {
		return err
	}
	if rsp.ContentLength >= 0 && l != rsp.ContentLength {
		return fmt.Errorf("failed to download tar expected %v, but copied %v", rsp.ContentLength, l)
	}
	tarstream = fo
	_, _ = fo.Seek(0, 0)
	if strings.HasSuffix(url, ".tar.gz") {
		if err := ExtractTarGz(tarstream, dest); err != nil {
			return err
		}
	} else {
		st, _ := fo.Stat()
		if err := ExtractZip(fo, st.Size(), dest); err != nil {
			return err
		}
	}
	return nil
}

// Official GitHub Actions Runner
func DownloadRunner(ctx context.Context, logger Logger, plat string, dest string, version string) error {
	AURL := func(arch, ext string) string {
		return fmt.Sprintf("https://github.com/actions/runner/releases/download/v%s/actions-runner-%s-%s.%s", version, arch, version, ext)
	}
	download := map[string]string{
		"windows/386":   AURL("win-x86", "zip"),
		"windows/amd64": AURL("win-x64", "zip"),
		"windows/arm64": AURL("win-arm64", "zip"),
		"linux/amd64":   AURL("linux-x64", "tar.gz"),
		"linux/arm":     AURL("linux-arm", "tar.gz"),
		"linux/arm64":   AURL("linux-arm64", "tar.gz"),
		"darwin/amd64":  AURL("osx-x64", "tar.gz"),
		"darwin/arm64":  AURL("osx-arm64", "tar.gz"),
	}
	// Includes the bin folder in the archive
	return DownloadTool(ctx, logger, download[plat], dest)
}

// Includes windows container support
func DownloadRunnerServer(ctx context.Context, logger Logger, plat string, dest string, version string) error {
	AURL := func(arch, ext string) string {
		return fmt.Sprintf("https://github.com/ChristopherHX/runner.server/releases/download/v%s/runner.server-%s.%s", version, arch, ext)
	}
	download := map[string]string{
		"windows/386":   AURL("win-x86", "zip"),
		"windows/amd64": AURL("win-x64", "zip"),
		"windows/arm64": AURL("win-arm64", "zip"),
		"linux/amd64":   AURL("linux-x64", "tar.gz"),
		"linux/arm":     AURL("linux-arm", "tar.gz"),
		"linux/arm64":   AURL("linux-arm64", "tar.gz"),
		"darwin/amd64":  AURL("osx-x64", "tar.gz"),
		"darwin/arm64":  AURL("osx-arm64", "tar.gz"),
	}
	downloadURL, ok := download[plat]
	if !ok {
		return fmt.Errorf("unsupported platform %s", plat)
	}
	// Contains only the bin folder content
	return DownloadTool(ctx, logger, downloadURL, filepath.Join(dest, "bin"))
}

// Includes windows container support
func DownloadPwsh(ctx context.Context, logger Logger, plat string, dest string, version string) error {
	AURL := func(arch, ext string) string {
		return fmt.Sprintf("https://github.com/PowerShell/PowerShell/releases/download/v%s/powershell-7.4.7-%s.%s", version, arch, ext)
	}
	download := map[string]string{
		"windows/386":   AURL("win-x86", "zip"),
		"windows/amd64": AURL("win-x64", "zip"),
		"windows/arm64": AURL("win-arm64", "zip"),
		"linux/amd64":   AURL("linux-x64", "tar.gz"),
		"linux/arm":     AURL("linux-arm", "tar.gz"),
		"linux/arm64":   AURL("linux-arm64", "tar.gz"),
		"darwin/amd64":  AURL("osx-x64", "tar.gz"),
		"darwin/arm64":  AURL("osx-arm64", "tar.gz"),
	}
	downloadURL, ok := download[plat]
	if !ok {
		return fmt.Errorf("unsupported platform %s", plat)
	}
	// Contains only the bin folder content
	return DownloadTool(ctx, logger, downloadURL, filepath.Join(dest, "bin"))
}
