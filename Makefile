.PHONY: test run-api run-worker fmt

test:
	go test ./...

run-api:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')
