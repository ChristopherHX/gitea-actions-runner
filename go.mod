module gitea.com/gitea/act_runner

go 1.18

require (
	code.gitea.io/actions-proto-go v0.2.1
	github.com/ChristopherHX/github-act-runner v0.4.2
	github.com/bufbuild/connect-go v1.3.1
	github.com/google/uuid v1.3.0
	github.com/joho/godotenv v1.4.0
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/mattn/go-isatty v0.0.16
	github.com/nektos/act v0.2.22
	github.com/rhysd/actionlint v1.6.22
	github.com/sirupsen/logrus v1.9.0
	github.com/spf13/cobra v1.6.1
	golang.org/x/sync v0.0.0-20220819030929-7fc1605a5dde
	google.golang.org/protobuf v1.28.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/Microsoft/go-winio v0.5.2 // indirect
	github.com/ProtonMail/go-crypto v0.0.0-20220404123522-616f957b79ad // indirect
	github.com/acomagu/bufpipe v1.0.3 // indirect
	github.com/emirpasic/gods v1.12.0 // indirect
	github.com/fatih/color v1.13.0 // indirect
	github.com/go-git/gcfg v1.5.0 // indirect
	github.com/go-git/go-billy/v5 v5.3.1 // indirect
	github.com/go-git/go-git/v5 v5.4.2 // indirect
	github.com/go-ini/ini v1.67.0 // indirect
	github.com/golang-jwt/jwt v3.2.2+incompatible // indirect
	github.com/imdario/mergo v0.3.13 // indirect
	github.com/inconshreveable/mousetrap v1.0.1 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/rivo/uniseg v0.3.4 // indirect
	github.com/robfig/cron v1.2.0 // indirect
	github.com/sergi/go-diff v1.2.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/xanzy/ssh-agent v0.3.1 // indirect
	golang.org/x/crypto v0.0.0-20220331220935-ae2d96664a29 // indirect
	golang.org/x/net v0.0.0-20220906165146-f3363e06e74c // indirect
	golang.org/x/sys v0.0.0-20220818161305-2296e01440c6 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
)

replace github.com/nektos/act => gitea.com/gitea/act v0.234.0

replace github.com/ChristopherHX/github-act-runner => gitea.com/ChristopherHX/github-act-runner v0.0.0-20230107151555-a42f45f85dda
