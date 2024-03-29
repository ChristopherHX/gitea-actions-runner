DIST := dist
EXECUTABLE := gitea-actions-runner
GOFMT ?= gofumpt -l
DIST := dist
DIST_DIRS := $(DIST)/binaries $(DIST)/release
GO ?= go
SHASUM ?= shasum -a 256
HAS_GO = $(shell hash $(GO) > /dev/null 2>&1 && echo "GO" || echo "NOGO" )
XGO_PACKAGE ?= src.techknowlogick.com/xgo@latest
XGO_VERSION := go-1.18.x
GXZ_PAGAGE ?= github.com/ulikunitz/xz/cmd/gxz@v0.5.10

LINUX_ARCHS ?= linux/amd64,linux/arm64
DARWIN_ARCHS ?= darwin-12/amd64,darwin-12/arm64
WINDOWS_ARCHS ?= windows/amd64
GOFILES := $(shell find . -type f -name "*.go" ! -name "generated.*")

ifneq ($(shell uname), Darwin)
	EXTLDFLAGS = -extldflags "-static" $(null)
else
	EXTLDFLAGS =
endif

ifeq ($(HAS_GO), GO)
	GOPATH ?= $(shell $(GO) env GOPATH)
	export PATH := $(GOPATH)/bin:$(PATH)

	CGO_EXTRA_CFLAGS := -DSQLITE_MAX_VARIABLE_NUMBER=32766
	CGO_CFLAGS ?= $(shell $(GO) env CGO_CFLAGS) $(CGO_EXTRA_CFLAGS)
endif

ifeq ($(OS), Windows_NT)
	GOFLAGS := -v -buildmode=exe
	EXECUTABLE ?= $(EXECUTABLE).exe
else ifeq ($(OS), Windows)
	GOFLAGS := -v -buildmode=exe
	EXECUTABLE ?= $(EXECUTABLE).exe
else
	GOFLAGS := -v
	EXECUTABLE ?= $(EXECUTABLE)
endif

STORED_VERSION_FILE := VERSION

ifneq ($(DRONE_TAG),)
	VERSION ?= $(subst v,,$(DRONE_TAG))
	RELEASE_VERSION ?= $(VERSION)
else
	ifneq ($(DRONE_BRANCH),)
		VERSION ?= $(subst release/v,,$(DRONE_BRANCH))
	else
		VERSION ?= master
	endif

	STORED_VERSION=$(shell cat $(STORED_VERSION_FILE) 2>/dev/null)
	ifneq ($(STORED_VERSION),)
		RELEASE_VERSION ?= $(STORED_VERSION)
	else
		RELEASE_VERSION ?= $(shell git describe --tags --always | sed 's/-/+/' | sed 's/^v//')
	endif
endif

TAGS ?=
LDFLAGS ?= -X 'gitea.com/gitea/act_runner/cmd.version=$(RELEASE_VERSION)'

all: build

fmt:
	@hash gofumpt > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) install -u mvdan.cc/gofumpt; \
	fi
	$(GOFMT) -w $(GOFILES)

vet:
	$(GO) vet ./...

.PHONY: fmt-check
fmt-check:
	@hash gofumpt > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) install -u mvdan.cc/gofumpt; \
	fi
	@diff=$$($(GOFMT) -d $(GOFILES)); \
	if [ -n "$$diff" ]; then \
		echo "Please run 'make fmt' and commit the result:"; \
		echo "$${diff}"; \
		exit 1; \
	fi;

test: fmt-check
	@$(GO) test -v -cover -coverprofile coverage.txt ./... && echo "\n==>\033[32m Ok\033[m\n" || exit 1

install: $(GOFILES)
	$(GO) install -v -tags '$(TAGS)' -ldflags '$(EXTLDFLAGS)-s -w $(LDFLAGS)'

build: $(EXECUTABLE)

$(EXECUTABLE): $(GOFILES)
	$(GO) build -v -tags '$(TAGS)' -ldflags '$(EXTLDFLAGS)-s -w $(LDFLAGS)' -o $@

.PHONY: deps-backend
deps-backend:
	$(GO) mod download
	$(GO) install $(GXZ_PAGAGE)
	$(GO) install $(XGO_PACKAGE)

.PHONY: release
release: release-windows release-linux release-darwin release-copy release-compress release-check

$(DIST_DIRS):
	mkdir -p $(DIST_DIRS)

.PHONY: release-windows
release-windows: | $(DIST_DIRS)
	CGO_CFLAGS="$(CGO_CFLAGS)" $(GO) run $(XGO_PACKAGE) -go $(XGO_VERSION) -buildmode exe -dest $(DIST)/binaries -tags 'netgo osusergo $(TAGS)' -ldflags '-linkmode external -extldflags "-static" $(LDFLAGS)' -targets '$(WINDOWS_ARCHS)' -out $(EXECUTABLE)-$(VERSION) .
ifeq ($(CI),true)
	cp -r /build/* $(DIST)/binaries/
endif

.PHONY: release-linux
release-linux: | $(DIST_DIRS)
	CGO_CFLAGS="$(CGO_CFLAGS)" $(GO) run $(XGO_PACKAGE) -go $(XGO_VERSION) -dest $(DIST)/binaries -tags 'netgo osusergo $(TAGS)' -ldflags '-linkmode external -extldflags "-static" $(LDFLAGS)' -targets '$(LINUX_ARCHS)' -out $(EXECUTABLE)-$(VERSION) .
ifeq ($(CI),true)
	cp -r /build/* $(DIST)/binaries/
endif

.PHONY: release-darwin
release-darwin: | $(DIST_DIRS)
	CGO_CFLAGS="$(CGO_CFLAGS)" $(GO) run $(XGO_PACKAGE) -go $(XGO_VERSION) -dest $(DIST)/binaries -tags 'netgo osusergo $(TAGS)' -ldflags '$(LDFLAGS)' -targets '$(DARWIN_ARCHS)' -out $(EXECUTABLE)-$(VERSION) .
ifeq ($(CI),true)
	cp -r /build/* $(DIST)/binaries/
endif

.PHONY: release-copy
release-copy: | $(DIST_DIRS)
	cd $(DIST); for file in `find . -type f -name "*"`; do cp $${file} ./release/; done;

.PHONY: release-check
release-check: | $(DIST_DIRS)
	cd $(DIST)/release/; for file in `find . -type f -name "*"`; do echo "checksumming $${file}" && $(SHASUM) `echo $${file} | sed 's/^..//'` > $${file}.sha256; done;

.PHONY: release-compress
release-compress: | $(DIST_DIRS)
	cd $(DIST)/release/; for file in `find . -type f -name "*"`; do echo "compressing $${file}" && $(GO) run $(GXZ_PAGAGE) -k -9 $${file}; done;

clean:
	$(GO) clean -x -i ./...
	rm -rf coverage.txt $(EXECUTABLE) $(DIST)

version:
	@echo $(VERSION)
