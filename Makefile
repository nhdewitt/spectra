# Spectra Makefile
# Usage:
#	make release		- Cross-compile agent binaries + checksums
#	make clean			- Remove release artifacts

AGENT_SRC = ./cmd/agent
RELEASE_DIR = releases

VERSION	?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT	?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE	?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS = -s -w \
	-X github.com/nhdewitt/spectra/internal/version.Version=$(VERSION) \
	-X github.com/nhdewitt/spectra/internal/version.Commit=$(COMMIT) \
	-X github.com/nhdewitt/spectra/internal/version.Date=$(DATE)

PLATFORMS = \
	linux/amd64/ \
	linux/arm64/ \
	linux/arm/6 \
	linux/arm/7 \
	freebsd/amd64/ \
	windows/amd64/

DARWIN_CGO_PLATFORMS = darwin/amd64 darwin/arm64

.PHONY: release clean

build-server:
	go build -ldflags "$(LDFLAGS)" -o bin/spectra-server ./cmd/server

release: clean
	@mkdir -p $(RELEASE_DIR)
	@rm -f $(RELEASE_DIR)/checksums.sha256
	@echo " VERSION $(VERSION) ($(COMMIT)) $(DATE)"
	@echo ""
	@for arch in amd64 arm64; do \
		name="spectra-agent-darwin-$$arch"; \
		echo "  BUILD	$$name (trying CGO)"; \
		if GOOS=darwin GOARCH=$$arch CGO_ENABLED=1 \
			go build -ldflags="$(LDFLAGS)" -trimpath \
			-o $(RELEASE_DIR)/$$name $(AGENT_SRC) 2>/dev/null; then \
			echo "  OK		$$name (CGO)"; \
		else \
			echo "  FALL	$$name (no-CGO)"; \
			GOOS=darwin GOARCH=$$arch CGO_ENABLED=0 \
				go build -ldflags="$(LDFLAGS)" -trimpath \
				-o $(RELEASE_DIR)/$$name $(AGENT_SRC) || exit 1; \
		fi; \
		sha256sum $(RELEASE_DIR)/$$name >> $(RELEASE_DIR)/checksums.sha256; \
	done
	@for platform in $(PLATFORMS); do \
		os=$$(echo $$platform | cut -d/ -f1); \
		arch=$$(echo $$platform | cut -d/ -f2); \
		arm=$$(echo $$platform | cut -d/ -f3); \
		ext=""; \
		if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		name="spectra-agent-$$os-$$arch"; \
		if [ -n "$$arm" ]; then name="spectra-agent-$$os-armv$$arm"; fi; \
		echo "  BUILD	$$name$$ext"; \
		GOOS=$$os GOARCH=$$arch GOARM=$$arm \
			go build -ldflags="$(LDFLAGS)" -trimpath \
			-o $(RELEASE_DIR)/$$name$$ext $(AGENT_SRC) || exit 1; \
		sha256sum $(RELEASE_DIR)/$$name$$ext >> $(RELEASE_DIR)/checksums.sha256; \
	done
	@echo ""
	@echo "  Built $$(ls $(RELEASE_DIR)/spectra-agent-* 2>/dev/null | wc -l) binaries"
	@echo "  Checksums: $(RELEASE_DIR)/checksums.sha256"

clean:
	rm -rf $(RELEASE_DIR)