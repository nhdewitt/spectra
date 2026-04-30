package server

import "fmt"

// upgradeInstructions returns platform-specific steps for upgrading the agent binary.
type upgradeInstructions struct {
	Type  string `json:"type"`
	Steps string `json:"steps"`
}

// uninstallInstructions returns platform-specific steps for removing the agent.
type uninstallInstructions struct {
	Type  string `json:"type"`
	Steps string `json:"steps"`
}

// generateInstallInstructions returns platform-specific installation steps.
func generateInstallInstructions(p *platformInfo, serverURL, token string) installInstructions {
	switch p.OS {
	case "linux":
		return generateSystemdInstructions(p, serverURL, token)
	case "darwin":
		return generateLaunchdInstructions(p, serverURL, token)
	case "freebsd":
		return generateRCDInstructions(p, serverURL, token)
	case "windows":
		return generateWindowsInstructions(p, serverURL, token)
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

func generateSystemdInstructions(p *platformInfo, serverURL, token string) installInstructions {
	unit := `[Unit]
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

	steps := fmt.Sprintf(`1. Install the binary
sudo install -d /usr/local/bin
sudo install -m 0755 %s /usr/local/bin/spectra-agent

2. Create the service user and group
sudo groupadd --system spectra 2>/dev/null || true
sudo useradd --system --no-create-home --gid spectra --shell /usr/sbin/nologin spectra 2>/dev/null || true

3. Create the config directory
sudo install -d -m 0755 -o spectra -g spectra /etc/spectra

4. Save the config
sudo tee /etc/spectra/agent.json > /dev/null <<'EOF'
{
	"server": "%s",
	"token": "%s"
}
EOF
sudo chown spectra:spectra /etc/spectra/agent.json
sudo chmod 0600 /etc/spectra/agent.json

5. Install the systemd unit
sudo tee /etc/systemd/system/spectra-agent.service > /dev/null <<'EOF'
%s
EOF

6. Reload systemd and start the service
sudo systemctl daemon-reload
sudo systemctl enable --now spectra-agent

7. Verify
sudo systemctl status spectra-agent --no-pager
sudo journalctl -u spectra-agent -n 50 --no-pager
`, p.Filename, serverURL, token, unit)

	return installInstructions{
		Type:    "systemd",
		Content: unit,
		Steps:   steps,
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

func generateLaunchdInstructions(p *platformInfo, serverURL, token string) installInstructions {
	plist := `<?xml version="1.0" encoding="UTF-8"?>
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

	steps := fmt.Sprintf(`1. Install the binary
sudo install -d /usr/local/bin
sudo install -m 0755 %s /usr/local/bin/spectra-agent

2. Create the config directory
sudo install -d -m 0755 /usr/local/etc/spectra

3. Save the config
sudo tee /usr/local/etc/spectra/agent.json > /dev/null <<'EOF'
{
	"server": "%s",
	"token": "%s"
}
EOF
sudo chmod 0600 /usr/local/etc/spectra/agent.json

4. Create the log directory
sudo install -d -m 0755 /usr/local/var/log

5. Install the launchd plist
sudo tee /Library/LaunchDaemons/com.spectra.agent.plist > /dev/null <<'EOF'
%s
EOF

6. Load and start the daemon
sudo launchctl bootstrap system /Library/LaunchDaemons/com.spectra.agent.plist

7. Verify
sudo launchctl print system/com.spectra.agent
sudo tail -n 50 /usr/local/var/log/spectra-agent.log
sudo tail -n 50 /usr/local/var/log/spectra-agent.err
`, p.Filename, serverURL, token, plist)

	return installInstructions{
		Type:    "launchd",
		Content: plist,
		Steps:   steps,
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

func generateRCDInstructions(p *platformInfo, serverURL, token string) installInstructions {
	rcScript := `#!/bin/sh

# PROVIDE: spectra_agent
# REQUIRE: NETWORKING
# KEYWORD: shutdown

. /etc/rc.subr

name="spectra_agent"
rcvar="${name}_enable"

: ${spectra_agent_enable:="NO"}
: ${spectra_agent_user:="root"}
: ${spectra_agent_config:="/usr/local/etc/spectra/agent.json"}

pidfile="/var/run/${name}.pid"
command="/usr/sbin/daemon"
command_args="-p ${pidfile} -f /usr/local/bin/spectra-agent -config ${spectra_agent_config}"

load_rc_config $name
run_rc_command "$1"
`

	steps := fmt.Sprintf(`1. Install the binary
sudo install -d /usr/local/bin
sudo install -m 0755 %s /usr/local/bin/spectra-agent

2. Create the config directory
sudo install -d -m 0755 /usr/local/etc/spectra

3. Save the config
sudo tee /usr/local/etc/spectra/agent.json > /dev/null <<'EOF'
{
	"server": "%s",
	"token": "%s"
}
EOF
sudo chmod 0600 /usr/local/etc/spectra/agent.json

4. Install the rc.d script
sudo tee /usr/local/etc/rc.d/spectra_agent > /dev/null <<'EOF'
%s
EOF
sudo chmod 0555 /usr/local/etc/rc.d/spectra_agent

5. Enable the service
sudo sysrc spectra_agent_enable=YES

6. Start the service
sudo service spectra_agent start

7. Verify
sudo service spectra_agent status
`, p.Filename, serverURL, token, rcScript)

	return installInstructions{
		Type:    "rc_d",
		Content: rcScript,
		Steps:   steps,
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

func generateWindowsInstructions(p *platformInfo, serverURL, token string) installInstructions {
	steps := fmt.Sprintf(`1. Open PowerShell as Administrator

2. Create the install directory
New-Item -Path 'C:\Spectra' -ItemType Directory -Force | Out-Null

3. Copy the binary
Copy-Item '.\%s' 'C:\Spectra\spectra-agent.exe' -Force

4. Create the config
@'
{
	"server": "%s",
	"token": "%s"
}
'@ | Set-Content -Path 'C:\Spectra\agent.json'

5. Install the Windows service
sc.exe create SpectraAgent binPath= "C:\Spectra\spectra-agent.exe -config C:\Spectra\agent.json" start= auto
sc.exe description SpectraAgent "Spectra Monitoring Agent"

6. Start the service
sc.exe start SpectraAgent

7. Verify
sc.exe query SpectraAgent
`, p.Filename, serverURL, token)

	return installInstructions{
		Type:    "windows_service",
		Content: "",
		Steps:   steps,
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
