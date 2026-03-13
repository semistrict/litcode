OUT_DIR := out
BIN_DIR := $(OUT_DIR)

.PHONY: check fix test bench build html-docs cover lint fmt

check:
	go run . check

fix:
	go run . fix

lint:
	golangci-lint run ./...

fmt:
	gofmt -w -s $$(find . -name '*.go')

test:
	go test ./...

bench:
	mkdir -p $(OUT_DIR)
	go test ./internal/checker/ -bench=. -benchmem -cpuprofile=$(OUT_DIR)/cpu.prof -memprofile=$(OUT_DIR)/mem.prof

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/litcode .

html-docs:
	go run . html-docs

cover:
	mkdir -p $(OUT_DIR)
	go test ./... -coverprofile=$(OUT_DIR)/cover.out
	go tool cover -func=$(OUT_DIR)/cover.out
