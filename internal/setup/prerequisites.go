package setup

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"unicode"
)

const targetPostgresMajorVersion = "16"

type Distro struct {
	Family   DistroFamily
	ID       string // ID from os-release (e.g. "debian", "ubuntu", "rocky")
	Codename string // Debian/Ubuntu codename (e.g. "bookworm", "jammy")
	RHELVer  string // RHEL major version (e.g. "8", "9")
	PkgMgr   string // "apt-get", "dnf", "yum", "pkg"
}

// DistroFamily represents a Linux distribution family.
type DistroFamily int

const (
	FamilyUnknown DistroFamily = iota
	FamilyDebian
	FamilyUbuntu
	FamilyRHEL // RHEL, CentOS, AlmaLinux, Rocky, Amazon Linux
	FamilyFreeBSD
)

func (f DistroFamily) String() string {
	switch f {
	case FamilyDebian:
		return "Debian"
	case FamilyUbuntu:
		return "Ubuntu"
	case FamilyRHEL:
		return "RHEL/CentOS"
	case FamilyFreeBSD:
		return "FreeBSD"
	default:
		return "Unknown"
	}
}

// DetectDistro identifies the OS and gathers platform-specific details.
func DetectDistro() (*Distro, error) {
	if runtime.GOOS == "freebsd" {
		return &Distro{
			Family: FamilyFreeBSD,
			PkgMgr: "pkg",
		}, nil
	}

	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	fields, err := parseOSRelease("/etc/os-release")
	if err != nil {
		return nil, fmt.Errorf("could not read /etc/os-release: %w", err)
	}

	d := &Distro{
		ID: fields["ID"],
	}

	// Determine family from ID, then ID_LIKE
	idLike := strings.Fields(fields["ID_LIKE"])
	allIDs := append([]string{d.ID}, idLike...)

	for _, id := range allIDs {
		switch id {
		case "ubuntu":
			d.Family = FamilyUbuntu
		case "debian":
			if d.Family == FamilyUnknown {
				d.Family = FamilyDebian
			}
		case "rhel", "centos", "fedora":
			if d.Family == FamilyUnknown {
				d.Family = FamilyRHEL
			}
		}
		if d.Family != FamilyUnknown {
			break
		}
	}

	if d.Family == FamilyUnknown {
		switch d.ID {
		case "amzn":
			d.Family = FamilyRHEL
		case "rocky", "almalinux":
			d.Family = FamilyRHEL
		}
	}

	if d.Family == FamilyUnknown {
		return nil, fmt.Errorf("unsupported Linux distribution: %s", d.ID)
	}

	// Family-specific details
	switch d.Family {
	case FamilyDebian, FamilyUbuntu:
		d.PkgMgr = "apt-get"
		d.Codename, _ = runCmd("lsb_release", "-cs")
		if d.Codename == "" {
			d.Codename = fields["VERSION_CODENAME"] // fallback
		}
	case FamilyRHEL:
		if commandExists("dnf") {
			d.PkgMgr = "dnf"
		} else {
			d.PkgMgr = "yum"
		}
		ver, _ := runCmd("rpm", "-E", "%rhel")
		if isNumeric(ver) {
			d.RHELVer = ver
		}
	}

	return d, nil
}

// parseOSRelease reads /etc/os-release into a key-value map.
func parseOSRelease(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	fields := make(map[string]string)
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		v = strings.TrimSpace(v)
		v = strings.Trim(v, `"'`)
		fields[strings.ToUpper(strings.TrimSpace(k))] = strings.ToLower(v)
	}

	return fields, nil
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// validIdent checks if a string is a safe SQL identifier.
func validIdent(s string) bool {
	if s == "" || len(s) > 63 {
		return false
	}
	for i, r := range s {
		if r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			continue
		}
		if i > 0 && r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
}

// escapePassword escapes single quotes for SQL string literals.
func escapePassword(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// EnsurePrerequisites checks and installs PostgreSQL + TimescaleDB.
func EnsurePrerequisites() error {
	d, err := DetectDistro()
	if err != nil {
		return fmt.Errorf("%w - install PostgreSQL and TimescaleDB manually", err)
	}

	fmt.Printf("  Detected: %s (%s)\n", d.Family, d.ID)

	if d.Family == FamilyDebian || d.Family == FamilyUbuntu {
		if _, err := runCmd(d.PkgMgr, "install", "-y", "-qq", "gnupg", "lsb-release", "wget", "curl", "ca-certificates"); err != nil {
			return fmt.Errorf("install base dependencies: %w", err)
		}
	}

	if commandExists("psql") {
		fmt.Println("  PostgreSQL: found")
	} else {
		fmt.Println("  PostgreSQL: not found, installing...")
		if err := d.installPostgres(); err != nil {
			return fmt.Errorf("install PostgreSQL: %w", err)
		}
		fmt.Println("  PostgreSQL: installed")
	}

	if d.hasTimescaleDB() {
		fmt.Println("  TimescaleDB: found")
	} else {
		fmt.Println("  TimescaleDB: not found, installing...")
		if err := d.installTimescaleDB(); err != nil {
			return fmt.Errorf("install TimescaleDB: %w", err)
		}
		fmt.Println("  TimescaleDB: installed")
	}

	fmt.Print("  Tuning TimescaleDB... ")
	if err := d.tuneTimescaleDB(); err != nil {
		fmt.Printf("SKIP (%v)\n", err)
	} else {
		fmt.Println("OK")
	}

	if err := d.postgresService("start"); err != nil {
		return fmt.Errorf("start PostgreSQL: %w", err)
	}

	return nil
}

// CreateDatabase creates the spectra user and database via psql.
func CreateDatabase(dbName, dbUser, dbPass string) error {
	if !validIdent(dbName) {
		return fmt.Errorf("invalid database name: %q", dbName)
	}
	if !validIdent(dbUser) {
		return fmt.Errorf("invalid database user: %q", dbUser)
	}

	escapedPass := escapePassword(dbPass)

	if _, err := runCmd("su", "-", "postgres", "-c", fmt.Sprintf(`psql -c "CREATE USER %s WITH PASSWORD '%s';"`, dbUser, escapedPass)); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("create user: %w", err)
		}
	}

	if _, err := runCmd("su", "-", "postgres", "-c", fmt.Sprintf(`psql -c "CREATE DATABASE %s OWNER %s;"`, dbName, dbUser)); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("create database: %w", err)
		}
	}

	if _, err := runCmd("su", "-", "postgres", "-c", fmt.Sprintf(`psql -c "GRANT ALL PRIVILEGES ON DATABASE %s TO %s;"`, dbName, dbUser)); err != nil {
		return fmt.Errorf("grant privileges: %w", err)
	}

	return nil
}

func (d *Distro) installPostgres() error {
	switch d.Family {
	case FamilyDebian, FamilyUbuntu:
		return d.installPostgresDebian()
	case FamilyRHEL:
		return d.installPostgresRHEL()
	case FamilyFreeBSD:
		return d.installPostgresFreeBSD()
	default:
		return fmt.Errorf("unsupported distro: %s", d.Family)
	}
}

func (d *Distro) installPostgresDebian() error {
	if _, err := runCmd(d.PkgMgr, "update", "-qq"); err != nil {
		return err
	}

	if _, err := runCmd("sh", "-c", `curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc | gpg --dearmor --yes -o /usr/share/keyrings/postgresql.gpg`); err != nil {
		return fmt.Errorf("add pgdg key: %w", err)
	}

	if d.Codename == "" {
		d.Codename, _ = runCmd("lsb_release", "-cs")
	}
	if d.Codename == "" {
		return fmt.Errorf("could not determine distro codename")
	}

	repoLine := fmt.Sprintf("deb [signed-by=/usr/share/keyrings/postgresql.gpg] https://apt.postgresql.org/pub/repos/apt %s-pgdg main\n", d.Codename)
	if err := os.WriteFile("/etc/apt/sources.list.d/pgdg.list", []byte(repoLine), 0644); err != nil {
		return fmt.Errorf("write pgdg repo: %w", err)
	}

	if _, err := runCmd(d.PkgMgr, "update", "-qq"); err != nil {
		return err
	}

	pgPkg := fmt.Sprintf("postgresql-%s", targetPostgresMajorVersion)
	if _, err := runCmd(d.PkgMgr, "install", "-y", "-qq", pgPkg); err != nil {
		return err
	}

	return nil
}

func (d *Distro) installPostgresRHEL() error {
	if d.RHELVer == "" {
		return fmt.Errorf("could not determine RHEL version")
	}

	// Disable built-in PostgreSQL module on RHEL 8+.
	_, _ = runCmd(d.PkgMgr, "module", "disable", "postgresql", "-y")

	repoURL := fmt.Sprintf("https://download.postgresql.org/pub/repos/yum/reporpms/EL-%s-x86_64/pgdg-redhat-repo-latest.noarch.rpm", d.RHELVer)
	if _, err := runCmd(d.PkgMgr, "install", "-y", repoURL); err != nil {
		return fmt.Errorf("install pgdg repo: %w", err)
	}

	serverPkg := fmt.Sprintf("postgresql%s-server", targetPostgresMajorVersion)
	if _, err := runCmd(d.PkgMgr, "install", "-y", serverPkg); err != nil {
		return err
	}

	initDB := fmt.Sprintf("/usr/pgsql-%s/bin/postgresql-%s-setup", targetPostgresMajorVersion, targetPostgresMajorVersion)
	if _, err := runCmd(initDB, "initdb"); err != nil {
		return fmt.Errorf("initdb failed: %w", err)
	}

	svcName := fmt.Sprintf("postgresql-%s", targetPostgresMajorVersion)
	if _, err := runCmd("systemctl", "enable", "--now", svcName); err != nil {
		return fmt.Errorf("enable postgresql: %w", err)
	}

	return nil
}

func (d *Distro) installPostgresFreeBSD() error {
	pgPkg := fmt.Sprintf("postgresql%s-server", targetPostgresMajorVersion)
	if _, err := runCmd("pkg", "install", "-y", pgPkg); err != nil {
		return err
	}

	if _, err := runCmd("sysrc", "postgresql_enable=YES"); err != nil {
		return fmt.Errorf("enable postgresql: %w", err)
	}

	dataDir := fmt.Sprintf("/var/db/postgres/data%s", targetPostgresMajorVersion)
	if _, err := os.Stat(dataDir); errors.Is(err, os.ErrNotExist) {
		if _, err := runCmd("service", "postgresql", "initdb"); err != nil {
			return fmt.Errorf("initdb failed: %w", err)
		}
	}

	if _, err := runCmd("service", "postgresql", "start"); err != nil {
		return fmt.Errorf("start postgresql: %w", err)
	}

	return nil
}

func (d *Distro) hasTimescaleDB() bool {
	pkg := fmt.Sprintf("timescaledb-2-postgresql-%s", targetPostgresMajorVersion)

	switch d.Family {
	case FamilyDebian, FamilyUbuntu:
		out, err := runCmd("dpkg-query", "-W", "-f=${Status}", pkg)
		return err == nil && strings.Contains(out, "install ok installed")
	case FamilyRHEL:
		_, err := runCmd("rpm", "-q", pkg)
		return err == nil
	case FamilyFreeBSD:
		bsdPkg := fmt.Sprintf("timescaledb-2-pg%s", targetPostgresMajorVersion)
		_, err := runCmd("pkg", "info", bsdPkg)
		return err == nil
	default:
		return false
	}
}

func (d *Distro) installTimescaleDB() error {
	switch d.Family {
	case FamilyDebian, FamilyUbuntu:
		return d.installTimescaleDBDebian()
	case FamilyRHEL:
		return d.installTimescaleDBRHEL()
	case FamilyFreeBSD:
		return d.installTimescaleDBFreeBSD()
	default:
		return fmt.Errorf("unsupported distro: %s", d.Family)
	}
}

func (d *Distro) installTimescaleDBDebian() error {
	if _, err := runCmd("sh", "-c", `curl -fsSL https://packagecloud.io/timescale/timescaledb/gpgkey | gpg --dearmor --yes -o /usr/share/keyrings/timescaledb.gpg`); err != nil {
		return fmt.Errorf("add timescaledb key: %w", err)
	}

	if d.Codename == "" {
		return fmt.Errorf("could not determine distro codename")
	}

	repoDistro := "ubuntu"
	if d.Family == FamilyDebian {
		repoDistro = "debian"
	}

	repoLine := fmt.Sprintf("deb [signed-by=/usr/share/keyrings/timescaledb.gpg] https://packagecloud.io/timescale/timescaledb/%s/ %s main\n", repoDistro, d.Codename)
	if err := os.WriteFile("/etc/apt/sources.list.d/timescaledb.list", []byte(repoLine), 0644); err != nil {
		return fmt.Errorf("write timescaledb repo: %w", err)
	}

	if _, err := runCmd(d.PkgMgr, "update", "-qq"); err != nil {
		return err
	}

	tsPkg := fmt.Sprintf("timescaledb-2-postgresql-%s", targetPostgresMajorVersion)
	if _, err := runCmd(d.PkgMgr, "install", "-y", "-qq", tsPkg); err != nil {
		return err
	}

	return nil
}

func (d *Distro) installTimescaleDBRHEL() error {
	if d.RHELVer == "" {
		return fmt.Errorf("could not determine RHEL version")
	}

	repoContent := fmt.Sprintf(`[timescaledb]
name=TimescaleDB
baseurl=https://packagecloud.io/timescale/timescaledb/el/%s/$basearch
gpgcheck=0
enabled=1
`, d.RHELVer)

	if err := os.WriteFile("/etc/yum.repos.d/timescaledb.repo", []byte(repoContent), 0644); err != nil {
		return fmt.Errorf("write timescaledb repo: %w", err)
	}

	tsPkg := fmt.Sprintf("timescaledb-2-postgresql-%s", targetPostgresMajorVersion)
	if _, err := runCmd(d.PkgMgr, "install", "-y", tsPkg); err != nil {
		return err
	}

	return nil
}

func (d *Distro) installTimescaleDBFreeBSD() error {
	tsPkg := fmt.Sprintf("timescaledb-2-pg%s", targetPostgresMajorVersion)
	if _, err := runCmd("pkg", "install", "-y", tsPkg); err != nil {
		return err
	}

	return nil
}

func (d *Distro) tuneTimescaleDB() error {
	if commandExists("timescaledb-tune") {
		_, err := runCmd("timescaledb-tune", "--yes", "--quiet")
		if err == nil {
			return d.postgresService("restart")
		}
		return err
	}

	confPath := d.findPgConf()
	if confPath == "" {
		return fmt.Errorf("could not find postgresql.conf")
	}

	data, err := os.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("read postgresql.conf: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	found := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || !strings.HasPrefix(trimmed, "shared_preload_libraries") {
			continue
		}

		found = true
		if strings.Contains(trimmed, "timescaledb") {
			return nil
		}

		start := strings.IndexByte(trimmed, '\'')
		end := strings.LastIndexByte(trimmed, '\'')
		if start >= 0 && end > start {
			existing := trimmed[start+1 : end]
			if existing == "" {
				lines[i] = "shared_preload_libraries = 'timescaledb'"
			} else {
				lines[i] = fmt.Sprintf("shared_preload_libraries = '%s,timescaledb'", existing)
			}
		} else {
			lines[i] = "shared_preload_libraries = 'timescaledb'"
		}
		break
	}

	if !found {
		lines = append(lines, "shared_preload_libraries = 'timescaledb'")
	}

	if err := os.WriteFile(confPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		return fmt.Errorf("write postgresql.conf: %w", err)
	}

	return d.postgresService("restart")
}

func (d *Distro) findPgConf() string {
	candidates := []string{
		fmt.Sprintf("/etc/postgresql/%s/main/postgresql.conf", targetPostgresMajorVersion),
		fmt.Sprintf("/var/lib/pgsql/%s/data/postgresql.conf", targetPostgresMajorVersion),
		fmt.Sprintf("/var/db/postgres/data%s/postgresql.conf", targetPostgresMajorVersion),
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func (d *Distro) postgresService(action string) error {
	switch d.Family {
	case FamilyFreeBSD:
		_, err := runCmd("service", "postgresql", action)
		return err
	default:
		svc := fmt.Sprintf("postgresql-%s", targetPostgresMajorVersion)
		if _, err := runCmd("systemctl", action, svc); err != nil {
			_, err = runCmd("systemctl", action, "postgresql")
			return err
		}
		return nil
	}
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func runCmd(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// inContainer reports whether the process is running inside a container, so
// setup can skip systemd service management.
func inContainer() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	// Podman and systemd-nspawn set this
	if os.Getenv("container") != "" {
		return true
	}
	return false
}
