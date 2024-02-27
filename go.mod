module gitea.com/gitea/act_runner

go 1.18

require (
	code.gitea.io/actions-proto-go v0.3.1
	github.com/ChristopherHX/github-act-runner v0.4.2
	github.com/bufbuild/connect-go v1.3.1
	github.com/google/uuid v1.3.0
	github.com/joho/godotenv v1.5.1
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/mattn/go-isatty v0.0.18
	github.com/nektos/act v0.2.45
	github.com/rhysd/actionlint v1.6.24
	github.com/sirupsen/logrus v1.9.0
	github.com/spf13/cobra v1.7.0
	golang.org/x/sync v0.1.0
	google.golang.org/protobuf v1.28.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/Microsoft/go-winio v0.5.2 // indirect
	github.com/ProtonMail/go-crypto v0.0.0-20230217124315-7d5c6f04bbb8 // indirect
	github.com/acomagu/bufpipe v1.0.4 // indirect
	github.com/cloudflare/circl v1.1.0 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/fatih/color v1.15.0 // indirect
	github.com/go-git/gcfg v1.5.0 // indirect
	github.com/go-git/go-billy/v5 v5.4.1 // indirect
	github.com/go-git/go-git/v5 v5.6.2-0.20230411180853-ce62f3e9ff86 // indirect
	github.com/golang-jwt/jwt v3.2.2+incompatible // indirect
	github.com/imdario/mergo v0.3.15 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/julienschmidt/httprouter v1.3.0 // indirect
	github.com/kardianos/service v1.2.2 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-runewidth v0.0.14 // indirect
	github.com/pjbgf/sha1cd v0.3.0 // indirect
	github.com/rivo/uniseg v0.4.4 // indirect
	github.com/robfig/cron v1.2.0 // indirect
	github.com/sergi/go-diff v1.2.0 // indirect
	github.com/skeema/knownhosts v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/timshannon/bolthold v0.0.0-20210913165410-232392fc8a6a // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	go.etcd.io/bbolt v1.3.7 // indirect
	golang.org/x/crypto v0.6.0 // indirect
	golang.org/x/net v0.7.0 // indirect
	golang.org/x/sys v0.7.0 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
)

replace github.com/nektos/act => gitea.com/gitea/act v0.245.1

replace github.com/ChristopherHX/github-act-runner => gitea.com/ChristopherHX/github-act-runner v0.0.0-20230107151555-a42f45f85dda
