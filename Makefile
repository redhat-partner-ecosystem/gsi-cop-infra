
.PHONY: all
all: test

.PHONY: test
test:
	cd internal && go test
	
.PHONY: test_coverage
test_coverage:
	go test `go list ./...` -coverprofile=coverage.txt -covermode=atomic

.PHONY: test_build
test_build:
	cd cmd/static-site && go build main.go && rm main

