VERSION = v0.5.0

VERSION_STAMP="main.version=v$(VERSION)"
VERSION_HASH="main.gitHash=$$(git rev-parse HEAD)"
BUILD_FLAGS="-X ${VERSION_STAMP} -X ${VERSION_HASH}"

SRC := $(shell find . -type f -name '*.go')

bin/invoicer: $(SRC)
	go build -o $@ -ldflags ${BUILD_FLAGS}

bin/invoicer-race: $(SRC)
	go build -race -o $@ -ldflags ${BUILD_FLAGS}


bin/darwin/invoicer: 		$(SRC)
	env GOOS=darwin GOARCH=amd64 		go build -o $@  -ldflags ${BUILD_FLAGS}

bin/linux-armv6/invoicer: 	$(SRC)
	env GOOS=linux GOARCH=arm GOARM=6 	go build -o $@  -ldflags ${BUILD_FLAGS}

bin/linux-armv7/invoicer: 	$(SRC)
	env GOOS=linux GOARCH=arm GOARM=7 	go build -o $@  -ldflags ${BUILD_FLAGS}

bin/linux-amd64/invoicer: 	$(SRC)
	env GOOS=linux GOARCH=amd64 		go build -o $@  -ldflags ${BUILD_FLAGS}

bin/freebsd-amd64/invoicer: $(SRC)
	env GOOS=freebsd GOARCH=amd64 		go build -o $@  -ldflags ${BUILD_FLAGS}

bin/openbsd-amd64/invoicer: $(SRC)
	env GOOS=openbsd GOARCH=amd64 		go build -o $@  -ldflags ${BUILD_FLAGS}


bin/invoicer-$(VERSION)-darwin.tgz: 		bin/darwin/invoicer
	tar -cvzf $@ $<
bin/invoicer-$(VERSION)-linux-armv6.tgz: 	bin/linux-armv6/invoicer
	tar -cvzf $@ $<
bin/invoicer-$(VERSION)-linux-armv7.tgz: 	bin/linux-armv7/invoicer
	tar -cvzf $@ $<
bin/invoicer-$(VERSION)-linux-amd64.tgz: 	bin/linux-amd64/invoicer
	tar -cvzf $@ $<
bin/invoicer-$(VERSION)-freebsd-amd64.tgz: 	bin/freebsd-amd64/invoicer
	tar -cvzf $@ $<
bin/invoicer-$(VERSION)-openbsd-amd64.tgz: 	bin/openbsd-amd64/invoicer
	tar -cvzf $@ $<


static/index.html:
	mkdir -p static
	curl -s https://api.github.com/repos/lncm/donations/releases/latest \
		| grep "browser_download_url.*html" \
		| cut -d '"' -f 4 \
		| wget -O $@ -qi -

run: $(SRC)
	go run main.go -config ./invoicer.conf

tag:
	git tag -sa $(VERSION) -m "$(VERSION)"

ci: bin/invoicer-$(VERSION)-darwin.tgz \
	bin/invoicer-$(VERSION)-linux-armv6.tgz \
	bin/invoicer-$(VERSION)-linux-armv7.tgz \
	bin/invoicer-$(VERSION)-linux-amd64.tgz \
	bin/invoicer-$(VERSION)-freebsd-amd64.tgz \
	bin/invoicer-$(VERSION)-openbsd-amd64.tgz

all: tag ci

REMOTE_USER ?= ln
REMOTE_HOST ?= pi-hdd

REMOTE_DIR_BINARY ?= /home/ln/bin/
deploy-invoicer: bin/linux-armv7/invoicer
	rsync $< "${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_DIR_BINARY}"

REMOTE_DIR_STATIC ?= /home/ln/static/
deploy-static: static/index.html
	rsync $< "${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_DIR_STATIC}"

deploy: deploy-invoicer deploy-static

clean:
	rm -rf bin/*

.PHONY: run tag all deploy deploy-invoicer deploy-static clean ci static/index.html

