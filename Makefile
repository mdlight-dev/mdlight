.PHONY: build build-darwin build-linux build-windows clean dev test bench

GOOS  ?= linux
GOARCH?= amd64
VERSION?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

BINARY := build/bin/mdlight

# ── Development ────────────────────────────────────────────────────────────────

dev:
	wails dev

test:
	go test ./... -count=1 -race

vet:
	go vet ./...

lint:
	go vet ./...
	cd frontend && npm run build 2>&1 | grep -E "error|warn" || true

# ── Building (native) ──────────────────────────────────────────────────────────

build: frontend sync-themes
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=1 \
		go build -trimpath -tags "desktop,production" \
		-ldflags="-s -w -X main.version=$(VERSION)" \
		-o $(BINARY) .
	@ls -lh $(BINARY) | awk '{print "  → " $$5}'

build-linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 $(MAKE) build
	GOOS=linux GOARCH=arm64 CGO_ENABLED=1 $(MAKE) build

build-darwin:
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 $(MAKE) build
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 $(MAKE) build

build-windows:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=1 $(MAKE) build

# ── Frontend ────────────────────────────────────────────────────────────────────

frontend:
	cd frontend && npm install && npm run build

# sync-themes copies theme CSS from the frontend source tree into the Go embed
# target, keeping them in sync. The frontend copy is authoritative.
sync-themes:
	cp frontend/src/themes/builtin/*.css internal/theme/builtin/

# ── Benchmarks ──────────────────────────────────────────────────────────────────

bench: build
	@echo "=== Binary size ==="
	@ls -lh $(BINARY) | awk '{print "  " $$5}'
	@echo "=== Startup bench (cold, --bench) ==="
	@./$(BINARY) testdata/sample.md --bench 2>&1 | grep "\[bench\]"

# ── Cleanup ─────────────────────────────────────────────────────────────────────

clean:
	rm -rf build/bin dist frontend/dist
