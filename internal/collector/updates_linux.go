//go:build linux

package collector

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// apkUpgradeRe matches "(1/12) Upgrading musl (1.2.4-r1 -> 1.2.4-r2)"
var apkUpgradeRe = regexp.MustCompile(`\(\d+/\d+\) Upgrading (\S+) \(\S+ -> (\S+)\)`)

// updateChecker defines how to query a package manager for pending updates.
type updateChecker struct {
	name  string
	exe   string
	args  []string
	parse parseLine
}

// parseLine returns a PendingUpdate from a line, or ok=false to skip it.
type parseLine func(string) (protocol.PendingUpdate, bool)

func scanUpdates(r io.Reader, fn parseLine) ([]protocol.PendingUpdate, error) {
	var updates []protocol.PendingUpdate
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if u, ok := fn(scanner.Text()); ok {
			updates = append(updates, u)
		}
	}
	return updates, scanner.Err()
}

var updateCheckers = []updateChecker{
	{
		name:  "apt",
		exe:   "apt",
		args:  []string{"list", "--upgradable"},
		parse: parseAptLine,
	},
	{
		name:  "apk",
		exe:   "apk",
		args:  []string{"upgrade", "--simulate"},
		parse: parseApkLine,
	},
	{
		name:  "pacman",
		exe:   "checkupdates",
		args:  nil,
		parse: parsePacmanLine,
	},
}

// CollectUpdates checks for pending system updates using the relevant
// package manager. Returns a single UpdateMetric.
func CollectUpdates(ctx context.Context) ([]protocol.Metric, error) {
	// check yum first - it requires two commands
	if _, err := exec.LookPath("yum"); err == nil {
		secPkgs, _ := collectYumUpdates(ctx)
		if secPkgs == nil {
			secPkgs = make(map[string]bool)
		}
		uMetric, err := buildYumUpdateMetric(ctx, secPkgs)
		if err == nil {
			secCount := 0
			for _, u := range uMetric {
				if u.Security {
					secCount++
				}
			}
			return []protocol.Metric{protocol.UpdateMetric{
				PendingCount:   len(uMetric),
				SecurityCount:  secCount,
				RebootRequired: checkRebootRequired(),
				PackageManager: "yum",
				Packages:       uMetric,
			}}, nil
		}
	}

	for _, checker := range updateCheckers {
		if _, err := exec.LookPath(checker.exe); err != nil {
			continue
		}

		updates, mgr, err := runUpdateCheck(ctx, checker)
		if err != nil {
			continue
		}

		secCount := 0
		for _, u := range updates {
			if u.Security {
				secCount++
			}
		}

		return []protocol.Metric{protocol.UpdateMetric{
			PendingCount:   len(updates),
			SecurityCount:  secCount,
			RebootRequired: checkRebootRequired(),
			PackageManager: mgr,
			Packages:       updates,
		}}, nil
	}

	return nil, nil
}

func runUpdateCheck(ctx context.Context, checker updateChecker) ([]protocol.PendingUpdate, string, error) {
	cmd := exec.CommandContext(ctx, checker.exe, checker.args...)
	out, err := cmd.Output()
	// yum check-update returns exit code 100 when updates are available
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if checker.name == "yum" && exitErr.ExitCode() == 100 {
				// pass
			} else {
				return nil, "", err
			}
		} else {
			return nil, "", err
		}
	}

	updates, err := scanUpdates(bytes.NewReader(out), checker.parse)
	if err != nil {
		return nil, "", err
	}

	return updates, checker.name, nil
}

// parseAptLine parses "apt list --upgradable" output.
func parseAptLine(line string) (protocol.PendingUpdate, bool) {
	if strings.HasPrefix(line, "Listing") || strings.TrimSpace(line) == "" {
		// skip header line
		return protocol.PendingUpdate{}, false
	}
	name, _, ok := strings.Cut(line, "/")
	if !ok {
		return protocol.PendingUpdate{}, false
	}

	// extract version
	after := line[len(name)+1:]
	fields := strings.Fields(after)
	version := ""
	if len(fields) >= 2 {
		version = fields[1]
	}

	isSecurity := strings.Contains(after, "-security")

	return protocol.PendingUpdate{
		Name:     name,
		Version:  version,
		Security: isSecurity,
	}, true
}

// collectYumUpdates parses "yum updateinfo list security --quiet" and
// returns a map of packages that require security updates.
func collectYumUpdates(ctx context.Context) (map[string]bool, error) {
	updates := make(map[string]bool)
	cmd := exec.CommandContext(ctx, "yum", "updateinfo", "list", "security", "--quiet")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		updates[extractRPMName(fields[2])] = true
	}

	return updates, scanner.Err()
}

// buildYumUpdateMetric parses "yum check-update --quiet" output.
func buildYumUpdateMetric(ctx context.Context, updates map[string]bool) ([]protocol.PendingUpdate, error) {
	cmd := exec.CommandContext(ctx, "yum", "check-update", "--quiet")
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 100 {
			// normal: updates available
		} else {
			return nil, err
		}
	}

	var pu []protocol.PendingUpdate
	scanner := bufio.NewScanner(bytes.NewReader(out))

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		// skip non-package lines
		if !strings.Contains(line, ".") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		// fields[0] = program.arch, fields[1] = version, fields[2] = repo
		pkgArch := fields[0]
		name := pkgArch
		if dotIdx := strings.LastIndex(pkgArch, "."); dotIdx > 0 {
			name = pkgArch[:dotIdx]
		}

		pu = append(pu, protocol.PendingUpdate{
			Name:     name,
			Version:  fields[1],
			Security: requiresUpdate(name, updates),
		})
	}

	return pu, scanner.Err()
}

// extractRPMName returns the package name from an RPM NVRA.
func extractRPMName(nvra string) string {
	// strip .arch
	if i := strings.LastIndex(nvra, "."); i > 0 {
		nvra = nvra[:i]
	}
	// strip -version-release (two rightmost hyphen-separated segments)
	if i := strings.LastIndex(nvra, "-"); i > 0 {
		if j := strings.LastIndex(nvra[:i], "-"); j > 0 {
			return nvra[:j]
		}
	}
	return nvra
}

// parseApkLine parses "apk upgrade --simulate" output.
func parseApkLine(line string) (protocol.PendingUpdate, bool) {
	matches := apkUpgradeRe.FindStringSubmatch(line)
	if matches == nil {
		return protocol.PendingUpdate{}, false
	}

	return protocol.PendingUpdate{
		Name:     matches[1],
		Version:  matches[2],
		Security: false, // security updates aren't distinguished
	}, true
}

// parsePacmanLine parses "checkupdates" output.
func parsePacmanLine(line string) (protocol.PendingUpdate, bool) {
	fields := strings.Fields(line)
	if len(fields) < 4 || fields[2] != "->" {
		return protocol.PendingUpdate{}, false
	}

	return protocol.PendingUpdate{
		Name:     fields[0],
		Version:  fields[3],
		Security: false, // security updates aren't distinguished
	}, true
}

func requiresUpdate(pkg string, updates map[string]bool) bool {
	_, ok := updates[pkg]
	return ok
}

func checkRebootRequired() bool {
	// Debian/Ubuntu: /var/run/reboot-required
	if _, err := os.Stat("/var/run/reboot-required"); err == nil {
		return true
	}

	// RHEL/CentOS: needs-restarting -r exit code 1
	if path, err := exec.LookPath("needs-restarting"); err == nil {
		cmd := exec.Command(path, "-r")
		if err := cmd.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				return true
			}
		}
	}
	return false
}
