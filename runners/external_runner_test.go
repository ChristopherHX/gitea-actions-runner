package runners

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCloneExternalRunner(t *testing.T) {
	aure, prefix, ext, agentname, tmpdir, err := CreateExternalRunnerDirectory(Parameters{
		RunnerPath:      "/Users/christopher/Documents/ActionsAndPipelines/gitea-actions-runner/actions-runner-3.12.1",
		RunnerDirectory: "runners",
	})
	defer os.RemoveAll(tmpdir)
	assert.NoError(t, err)
	assert.Equal(t, false, aure)
	assert.Equal(t, "Runner", prefix)
	assert.Equal(t, "", ext)
	assert.NotEmpty(t, agentname)
	assert.Equal(t, "/Users/christopher/Documents/ActionsAndPipelines/gitea-actions-runner/runners/runners", path.Dir(tmpdir))
}
