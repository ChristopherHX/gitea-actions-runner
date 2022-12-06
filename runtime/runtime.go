package runtime

import (
	"context"
	"strings"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"gitea.com/gitea/act_runner/client"
)

// Runner runs the pipeline.
type Runner struct {
	Machine       string
	ForgeInstance string
	Environ       map[string]string
	Client        client.Client
	Labels        []string
}

// Run runs the pipeline stage.
func (s *Runner) Run(ctx context.Context, task *runnerv1.Task) error {
	return NewTask(s.ForgeInstance, task.Id, s.Client, s.Environ, s.platformPicker).Run(ctx, task)
}

func (s *Runner) platformPicker(labels []string) string {
	// "ubuntu-18.04:docker://node:16-buster"

	platforms := make(map[string]string, len(labels))
	for _, l := range s.Labels {
		// "ubuntu-18.04:docker://node:16-buster"
		splits := strings.SplitN(l, ":", 2)
		// ["ubuntu-18.04", "docker://node:16-buster"]
		k, v := splits[0], splits[1]

		if prefix := "docker://"; !strings.HasPrefix(v, prefix) {
			continue
		} else {
			v = strings.TrimPrefix(v, prefix)
		}
		// ubuntu-18.04 => node:16-buster
		platforms[k] = v
	}

	for _, label := range labels {
		if v, ok := platforms[label]; ok {
			return v
		}
	}

	// TODO: support multiple labels
	// like:
	//   ["ubuntu-22.04"] => "ubuntu:22.04"
	//   ["with-gpu"] => "linux:with-gpu"
	//   ["ubuntu-22.04", "with-gpu"] => "ubuntu:22.04_with-gpu"

	// return default
	return "node:16-bullseye"
}
