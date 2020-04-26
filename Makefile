
DEFAULT: build-cur

GORUN = env GO111MODULE=on go run


build-cur:
	$(GORUN) internal/build-bin/build_bin.go build-cur

build-cur-pack:
	$(GORUN) internal/build-bin/build_bin.go build-cur-pack

build-all:
	$(GORUN) internal/build-bin/build_bin.go build-all

build-all-pack:
	$(GORUN) internal/build-bin/build_bin.go build-all-pack
