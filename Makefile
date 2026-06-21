BINARY  := favycon
EXT     := $(if $(filter windows,$(shell go env GOOS)),.exe,)
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X main.version=$(VERSION)

DIST    := dist

PLATFORMS := \
	windows-amd64 \
	windows-arm64 \
	darwin-amd64 \
	darwin-arm64 \
	linux-amd64 \
	linux-arm64

.PHONY: build clean dist all

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY)$(EXT) .

clean:
	rm -f $(BINARY) $(BINARY).exe
	rm -rf $(DIST)

dist: clean
	@mkdir -p $(DIST)
	@$(foreach P,$(PLATFORMS),\
		$(eval GOOS := $(word 1,$(subst -, ,$P)))\
		$(eval GOARCH := $(word 2,$(subst -, ,$P)))\
		$(eval EXT := $(if $(filter windows,$(GOOS)),.exe,))\
		echo "Building $(GOOS)/$(GOARCH)..." &&\
		GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" \
			-o $(DIST)/$(BINARY)-$(GOOS)-$(GOARCH)$(EXT) . &&\
	) echo "Built all binaries in $(DIST)/"

all: dist
