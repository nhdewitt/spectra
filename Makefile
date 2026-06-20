# Spectra Makefile
# Usage:
#	make release			- Cross-compile agent binaries + checksums
#	make build-server		- Build frontend assets and server binary
#	make build-setup		- Build the setup binary
#	make setup				- First-time stand-up: install binaries + unit, run setup
#	make deploy-releases	- Copy agent release binaries/checksums to server
#	make deploy-server		- Deploy server binary and restart spectra-server
#	make deploy				- Deploy releases, build server, deploy server
#	make clean				- Remove release artifacts
#
# Local vs remote: targets that install or deploy run locally when DEPLOY_HOST
# is empty (sudo is scoped to privileged steps) and over SSH when it is set.

AGENT_SRC = ./cmd/agent
RELEASE_DIR = releases

DEPLOY_HOST ?=
DEPLOY_USER ?= root
DEPLOY_PATH ?= /opt/spectra

VERSION	?= $(shell git describe --tags 2>/dev/null || echo "0.0.0-$$(git rev-list --count HEAD 2>/dev/null || echo 0).$$(git rev-parse --short HEAD 2>/dev/null || echo unknown)")
COMMIT	?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE	?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# sha256sum on Linux, shasum -a 256 on macOS.
SHASUM := $(shell command -v sha256sum >/dev/null 2>&1 && echo sha256sum || echo "shasum -a 256")

# Base flags for all binaries. Agent builds add GoARM (set per-platform in the
# release loop); server/setup must not carry it.
BASE_LDFLAGS = -s -w \
	-X github.com/nhdewitt/spectra/internal/version.Version=$(VERSION) \
	-X github.com/nhdewitt/spectra/internal/version.Commit=$(COMMIT) \
	-X github.com/nhdewitt/spectra/internal/version.Date=$(DATE)

AGENT_LDFLAGS = $(BASE_LDFLAGS) \
	-X github.com/nhdewitt/spectra/internal/version.GoARM=$$arm

PLATFORMS = \
	linux/amd64/ \
	linux/arm64/ \
	linux/arm/6 \
	linux/arm/7 \
	freebsd/amd64/ \
	windows/amd64/

.PHONY: release build-server build-setup setup deploy-server deploy-releases deploy clean

build-server:
	@mkdir -p $(RELEASE_DIR)
	cd web && npm ci && npm run build
	go build -ldflags "$(BASE_LDFLAGS)" -trimpath -o $(RELEASE_DIR)/spectra-server ./cmd/server

build-setup:
	@mkdir -p $(RELEASE_DIR)
	go build -ldflags "$(BASE_LDFLAGS)" -trimpath -o $(RELEASE_DIR)/spectra-setup ./cmd/setup

# setup — first-time stand-up. Builds binaries, installs them + the systemd unit,
# then runs spectra-setup (which configures DB/admin/key and starts the service).
#
# Local (on the box):   make setup           (sudo scoped to privileged steps)
# Remote (workstation): make setup DEPLOY_HOST=<ip> [DEPLOY_USER=root] [DEPLOY_PATH=/opt/spectra]
setup: build-server build-setup
	@if [ -n "$(DEPLOY_HOST)" ]; then \
		echo "  Remote setup on $(DEPLOY_HOST)..."; \
		ssh $(DEPLOY_USER)@$(DEPLOY_HOST) 'mkdir -p $(DEPLOY_PATH)/releases /etc/spectra'; \
		scp $(RELEASE_DIR)/spectra-server $(RELEASE_DIR)/spectra-setup $(DEPLOY_USER)@$(DEPLOY_HOST):$(DEPLOY_PATH)/; \
		scp deploy/spectra-server.service $(DEPLOY_USER)@$(DEPLOY_HOST):/etc/systemd/system/spectra-server.service; \
		ssh $(DEPLOY_USER)@$(DEPLOY_HOST) 'chmod 755 $(DEPLOY_PATH)/spectra-server $(DEPLOY_PATH)/spectra-setup && systemctl daemon-reload'; \
		ssh -t $(DEPLOY_USER)@$(DEPLOY_HOST) '$(DEPLOY_PATH)/spectra-setup'; \
	else \
		set -e; \
		echo "  Local setup (sudo for privileged steps)..."; \
		sudo mkdir -p $(DEPLOY_PATH)/releases /etc/spectra; \
		sudo install -m 755 $(RELEASE_DIR)/spectra-server $(DEPLOY_PATH)/spectra-server; \
		sudo install -m 755 $(RELEASE_DIR)/spectra-setup $(DEPLOY_PATH)/spectra-setup; \
		sudo install -m 644 deploy/spectra-server.service /etc/systemd/system/spectra-server.service; \
		sudo systemctl daemon-reload; \
		sudo $(DEPLOY_PATH)/spectra-setup; \
	fi

# deploy-server — push a new server binary and restart, with rollback on a failed
# restart. Local or remote based on DEPLOY_HOST.
deploy-server:
	@test -f $(RELEASE_DIR)/spectra-server || { echo "No server binary. Run 'make build-server' first."; exit 1; }
	@if [ -n "$(DEPLOY_HOST)" ]; then \
		scp $(RELEASE_DIR)/spectra-server $(DEPLOY_USER)@$(DEPLOY_HOST):$(DEPLOY_PATH)/spectra-server.new; \
		ssh $(DEPLOY_USER)@$(DEPLOY_HOST) '\
			set -e; \
			if [ -f $(DEPLOY_PATH)/spectra-server ]; then \
				cp $(DEPLOY_PATH)/spectra-server $(DEPLOY_PATH)/spectra-server.bak; \
			fi; \
			install -m 755 $(DEPLOY_PATH)/spectra-server.new $(DEPLOY_PATH)/spectra-server; \
			rm -f $(DEPLOY_PATH)/spectra-server.new; \
			if ! systemctl restart spectra-server; then \
				echo "Restart failed; rolling back"; \
				if [ -f $(DEPLOY_PATH)/spectra-server.bak ]; then \
					mv $(DEPLOY_PATH)/spectra-server.bak $(DEPLOY_PATH)/spectra-server; \
					systemctl restart spectra-server; \
				fi; \
				exit 1; \
			fi; \
			sleep 2; \
			systemctl is-active spectra-server'; \
	else \
		set -e; \
		if [ -f $(DEPLOY_PATH)/spectra-server ]; then \
			sudo cp $(DEPLOY_PATH)/spectra-server $(DEPLOY_PATH)/spectra-server.bak; \
		fi; \
		sudo install -m 755 $(RELEASE_DIR)/spectra-server $(DEPLOY_PATH)/spectra-server; \
		if ! sudo systemctl restart spectra-server; then \
			echo "Restart failed; rolling back"; \
			if [ -f $(DEPLOY_PATH)/spectra-server.bak ]; then \
				sudo mv $(DEPLOY_PATH)/spectra-server.bak $(DEPLOY_PATH)/spectra-server; \
				sudo systemctl restart spectra-server; \
			fi; \
			exit 1; \
		fi; \
		sleep 2; \
		sudo systemctl is-active spectra-server; \
	fi
	@echo "  Server deployed and running."

release: clean
	@mkdir -p $(RELEASE_DIR)
	@rm -f $(RELEASE_DIR)/checksums.sha256
	@echo " VERSION $(VERSION) ($(COMMIT)) $(DATE)"
	@echo ""
	@for arch in amd64 arm64; do \
		name="spectra-agent-darwin-$$arch"; \
		echo "  BUILD	$$name (CGO if available)"; \
		if GOOS=darwin GOARCH=$$arch CGO_ENABLED=1 \
			go build -ldflags="$(AGENT_LDFLAGS)" -trimpath \
			-o $(RELEASE_DIR)/$$name $(AGENT_SRC) 2>/dev/null; then \
			echo "  OK		$$name (CGO)"; \
		else \
			echo "  FALL	$$name (CGO unavailable; static fallback)"; \
			GOOS=darwin GOARCH=$$arch CGO_ENABLED=0 \
				go build -ldflags="$(AGENT_LDFLAGS)" -trimpath \
				-o $(RELEASE_DIR)/$$name $(AGENT_SRC) || exit 1; \
		fi; \
		$(SHASUM) $(RELEASE_DIR)/$$name >> $(RELEASE_DIR)/checksums.sha256; \
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
			go build -ldflags="$(AGENT_LDFLAGS)" -trimpath \
			-o $(RELEASE_DIR)/$$name$$ext $(AGENT_SRC) || exit 1; \
		$(SHASUM) $(RELEASE_DIR)/$$name$$ext >> $(RELEASE_DIR)/checksums.sha256; \
	done
	@echo ""
	@echo "  Built $$(ls $(RELEASE_DIR)/spectra-agent-* 2>/dev/null | wc -l) binaries"
	@echo "  Checksums: $(RELEASE_DIR)/checksums.sha256"

deploy-releases:
	@test -f $(RELEASE_DIR)/checksums.sha256 || { echo "No checksums. Run 'make release' first."; exit 1; }
	@test "$$(ls $(RELEASE_DIR)/spectra-agent-* 2>/dev/null | wc -l)" -gt 0 || { echo "No agent binaries. Run 'make release' first."; exit 1; }
	@if [ -n "$(DEPLOY_HOST)" ]; then \
		scp $(RELEASE_DIR)/spectra-agent-* $(RELEASE_DIR)/checksums.sha256 $(DEPLOY_USER)@$(DEPLOY_HOST):$(DEPLOY_PATH)/releases/; \
	else \
		sudo cp $(RELEASE_DIR)/spectra-agent-* $(RELEASE_DIR)/checksums.sha256 $(DEPLOY_PATH)/releases/; \
	fi
	@echo "  Releases deployed."

deploy: release deploy-releases build-server deploy-server
	@echo "  Full deploy complete."

clean:
	rm -rf $(RELEASE_DIR)