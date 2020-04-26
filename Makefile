
DEFAULT: all

GORUN = env GO111MODULE=on go run


all:
	$(GORUN) internal/build-bin/build_bin.go install

build-pack:
	$(GORUN) internal/build-bin/build_bin.go install-pack
