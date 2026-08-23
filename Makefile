.PHONY: test audit
.IGNORE: audit

all: audit test

test:
	@go test ./...

audit:
	@which golangci-lint >/dev/null || (echo "Cannot run linters. Have you installed golangci-lint?" && false)
	@golangci-lint run
