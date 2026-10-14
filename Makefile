GOBIN ?= $(shell go env GOPATH)/bin
GOLANGCI_LINT := $(if $(shell command -v golangci-lint 2>/dev/null),golangci-lint,$(GOBIN)/golangci-lint)

# 库包（examples 不计入覆盖率，只做编译烟雾测试）
LIB_PKGS := ./errs/... ./paging/... ./logx/... ./api/... ./storage/...
COVERAGE_THRESHOLD := 99

.PHONY: build vet lint test cover mock integration tidy

build:
	go build ./...

vet:
	go vet ./...

lint:
	$(GOLANGCI_LINT) run

test:
	go test -race -coverprofile=coverage.out $(LIB_PKGS)

cover: test
	@go tool cover -func=coverage.out | tail -1
	@total=$$(go tool cover -func=coverage.out | tail -1 | awk '{print substr($$3, 1, length($$3)-1)}'); \
	awk -v t=$$total -v th=$(COVERAGE_THRESHOLD) 'BEGIN { exit (t+0 < th) ? 1 : 0 }' || \
	{ echo "FAIL: coverage $${total}% < $(COVERAGE_THRESHOLD)%"; exit 1; }

mock:
	go generate ./...

integration:
	go test -tags=integration $(LIB_PKGS)

tidy:
	go mod tidy
