.DEFAULT_GOAL := help

# Determine this makefile's path.
# Be sure to place this BEFORE `include` directives, if any.
DEFAULT_BRANCH := main
THIS_FILE := $(lastword $(MAKEFILE_LIST))
PKG := github.com/sertvitas/ecs-agent-status
COMMIT := $(shell git rev-parse HEAD)
PKG_LIST := $(shell go list ${PKG}/... | grep -v /vendor/)
GO_FILES := $(shell find . -name '*.go' | grep -v /vendor/)
CDIR = $(shell pwd)
EXECUTABLES := ecs-agent-status
GOOS := linux
GOARCH := amd64

CURRENT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
DEFAULT_BRANCH := main

help: ## Show this help.
	@fgrep -h "##" $(MAKEFILE_LIST) | fgrep -v fgrep | sed -e 's/\\$$//' | sed -e 's/##//'

${EXECUTABLES}:
	@for o in $(GOOS); do \
	  for a in $(GOARCH); do \
        echo "$(COMMIT)/$${o}/$${a}" ; \
        mkdir -p build/$(COMMIT)/$${o}/$${a} ; \
        echo "COMMIT: $(COMMIT)" >> build/$(COMMIT)/$${o}/$${a}/version.txt ; \
        env GOOS=$${o} GOARCH=$${a} \
        go build  -v -o build/$(COMMIT)/$${o}/$${a}/$@ \
				-ldflags="-X github.com/sertvitas/ecs-agent-status/version.Version=${COMMIT}" ${PKG}/cmd/$@; \
	  done \
    done ; \

build: git-status ${EXECUTABLES}
	rm -rf build/current
	cp -R $(CDIR)/build/$(COMMIT) $(CDIR)/build/current

docker-build: build ## create docker image with commit tag
	( \
	   docker build --no-cache \
       	-t ecs-agent-status:$(COMMIT) \
       	-t ecs-agent-status:latest \
       	-f Dockerfile .; \
	)

release: docker-build ## upload the latest docker image to ECR
	( \
	   aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin 709310380790.dkr.ecr.us-east-1.amazonaws.com; \
	   docker tag ecs-agent-status:latest 709310380790.dkr.ecr.us-east-1.amazonaws.com/ecs-agent-status:latest; \
	   docker push 709310380790.dkr.ecr.us-east-1.amazonaws.com/ecs-agent-status:latest; \
	)

docker-run: ## run docker image
	( \
	   docker run -it --rm \
	   	-e INTERVAL='6' \
	   	ecs-agent-status \
	)

test:
	@go test -v ${PKG_LIST}
#	@go test -short ${PKG_LIST}

vet:
	@go vet ${PKG_LIST}

goimports: ## check imports
	go install golang.org/x/tools/cmd/goimports@latest
	goimports -w .

lint:  ##  run golint
	go install golang.org/x/lint/golint@latest
	@for file in ${GO_FILES} ;  do \
		golint $$file ; \
	done

fmt: ## run gofmt
	@go fmt ${PKG_LIST}

gocyclo: # run cyclomatic complexity check
	go install github.com/fzipp/gocyclo/cmd/gocyclo@latest
	gocyclo -over 25 .

static: goimports fmt vet lint gocyclo test

clean:
	-@rm ${OUT} ${OUT}-v*


git-status: ## require status is clean so we can use undo_edits to put things back
	@status=$$(git status --porcelain); \
	if [ ! -z "$${status}" ]; \
	then \
		echo "Error - working directory is dirty. Commit those changes!"; \
		exit 1; \
	fi

.PHONY: build release static upload vet lint fmt gocyclo goimports test