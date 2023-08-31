# actions runner

Actions runner is a runner for Gitea based on [actions/runner](https://github.com/actions/runner) and the workflow yaml parser of [act](https://gitea.com/gitea/act).

- This runner doesn't download actions via the git http protocol, it downloads them via tar.gz and zip archives.
- This runner doesn't support absolute actions in composite actions `https://github.com/actions/checkout@v3` will only work in workflow steps.
- This runner doesn't support go actions, however you can create a wrapper composite action as a workaround
- This runner does support service container
- This runner does support https://github.com/actions/runner-container-hooks
- This runner uses the official https://github.com/actions/runner runner to run your steps
- This runner is a protocol proxy between Gitea Actions and GitHub Actions

## Known Issues

- actions/runner's live logs are skipping lines, but it is not possible to upload the full log without breaking live logs after the steps are finished
- ~~job outputs cannot be sent back https://gitea.com/gitea/actions-proto-def/issues/4~~ fixed
- Not possible to update display name of steps from runner
  Pre and Post steps are part of setup and complete job, steps not in the job are ignored by the server

## Prerequisites

- Install powershell 7 https://github.com/powershell/powershell (actions-runner-worker.ps1)
  - For linux and macOS you can also use python3 instead (actions-runner-worker.py)
- Download and extract actions/runner https://github.com/actions/runner/releases
- You have to create simple .runner file in the root folder of the actions/runner with the following Content
  ```
  {"workFolder": "_work"}
  ```

## Quickstart

### Build

```bash
make build
```

### Register

```bash
./act_runner register
```

And you will be asked to input:

1. worker args for example `python3,actions-runner-worker.py,actions-runner/bin/Runner.Worker`, `pwsh,actions-runner-worker.ps1,actions-runner/bin/Runner.Worker`
   `actions-runner-worker`(`.ps1`/`.py`) are wrapper scripts to call the actions/runner via the platform specfic dotnet anonymous pipes

   `actions-runner-worker.py` doesn't work on windows

   On windows you might need to unblock the `actions-runner-worker.ps1` script via pwsh `Unblock-File actions-runner-worker.ps1` and `Runner.Worker` needs the `.exe` suffix.
   For example on windows use the following worker args `pwsh,actions-runner-worker.ps1,actions-runner/bin/Runner.Worker.exe`
2. Gitea instance URL, like `http://192.168.8.8:3000/`. You should use your gitea instance ROOT_URL as the instance argument
 and you should not use `localhost` or `127.0.0.1` as instance IP;
3. Runner token, you can get it from `http://192.168.8.8:3000/admin/runners`;
4. Runner name, you can just leave it blank;
5. Runner labels, you can just leave it blank.

The process looks like:

```text
INFO Registering runner, arch=amd64, os=darwin, version=0.1.5.
INFO Enter the worker args for example pwsh,actions-runner-worker.ps1,actions-runner/bin/Runner.Worker:
pwsh,actions-runner-worker.ps1,actions-runner/bin/Runner.Worker
INFO Enter the Gitea instance URL (for example, https://gitea.com/):
http://192.168.8.8:3000/
INFO Enter the runner token:
fe884e8027dc292970d4e0303fe82b14xxxxxxxx
INFO Enter the runner name (if set empty, use hostname:Test.local ):

INFO Enter the runner labels, leave blank to use the default labels (comma-separated, for example, self-hosted,ubuntu-latest):

INFO Registering runner, name=Test.local, instance=http://192.168.8.8:3000/, labels=[self-hosted ubuntu-latest].
DEBU Successfully pinged the Gitea instance server
INFO Runner registered successfully.
```

You can also register with command line arguments.

```bash
./act_runner register --instance http://192.168.8.8:3000 --token <my_runner_token> --worker pwsh,actions-runner-worker.ps1,actions-runner/bin/Runner.Worker --no-interactive
```

```bash
./act_runner register --instance http://192.168.8.8:3000 --token <my_runner_token> --worker python3,actions-runner-worker.py,actions-runner/bin/Runner.Worker --no-interactive
```

If the registry succeed, you could run the runner directly.

### Run

```bash
./act_runner daemon
```