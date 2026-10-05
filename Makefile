BINARY_NAME := tf-unlock
PKG := .
LDFLAGS := -s -w

.PHONY: all build test clean dist

all: test build

build:
	go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) $(PKG)

test:
	go test -v ./...

dist:
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY_NAME)-windows-amd64.exe $(PKG)
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY_NAME)-windows-arm64.exe $(PKG)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY_NAME)-linux-amd64 $(PKG)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY_NAME)-linux-arm64 $(PKG)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY_NAME)-darwin-amd64 $(PKG)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY_NAME)-darwin-arm64 $(PKG)

clean:
	rm -rf dist $(BINARY_NAME) $(BINARY_NAME).exe
