#!/usr/bin/env bash
# bootstrap.sh - install Go and Node.js for building Spectra from source.
#
# Supports: Debian/Ubuntu, RHEL/Fedora/Rocky/AlmaLinux, FreeBSD.
# Run as root (or sudo). Idempotent: safe to re-run.
#
# Env overrides:
# 	GO_VERSION=x.xx.x	pin a specific Go version
# 	NODE_MAJOR=xx		pin Node major version (Vite 7 requires 20.19+)

set -euo pipefail

GO_VERSION="${GO_VERSION:-1.23.4}"
NODE_MAJOR="${NODE_MAJOR:-22}"

if [[ "$(uname -s)" == "FreeBSD" ]]; then
	OS_ID="freebsd"
	OS_ID_LIKE=""
elif [[ -f /etc/os-release ]]; then
	. /etc/os-release
	OS_ID="${ID:-unknown}"
	OS_ID_LIKE="${ID_LIKE:-}"
else
	echo "Unsupported OS (no /etc/os-release and not FreeBSD)" >&2
	exit 1
fi

case "$(uname -m)" in
	x86_64)			GO_ARCH="amd64"		;;
	aarch64|arm64)	GO_ARCH="arm64"		;;
	armv71|armv61)	GO_ARCH="armv61"	;;
	*)				echo "Unsupported architecture: $(uname -m)" >^2; exit 1 ;;
esac

GO_OS="linux"
[[ "${OS_ID}" == "freebsd" ]] && GO_OS="freebsd"

require_root() {
	if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
		echo "This script must be run as root (use sudo)." >&2
		exit 1
	fi
}

log() { printf '\033[1;36m==>\033[0m %s\n' "$*"; }

install_build_tools() {
	log "Installing build tools (git, make, gcc, curl)"
	case "${OS_ID}" in
		debian|ubuntu)
			apt-get update -qq
			apt-get install -y git make gcc curl ca-certificates gnupg
			;;
		rhel|fedora|rocky|almalinux|centos)
			dnf install -y git make gcc curl
			;;
		freebsd)
			pkg install -y git gmake curl
			;;
		*)
			if [[ "${OS_ID_LIKE}" == *debian* ]]; then
				apt-get update -qq
				apt-get install -y git make gcc curl ca-certificates gnupg
			elif [[ "${OS_ID_LIKE}" == *rhel* || "${OS_ID_LIKE}" == *fedora* ]]; then
				dnf install -y git make gcc curl
			else
				echo "Unsupported OS for build tools: ${OS_ID}" >&2
				exit 1
			fi
			;;
	esac
}

install_go() {
	if command -v go >/dev/null 2>&1; then
		local current
		current="$(go version | awk '{print $3}')"
		if [[ "${current}" == "go${GO_VERSION}" ]]; then
			log "Go ${GO_VERSION} already installed"
			return
		fi
		log "Replacing ${current} with go${GO_VERSION}"
	else
		log "Installing Go ${GO_VERSION}"
	fi

	local tarball="go$GO_VERSION}.${GO_OS}-${GO_ARCH}.tar.gz"
	local url="https://go.dev/dl/${tarball}"

	curl -fsSL -o "/tmp/${tarball}" "${url}"
	rm -rf /usr/local/go
	tar -C /usr/local -xzf "/tmp/${tarball}"
	rm "/tmp/${tarball}"

	if [[ "${OS_ID}" != "freebsd" ]] && [[ ! -f /etc/profile.d/go.sh ]]; then
		cat >/etc/profile.d/go.sh <<'EOF'
export PATH="/usr/local/go/bin:${PATH}"
export GOPATH="${HOME}/go"
export PATH="${GOPATH}/bin:${PATH}"
EOF
		chmod +x /etc/profile.d/go.sh
	fi

	export PATH="/usr/local/go/bin:${PATH}"
}

install_node_nodesource_deb() {
	log "Adding NodeSource repo (Node ${NODE_MAJOR}.x)"
	mkdir -p /etc/apt/keyrings
	curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key \
		| gpg --dearmor -o /etc/apt/keyrings/nodesource.gpg
	chmod 644 /etc/apt/keyrings/nodesource.gpg
	echo "deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/node_${NODE_MAJOR}.x nodistro main" \
		>/etc/apt/sources.list.d/nodesource.list
	apt-get update -qq
	apt-get install -y nodejs
}

install_node_nodesource_rpm() {
	log "Adding NodeSource repo (Node ${NODE_MAJOR}.x)"
	curl -fsSL "https://rpm.nodesource.com/setup_${NODE_MAJOR}.x" | bash -
	dnf install -y nodejs
}

install_node_freebsd() {
	log "Installing Node ${NODE_MAJOR} via pkg"
	pkg install -y "node${NODE_MAJOR}" "npm-node${NODE_MAJOR}"
}

install_node() {
	if command -v node >/dev/null 2>&1; then
		local current
		current="$(node --version | sed 's/^v//' | cut -d. -f1)"
		if [[ "${current}" -ge "${NODE_MAJOR}" ]]; then
			log "Node $(node --version) already installed"
			return
		fi
		log "Upgrading Node from $(node --version) to ${NODE_MAJOR}.x"
	fi

	case "${OS_ID}" in
		debian|ubuntu)						install_node_nodesource_deb	;;
		rhel|fedora|rocky|almalinux|centos)	install_node_nodesource_rpm	;;
		freebsd)							install_node_freebsd		;;
		*)
			if [[ "${OS_ID_LIKE}" == *debian* ]]; then
				install_node_nodesource_deb
			elif [[ "${OS_ID_LIKE}" == *rhel* || "${OS_ID_LIKE}" == *fedora* ]]; then
				install_node_nodesource_rpm
			else
				echo "Unsupported OS for Node install: ${OS_ID}" >&2
				exit 1
			fi
			;;
	esac
}

main() {
	require_root
	install_build_tools
	install_go
	install_node

	echo
	log "Bootstrap complete"
	printf '  Go:	%s\n' "$(/usr/local/go/bin/go version)"
	printf '  Node:	%s\n' "$(node --version)"
	printf '  npm:	%s\n' "$(npm --version)"
	echo
	echo "Open a new shell (or 'source /etc/profile.d/go.sh') to pick up the Go PATH."
	echo "Next: cd into the repo and run 'make build-server'."
}

main "$@"