# act runner

Act runner is a runner for Forges supports Github Actions protocol.

## Quickstart

### build

```shell
make build
```

### register

```shell
./act_runner register
```

And you will be asked to input:

1. Gitea instance URL, like `http://localhost:3000/`;
2. Runner token, you can get it from `http://localhost:3000/admin/runners`;
3. Runner name, you can just leave it blank;
4. Runner labels, you can just leave it blank.

The process looks like:
```text
INFO Registering runner, arch=amd64, os=darwin, version=0.1.5.
WARN Runner in user-mode.
INFO Enter the Gitea instance URL (for example, https://gitea.com/):
http://localhost:3000/
INFO Enter the runner token:
fe884e8027dc292970d4e0303fe82b14xxxxxxxx
INFO Enter the runner name (if set empty, use hostname:Test.local ):

INFO Enter the runner labels, leave blank to use the default labels (comma-separated, for example, ubuntu-20.04:docker://node:16-bullseye,ubuntu-18.04:docker://node:16-buster):

INFO Registering runner, name=Test.local, instance=http://localhost:3000/, labels=[ubuntu-latest:docker://node:16-bullseye ubuntu-22.04:docker://node:16-bullseye ubuntu-20.04:docker://node:16-bullseye ubuntu-18.04:docker://node:16-buster].
DEBU Successfully pinged the Gitea instance server
INFO Runner registered successfully.
```

### run

```shell
./act_runner daemon
```