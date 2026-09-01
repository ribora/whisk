.PHONY: test audit golden
.IGNORE: audit

all: audit test

test:
	@go test ./...

# Rewrites the goldens, so it is not part of all: run it when the change to the
# generator is the intended one, then read the diff.
golden:
	@go test ./cmd/whisk -run TestSuccess -update

audit:
	@which golangci-lint >/dev/null || (echo "Cannot run linters. Have you installed golangci-lint?" && false)
	@golangci-lint run
