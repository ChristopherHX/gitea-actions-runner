package util

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	log "github.com/sirupsen/logrus"
)

// https://github.com/actions/runner/releases
const ActionsRunnerVersion string = "2.326.0"
// https://github.com/christopherHX/runner.server/releases
const RunnerServerRunnerVersion string = "3.13.4"

//go:embed actions-runner-worker.py
var pythonWorkerScript string

//go:embed actions-runner-worker.ps1
var pwshWorkerScript string

func SetupRunner(runnerType int32, runnerVersion string) []string {
	d := DownloadRunner
	if runnerType == 2 {
		d = DownloadRunnerServer
	}
	if runnerVersion == "" {
		if runnerType == 1 {
			runnerVersion = ActionsRunnerVersion
		} else {
			runnerVersion = RunnerServerRunnerVersion
		}
	}
	wd, _ := os.Getwd()
	p := filepath.Join(wd, "actions-runner-"+runnerVersion)
	if fi, err := os.Stat(p); err == nil && fi.IsDir() {
		log.Infof("Runner %s already exists, skip downloading.", runnerVersion)
	} else {
		if err := d(context.Background(), log.StandardLogger(), runtime.GOOS+"/"+runtime.GOARCH, p, runnerVersion); err != nil {
			log.Infoln("Something went wrong: %s" + err.Error())
			return nil
		}
	}

	return SetupWorker(wd, p, runnerType, runnerVersion)
}

func SetupWorker(wd string, p string, runnerType int32, runnerVersion string) []string {
	flags := []string{"--runner-dir=" + p, "--runner-type=" + fmt.Sprint(runnerType), "--runner-version=" + runnerVersion, "--allow-clone"}
	pwshScript := filepath.Join(p, "actions-runner-worker.ps1")
	_ = os.WriteFile(pwshScript, []byte(pwshWorkerScript), 0755)
	pythonScript := filepath.Join(p, "actions-runner-worker.py")
	_ = os.WriteFile(pythonScript, []byte(pythonWorkerScript), 0755)

	var pythonPath string
	var err error
	ext := ""
	if runtime.GOOS != "windows" {
		pythonPath, err = exec.LookPath("python3")
		if err != nil {
			pythonPath, _ = exec.LookPath("python")
		}
	} else {
		ext = ".exe"
	}
	if pythonPath == "" {
		pwshPath, err := exec.LookPath("pwsh")
		if err != nil {
			pwshVersion := "7.4.7"
			pwshPath = filepath.Join(wd, "pwsh-"+pwshVersion)
			if fi, err := os.Stat(pwshPath); err == nil && fi.IsDir() {
				log.Infof("pwsh %s already exists, skip downloading.", pwshVersion)
			} else {
				log.Infoln("pwsh not found, downloading pwsh...")
				err = DownloadPwsh(context.Background(), log.StandardLogger(), runtime.GOOS+"/"+runtime.GOARCH, pwshPath, pwshVersion)
				if err != nil {
					log.Infoln("Something went wrong: %s" + err.Error())
					return nil
				}
			}
			pwshPath = filepath.Join(pwshPath, "pwsh"+ext)
		} else {
			log.Infoln("pwsh found, using pwsh...")
		}
		return append(flags, pwshPath, pwshScript, filepath.Join(p, "bin", "Runner.Worker"+ext))
	} else {
		log.Infoln("python found, using python...")
		return append(flags, pythonPath, pythonScript, filepath.Join(p, "bin", "Runner.Worker"+ext))
	}
}
