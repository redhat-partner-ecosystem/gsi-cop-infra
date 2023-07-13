
.PHONY: all
all: test

.PHONY: test
test: test_build test_code test_coverage

.PHONY: test_code
test_code:
	cd internal && go test
	go test
	
.PHONY: test_coverage
test_coverage:
	go test `go list ./...` -coverprofile=coverage.txt -covermode=atomic

.PHONY: test_build
test_build:
	cd cmd/httpd && go build main.go && rm main

