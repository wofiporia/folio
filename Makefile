lint:
	golangci-lint run ./...

test:
	$(MAKE) lint
	go test -coverpkg=./... -coverprofile=test/coverage.out ./...
	go tool cover -func=test/coverage.out

.PHONY: lint test
