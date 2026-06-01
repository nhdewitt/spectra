package server

import (
	"fmt"
	"strings"
)

const (
	systemdUnit = `[Unit]
Description=Spectra Monitoring Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/spectra-agent -config /etc/spectra/agent.json
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`

	launchdPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.spectra.agent</string>

	<key>ProgramArguments</key>
	<array>
		<string>/usr/local/bin/spectra-agent</string>
		<string>-config</string>
		<string>/usr/local/etc/spectra/agent.json</string>
	</array>

	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>

	<key>StandardOutPath</key>
	<string>/usr/local/var/log/spectra-agent.log</string>
	<key>StandardErrorPath</key>
	<string>/usr/local/var/log/spectra-agent.err</string>
</dict>
</plist>
`

	rcdRCScript = `#!/bin/sh

# PROVIDE: spectra_agent
# REQUIRE: NETWORKING
# KEYWORD: shutdown

. /etc/rc.subr

name="spectra_agent"
rcvar="${name}_enable"

: ${spectra_agent_enable:="NO"}
: ${spectra_agent_config:="/usr/local/etc/spectra/agent.json"}

pidfile="/var/run/${name}.pid"
procname="/usr/local/bin/spectra-agent"
command="/usr/sbin/daemon"
command_args="-p ${pidfile} -r -f ${procname} -config ${spectra_agent_config}"

load_rc_config $name
run_rc_command "$1"
`
)

// upgradeInstructions describes platform-specific steps for upgrading the agent binary.
type upgradeInstructions struct {
	Type  string `json:"type"`
	Steps string `json:"steps"`
}

// uninstallInstructions describes platform-specific steps for removing the agent.
type uninstallInstructions struct {
	Type  string `json:"type"`
	Steps string `json:"steps"`
}

// generateInstallInstructions returns platform-specific installation steps.
func generateInstallInstructions(p *platformInfo, serverURL, token, caCertPEM string) installInstructions {
	switch p.OS {
	case "linux":
		return generateSystemdInstructions(p, serverURL, token, caCertPEM)
	case "darwin":
		return generateLaunchdInstructions(p, serverURL, token, caCertPEM)
	case "freebsd":
		return generateRCDInstructions(p, serverURL, token, caCertPEM)
	case "windows":
		return generateWindowsInstructions(p, serverURL, token, caCertPEM)
	default:
		return installInstructions{
			Type:  "manual",
			Steps: fmt.Sprintf("./%s -register -server %s -token %s", p.Filename, serverURL, token),
		}
	}
}

// generateUpgradeInstructions returns platform-specific upgrade steps.
func generateUpgradeInstructions(p *platformInfo) upgradeInstructions {
	switch p.OS {
	case "linux":
		return generateSystemdUpgrade(p)
	case "darwin":
		return generateLaunchdUpgrade(p)
	case "freebsd":
		return generateRCDUpgrade(p)
	case "windows":
		return generateWindowsUpgrade(p)
	default:
		return upgradeInstructions{
			Type:  "manual",
			Steps: "Replace the binary and restart the agent.",
		}
	}
}

// generateUninstallInstructions returns platform-specific uninstall steps.
func generateUninstallInstructions(p *platformInfo) uninstallInstructions {
	switch p.OS {
	case "linux":
		return generateSystemdUninstall()
	case "darwin":
		return generateLaunchdUninstall()
	case "freebsd":
		return generateRCDUninstall()
	case "windows":
		return generateWindowsUninstall()
	default:
		return uninstallInstructions{
			Type:  "manual",
			Steps: "Stop and remove the agent binary and config.",
		}
	}
}

// agentConfigJSON returns the JSON config block for the agent,
// with optional CA cert path for TLS.
func agentConfigJSON(serverURL, token, caCertPath string) string {
	if caCertPath != "" {
		return fmt.Sprintf(`{
	"server": "%s",
	"token": "%s",
	"ca_cert": "%s"
}`, serverURL, token, caCertPath)
	}

	return fmt.Sprintf(`{
	"server": "%s",
	"token": "%s"
}`, serverURL, token)
}

func generateSystemdInstructions(p *platformInfo, serverURL, token, caCertPEM string) installInstructions {
	caCertPath := ""
	if caCertPEM != "" {
		caCertPath = "/etc/spectra/ca.crt"
	}
	configJSON := agentConfigJSON(serverURL, token, caCertPath)

	steps := []string{
		fmt.Sprintf(`Install the binary
sudo install -d /usr/local/bin
sudo install -m 0755 %s /usr/local/bin/spectra-agent`, p.Filename),

		`Create the service user and group
sudo groupadd --system spectra 2>/dev/null || true
sudo useradd --system --no-create-home --gid spectra --shell /usr/sbin/nologin spectra 2>/dev/null || true`,

		`Create the config directory
sudo install -d -m 0755 -o spectra -g spectra /etc/spectra`,
	}

	if caCertPEM != "" {
		steps = append(steps, fmt.Sprintf(`Install the CA certificate
sudo tee /etc/spectra/ca.crt > /dev/null <<'EOF'
%sEOF
sudo chmod 0644 /etc/spectra/ca.crt`, caCertPEM))
	}

	steps = append(steps,
		fmt.Sprintf(`Save the config
sudo tee /etc/spectra/agent.json > /dev/null <<'EOF'
%s
EOF
sudo chown spectra:spectra /etc/spectra/agent.json
sudo chmod 0600 /etc/spectra/agent.json`, configJSON),

		fmt.Sprintf(`Install the systemd unit
sudo tee /etc/systemd/system/spectra-agent.service > /dev/null <<'EOF'
%s
EOF`, systemdUnit),

		`Reload systemd and start the service
sudo systemctl daemon-reload
sudo systemctl enable --now spectra-agent`,

		`Verify
sudo systemctl status spectra-agent --no-pager
sudo journalctl -u spectra-agent -n 50 --no-pager`,
	)

	return installInstructions{
		Type:    "systemd",
		Content: systemdUnit,
		Steps:   numberSteps(steps),
	}
}

func generateSystemdUpgrade(p *platformInfo) upgradeInstructions {
	steps := fmt.Sprintf(`sudo systemctl stop spectra-agent
sudo install -m 0755 %s /usr/local/bin/spectra-agent
sudo systemctl start spectra-agent
sudo systemctl status spectra-agent --no-pager
`, p.Filename)

	return upgradeInstructions{
		Type:  "systemd",
		Steps: steps,
	}
}

func generateSystemdUninstall() uninstallInstructions {
	steps := `sudo systemctl disable --now spectra-agent
sudo rm -f /etc/systemd/system/spectra-agent.service
sudo systemctl daemon-reload
sudo rm -f /usr/local/bin/spectra-agent
sudo rm -rf /etc/spectra
sudo userdel spectra 2>/dev/null || true
sudo groupdel spectra 2>/dev/null || true
`

	return uninstallInstructions{
		Type:  "systemd",
		Steps: steps,
	}
}

func generateLaunchdInstructions(p *platformInfo, serverURL, token, caCertPEM string) installInstructions {
	caCertPath := ""
	if caCertPEM != "" {
		caCertPath = "/usr/local/etc/spectra/ca.crt"
	}
	configJSON := agentConfigJSON(serverURL, token, caCertPath)

	steps := []string{
		fmt.Sprintf(`Install the binary
sudo install -d /usr/local/bin
sudo install -m 0755 %s /usr/local/bin/spectra-agent`, p.Filename),

		`Create the config directory
sudo install -d -m 0755 /usr/local/etc/spectra`,
	}

	if caCertPEM != "" {
		steps = append(steps, fmt.Sprintf(`Install the CA certificate
sudo tee /usr/local/etc/spectra/ca.crt > /dev/null <<'EOF'
%sEOF
sudo chmod 0644 /usr/local/etc/spectra/ca.crt`, caCertPEM))
	}

	steps = append(steps,
		fmt.Sprintf(`Save the config
sudo tee /usr/local/etc/spectra/agent.json > /dev/null <<'EOF'
%s
EOF
sudo chmod 0600 /usr/local/etc/spectra/agent.json`, configJSON),

		`Create the log directory
sudo install -d -m 0755 /usr/local/var/log`,

		fmt.Sprintf(`Install the launchd plist
sudo tee /Library/LaunchDaemons/com.spectra.agent.plist > /dev/null <<'EOF'
%s
EOF`, launchdPlist),

		`Load and start the daemon
sudo launchctl bootstrap system /Library/LaunchDaemons/com.spectra.agent.plist`,

		`Verify
sudo launchctl print system/com.spectra.agent
sudo tail -n 50 /usr/local/var/log/spectra-agent.log
sudo tail -n 50 /usr/local/var/log/spectra-agent.err`,
	)

	return installInstructions{
		Type:    "launchd",
		Content: launchdPlist,
		Steps:   numberSteps(steps),
	}
}

func generateLaunchdUpgrade(p *platformInfo) upgradeInstructions {
	steps := fmt.Sprintf(`sudo launchctl bootout system /Library/LaunchDaemons/com.spectra.agent.plist
sudo install -m 0755 %s /usr/local/bin/spectra-agent
sudo launchctl bootstrap system /Library/LaunchDaemons/com.spectra.agent.plist
`, p.Filename)

	return upgradeInstructions{
		Type:  "launchd",
		Steps: steps,
	}
}

func generateLaunchdUninstall() uninstallInstructions {
	steps := `sudo launchctl bootout system /Library/LaunchDaemons/com.spectra.agent.plist 2>/dev/null || true
sudo rm -f /Library/LaunchDaemons/com.spectra.agent.plist
sudo rm -f /usr/local/bin/spectra-agent
sudo rm -rf /usr/local/etc/spectra
sudo rm -f /usr/local/var/log/spectra-agent.log /usr/local/var/log/spectra-agent.err
`

	return uninstallInstructions{
		Type:  "launchd",
		Steps: steps,
	}
}

func generateRCDInstructions(p *platformInfo, serverURL, token, caCertPEM string) installInstructions {
	caCertPath := ""
	if caCertPEM != "" {
		caCertPath = "/usr/local/etc/spectra/ca.crt"
	}
	configJSON := agentConfigJSON(serverURL, token, caCertPath)

	steps := []string{
		fmt.Sprintf(`Install the binary
sudo install -d /usr/local/bin
sudo install -m 0755 %s /usr/local/bin/spectra-agent`, p.Filename),

		`Create the config directory
sudo install -d -m 0755 /usr/local/etc/spectra`,
	}

	if caCertPEM != "" {
		steps = append(steps, fmt.Sprintf(`Install the CA certificate
sudo tee /usr/local/etc/spectra/ca.crt > /dev/null <<'EOF'
%sEOF
sudo chmod 0644 /usr/local/etc/spectra/ca.crt`, caCertPEM))
	}

	steps = append(steps,
		fmt.Sprintf(`Save the config
sudo tee /usr/local/etc/spectra/agent.json > /dev/null <<'EOF'
%s
EOF
sudo chmod 0600 /usr/local/etc/spectra/agent.json`, configJSON),

		fmt.Sprintf(`Install the rc.d script
sudo tee /usr/local/etc/rc.d/spectra_agent > /dev/null <<'EOF'
%s
EOF
sudo chmod 0555 /usr/local/etc/rc.d/spectra_agent`, rcdRCScript),

		`Enable the service
sudo sysrc spectra_agent_enable=YES`,

		`Start the service
sudo service spectra_agent start`,

		`Verify
sudo service spectra_agent status`,
	)

	return installInstructions{
		Type:    "rc_d",
		Content: rcdRCScript,
		Steps:   numberSteps(steps),
	}
}

func generateRCDUpgrade(p *platformInfo) upgradeInstructions {
	steps := fmt.Sprintf(`sudo service spectra_agent stop
sudo install -m 0755 %s /usr/local/bin/spectra-agent
sudo service spectra_agent start
sudo service spectra_agent status
`, p.Filename)

	return upgradeInstructions{
		Type:  "rc_d",
		Steps: steps,
	}
}

func generateRCDUninstall() uninstallInstructions {
	steps := `sudo service spectra_agent stop 2>/dev/null || true
sudo sysrc -x spectra_agent_enable
sudo rm -f /usr/local/etc/rc.d/spectra_agent
sudo rm -f /usr/local/bin/spectra-agent
sudo rm -rf /usr/local/etc/spectra
sudo rm -f /var/run/spectra_agent.pid
`

	return uninstallInstructions{
		Type:  "rc_d",
		Steps: steps,
	}
}

func generateWindowsInstructions(p *platformInfo, serverURL, token, caCertPEM string) installInstructions {
	caCertPath := ""
	if caCertPEM != "" {
		caCertPath = `C:\Spectra\ca.crt`
	}
	configJSON := agentConfigJSON(serverURL, token, caCertPath)

	steps := []string{
		`Open PowerShell as Administrator`,

		fmt.Sprintf(`Create the install directory and copy the binary
New-Item -Path 'C:\Spectra' -ItemType Directory -Force | Out-Null
Copy-Item '.\%s' 'C:\Spectra\spectra-agent.exe' -Force`, p.Filename),
	}

	if caCertPEM != "" {
		steps = append(steps, fmt.Sprintf(`Install the CA certificate
@'
%s'@ | Set-Content -Path 'C:\Spectra\ca.crt'`, caCertPEM))
	}

	steps = append(steps,
		fmt.Sprintf(`Create the config
@'
%s
'@ | Set-Content -Path 'C:\Spectra\agent.json'`, configJSON),

		`Install the Windows service
sc.exe create SpectraAgent binPath= "C:\Spectra\spectra-agent.exe -config C:\Spectra\agent.json" start= auto
sc.exe description SpectraAgent "Spectra Monitoring Agent"
sc.exe failure SpectraAgent reset= 60 actions= restart/5000/restart/5000/restart/5000`,

		`Start the service
sc.exe start SpectraAgent`,

		`Verify
sc.exe query SpectraAgent`,
	)

	return installInstructions{
		Type:    "windows_service",
		Content: "",
		Steps:   numberSteps(steps),
	}
}

func generateWindowsUpgrade(p *platformInfo) upgradeInstructions {
	steps := fmt.Sprintf(`sc.exe stop SpectraAgent
Copy-Item '.\%s' 'C:\Spectra\spectra-agent.exe' -Force
sc.exe start SpectraAgent
sc.exe query SpectraAgent
`, p.Filename)

	return upgradeInstructions{
		Type:  "windows_service",
		Steps: steps,
	}
}

func generateWindowsUninstall() uninstallInstructions {
	steps := `sc.exe stop SpectraAgent
sc.exe delete SpectraAgent
Remove-Item 'C:\Spectra' -Recurse -Force -ErrorAction SilentlyContinue
`

	return uninstallInstructions{
		Type:  "windows_service",
		Steps: steps,
	}
}

func numberSteps(steps []string) string {
	var b strings.Builder
	for i, s := range steps {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%d. %s\n", i+1, s)
	}
	return b.String()
}
