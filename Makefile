GO ?= go
SQLC ?= sqlc

.PHONY: generate fmt test race vet coverage check run dev

generate:
	$(SQLC) generate

fmt:
	GOCACHE=/tmp/syd-squash-go-cache $(GO) fmt ./...

test:
	GOCACHE=/tmp/syd-squash-go-cache $(GO) test ./...

coverage:
	GOCACHE=/tmp/syd-squash-go-cache $(GO) test -coverpkg=./internal/app,./internal/config,./internal/domain,./internal/sqlite,./internal/web -coverprofile=coverage.out ./internal/...
	GOCACHE=/tmp/syd-squash-go-cache $(GO) tool cover -func=coverage.out | awk '/^total:/ { gsub("%", "", $$3); if ($$3 + 0 < 80) { print "coverage " $$3 "% is below 80%"; exit 1 } }'

race:
	GOCACHE=/tmp/syd-squash-go-cache $(GO) test -race ./...

vet:
	GOCACHE=/tmp/syd-squash-go-cache $(GO) vet ./...

check: generate fmt test race vet coverage

run:
	GOCACHE=/tmp/syd-squash-go-cache APP_ENV=development $(GO) run ./cmd/web

dev:
	GOCACHE=/tmp/syd-squash-go-cache GO_BINARY=$(GO) $(GO) run ./cmd/dev
