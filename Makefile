# Spectra Makefile
# Usage:
#	make release			- Cross-compile agent binaries + checksums
#	make build-server		- Build frontend assets and server binary
#	make deploy-releases	- Copy agent release binaries/checksums to server
#	make deploy-server		- Deploy server binary and restart spectra-server
#	make deploy				- Deploy releases, build server, deploy server
#	make clean				- Remove release artifacts

AGENT_SRC = ./cmd/agent
RELEASE_DIR = releases

DEPLOY_HOST ?=
DEPLOY_USER ?= root
DEPLOY_PATH ?= /opt/spectra

VERSION	?= $(shell git describe --tags 2>/dev/null || echo "0.0.0-$(shell git rev-list --count HEAD).$(shell git rev-parse --short HEAD)")
COMMIT	?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE	?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS = -s -w \
	-X github.com/nhdewitt/spectra/internal/version.Version=$(VERSION) \
	-X github.com/nhdewitt/spectra/internal/version.Commit=$(COMMIT) \
	-X github.com/nhdewitt/spectra/internal/version.Date=$(DATE) \
	-X github.com/nhdewitt/spectra/internal/version.GoARM=$$arm

PLATFORMS = \
	linux/amd64/ \
	linux/arm64/ \
	linux/arm/6 \
	linux/arm/7 \
	freebsd/amd64/ \
	windows/amd64/

DARWIN_CGO_PLATFORMS = darwin/amd64 darwin/arm64

.PHONY: release build-server deploy-server deploy-releases deploy clean

build-server:
	@mkdir -p $(RELEASE_DIR)
	cd web && npm run build
	go build -ldflags "$(LDFLAGS)" -trimpath -o $(RELEASE_DIR)/spectra-server ./cmd/server

build-setup:
	@mkdir -p $(RELEASE_DIR)
	go build -ldflags "$(LDFLAGS)" -trimpath -o $(RELEASE_DIR)/spectra-setup ./cmd/setup

deploy-server:
	@test -f $(RELEASE_DIR)/spectra-server || { echo "No server binary. Run 'make build-server first.'"; exit 1; }
	@test -n "$(DEPLOY_HOST)" || { echo "Usage: make deploy-server DEPLOY_HOST=<ip> [DEPLOY_USER=root] [DEPLOY_PATH=/opt/spectra]"; exit 1; }
	scp $(RELEASE_DIR)/spectra-server $(DEPLOY_USER)@$(DEPLOY_HOST):$(DEPLOY_PATH)/spectra-server.new
	ssh $(DEPLOY_USER)@$(DEPLOY_HOST) '\
		mv $(DEPLOY_PATH)/spectra-server $(DEPLOY_PATH)/spectra-server.bak && \
		mv $(DEPLOY_PATH)/spectra-server.new $(DEPLOY_PATH)/spectra-server && \
		chmod 755 $(DEPLOY_PATH)/spectra-server && \
		systemctl restart spectra-server && \
		sleep 2 && \
		systemctl is-active spectra-server'
	@echo "  Server deployed and running."

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

deploy-releases:
	@test -d $(RELEASE_DIR) || { echo "No releases directory. Run 'make release' first."; exit 1; }
	@test -n "$(DEPLOY_HOST)" || { echo "Usage: make deploy-releases DEPLOY_HOST=<ip> [DEPLOY_USER=root] [DEPLOY_PATH=/opt/spectra]"; exit 1; }
	scp $(RELEASE_DIR)/spectra-agent-* $(RELEASE_DIR)/checksums.sha256 $(DEPLOY_USER)@$(DEPLOY_HOST):$(DEPLOY_PATH)/releases/
	@echo "  Releases deployed."

deploy: release deploy-releases build-server deploy-server
	@echo "  Full deploy complete."

clean:
	rm -rf $(RELEASE_DIR)