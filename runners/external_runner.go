package runners

import (
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

func CreateExternalRunnerDirectory(parameters Parameters) (azure bool, prefix, ext, agentname, tmpdir string, err error) {
	azure = parameters.AzurePipelines
	prefix = "Runner"
	if azure {
		prefix = "Agent"
	}
	ext = ""
	if runtime.GOOS == "windows" {
		ext = ".exe" // adjust this based on the target OS
	}
	root, err := filepath.Abs(parameters.RunnerPath)
	if err != nil {
		return
	}
	absPath, _ := filepath.Abs(parameters.RunnerDirectory)
	os.MkdirAll(absPath, 0755)
	tmpdir, _ = os.MkdirTemp(absPath, "runner-*")
	agentname = path.Base(tmpdir)
	os.MkdirAll(filepath.Join(tmpdir, "bin"), 0755)
	bindir := filepath.Join(root, "bin")
	err = filepath.Walk(bindir, func(bfile string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		fname := strings.TrimPrefix(bfile, bindir+string(os.PathSeparator))
		destfile := filepath.Join(tmpdir, "bin", fname)
		if strings.HasPrefix(fname, prefix+".") && (strings.HasSuffix(fname, ".exe") || strings.HasSuffix(fname, ".dll") || !strings.Contains(fname[len(prefix)+1:], ".")) {
			copyFile(bfile, destfile)
		} else {
			if info.IsDir() {
				os.Symlink(bfile, destfile)
			} else {
				os.Symlink(bfile, destfile)
			}
		}
		return nil
	})
	if err != nil {
		return
	}
	os.MkdirAll(filepath.Join(root, "externals"), 0755)
	os.Symlink(filepath.Join(root, "externals"), filepath.Join(tmpdir, "externals"))
	os.Symlink(filepath.Join(root, "license.html"), filepath.Join(tmpdir, "license.html"))
	return
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	info, _ := sourceFile.Stat()
	defer sourceFile.Close()

	destFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	return destFile.Sync()
}

type Parameters struct {
	AzurePipelines  bool
	RunnerPath      string
	RunnerDirectory string
}
