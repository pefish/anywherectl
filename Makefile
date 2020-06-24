
DEFAULT: build-cur

GORUN = go-build-tool


build-cur:
	$(GORUN) build-cur

build-cur-pack:
	$(GORUN) build-cur-pack

build-all:
	$(GORUN) build-all

build-all-pack:
	$(GORUN) build-all-pack
