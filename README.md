# act runner

Act runner is a runner for Gitea based on [actions/runner](https://github.com/actions/runner) and the workflow yaml parser of [act](https://gitea.com/gitea/act).

## Prerequisites

- Install powershell 7 https://github.com/powershell/powershell
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

1. github-act-runner worker args for example pwsh,actions-runner-worker.ps1,actions-runner/bin/Runner.Worker
   actions-runner-worker.ps1 is a wrapper script to call the actions/runner via the platform specfic dotnet anonymous pipes
2. Gitea instance URL, like `http://192.168.8.8:3000/`. You should use your gitea instance ROOT_URL as the instance argument
 and you should not use `localhost` or `127.0.0.1` as instance IP;
3. Runner token, you can get it from `http://192.168.8.8:3000/admin/runners`;
4. Runner name, you can just leave it blank;
5. Runner labels, you can just leave it blank.

The process looks like:

```text
INFO Registering runner, arch=amd64, os=darwin, version=0.1.5.
INFO Enter the github-act-runner worker args for example pwsh,actions-runner-worker.ps1,actions-runner/bin/Runner.Worker:
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

If the registry succeed, it will run immediately. Next time, you could run the runner directly.

### Run

```bash
./act_runner daemon
```