module github.com/ChristopherHX/gitea-actions-runner

go 1.23.0

toolchain go1.23.5

require (
	code.gitea.io/actions-proto-go v0.4.1
	connectrpc.com/connect v1.18.1
	github.com/ChristopherHX/github-act-runner v0.10.0
	github.com/avast/retry-go/v4 v4.6.1
	github.com/google/uuid v1.6.0
	github.com/joho/godotenv v1.5.1
	github.com/kardianos/service v1.2.2
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/mattn/go-isatty v0.0.20
	github.com/nektos/act v0.2.76
	github.com/rhysd/actionlint v1.7.7
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/cobra v1.9.1
	github.com/stretchr/testify v1.10.0
	golang.org/x/net v0.39.0
	golang.org/x/sync v0.13.0
	google.golang.org/protobuf v1.36.6
	gopkg.in/yaml.v3 v3.0.1
)

require (
	dario.cat/mergo v1.0.1 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/ProtonMail/go-crypto v1.2.0 // indirect
	github.com/bmatcuk/doublestar/v4 v4.8.1 // indirect
	github.com/cloudflare/circl v1.6.1 // indirect
	github.com/cyphar/filepath-securejoin v0.4.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.6.2 // indirect
	github.com/go-git/go-git/v5 v5.16.0 // indirect
	github.com/golang-jwt/jwt v3.2.2+incompatible // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/julienschmidt/httprouter v1.3.0 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/mattn/go-shellwords v1.0.12 // indirect
	github.com/pjbgf/sha1cd v0.3.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/sergi/go-diff v1.3.2-0.20230802210424-5b0b94c5c0d3 // indirect
	github.com/skeema/knownhosts v1.3.1 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/timshannon/bolthold v0.0.0-20240314194003-30aac6950928 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	go.etcd.io/bbolt v1.4.0 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
)

replace github.com/nektos/act => gitea.com/gitea/act v0.261.3

replace github.com/ChristopherHX/github-act-runner => gitea.com/ChristopherHX/github-act-runner v0.0.0-20250101191334-47a23853e4fa
