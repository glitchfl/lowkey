VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: test build windows doctor-e2e e2e clean

test:
	go vet ./...
	go test ./...

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/ ./cmd/...

windows:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/lowkey.exe ./cmd/lowkey
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/fakeriot.exe ./cmd/fakeriot

doctor-e2e: build
	scripts/doctor-e2e.sh bin/lowkey bin/fakeriot

e2e: build
	LOWKEY_EXE=$(CURDIR)/bin/lowkey go test ./tests/e2e -count=1 -v

clean:
	rm -rf bin
