
DEFAULT: all

BLDDIR = build
APPS = main

$(BLDDIR)/main: $(wildcard ./bin/main/*.go ./*/*.go)

all: $(APPS)

$(BLDDIR)/%:
	@mkdir -p $(dir $@)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -installsuffix cgo -a -tags netgo -ldflags '-w -extldflags "-static"' -o $@ ./bin/$*

$(APPS): %: $(BLDDIR)/%