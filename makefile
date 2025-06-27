# generated-from:0f1cfb3f9faa0c83355794c5720cb80c30b77f4fcb2887d31d2887bd169db413 DO NOT REMOVE, DO UPDATE

PLATFORM=$(shell uname -s | tr '[:upper:]' '[:lower:]')
PWD := $(shell pwd)

ifndef VERSION
	VERSION := $(shell git describe --tags --abbrev=0)
endif

COMMIT_HASH :=$(shell git rev-parse --short HEAD)
DEV_VERSION := dev-${COMMIT_HASH}

USERID := $(shell id -u $$USER)
GROUPID:= $(shell id -g $$USER)

export GOPRIVATE=github.com/moov-io

all: install build

.PHONY: install
install:
	go mod tidy

build:
	go build -ldflags "-X github.com/moov-io/rail-msg-sql.Version=${VERSION}" -o bin/rail-msg-sql github.com/moov-io/rail-msg-sql/cmd/rail-msg-sql

.PHONY: setup
setup:
	docker compose up -d --force-recreate --remove-orphans

.PHONY: check
check:
ifeq ($(OS),Windows_NT)
	@echo "Skipping checks on Windows, currently unsupported."
else
	@wget -O lint-project.sh https://raw.githubusercontent.com/moov-io/infra/master/go/lint-project.sh
	@chmod +x ./lint-project.sh
	COVER_THRESHOLD=45.0 ./lint-project.sh
endif

.PHONY: teardown
teardown:
	-docker compose down --remove-orphans

docker:
	docker build --pull --build-arg VERSION=${VERSION} -t moov/rail-msg-sql:${VERSION} -f Dockerfile .

docker-push:
	docker push moov/rail-msg-sql:${VERSION}

.PHONY: dev-docker
dev-docker:
	docker build --pull --build-arg VERSION=${DEV_VERSION} -t moov/rail-msg-sql:${DEV_VERSION} -f Dockerfile .

.PHONY: dev-push
dev-push:
	docker push moov/rail-msg-sql:${DEV_VERSION}

# Extra utilities not needed for building

run: build
	./bin/rail-msg-sql

docker-run:
	docker run -v ${PWD}/data:/data -v ${PWD}/configs:/configs --env APP_CONFIG="/configs/config.yml" -it --rm moov/rail-msg-sql:${VERSION}

test:
	go test -cover github.com/moov-io/rail-msg-sql/...

.PHONY: clean
clean:
ifeq ($(OS),Windows_NT)
	@echo "Skipping cleanup on Windows, currently unsupported."
else
	@rm -rf cover.out coverage.txt misspell* staticcheck*
	@rm -rf ./bin/
endif

# For open source projects

dist: clean build
ifeq ($(OS),Windows_NT)
	CGO_ENABLED=1 GOOS=windows go build -o bin/rail-msg-sql.exe cmd/rail-msg-sql/*
else
	CGO_ENABLED=1 GOOS=$(PLATFORM) go build -o bin/rail-msg-sql-$(PLATFORM)-amd64 cmd/rail-msg-sql/*
endif
