# This Makefile is meant to be used by people that do not usually work
# with Go source code. If you know what GOPATH is then you probably
# don't need to bother with make.

.PHONY: install all test clean

GOBIN = ./build/bin

install:
	go get -v -d ./...

all: install
	mkdir -p build/bin
	go build -o build/bin/etc-stratum .

test: all
	go test -v ./...

clean:
	go clean -cache
