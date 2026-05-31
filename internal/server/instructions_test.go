package server

import (
	"strings"
	"testing"
)

func TestGenerateInstallInstructions(t *testing.T) {
	tests := []struct {
		name         string
		os           string
		wantType     string
		wantContains []string
	}{
		{
			name:     "linux",
			os:       "linux",
			wantType: "systemd",
			wantContains: []string{
				"systemctl",
				"/etc/systemd/system/spectra-agent.service",
				"/usr/local/bin/spectra-agent",
				"test-binary",
				"https://example.com",
				"test-token",
				"groupadd --system spectra",
				"useradd --system",
				"chown spectra:spectra",
				"chmod 0600",
			},
		},
		{
			name:     "darwin",
			os:       "darwin",
			wantType: "launchd",
			wantContains: []string{
				"launchctl bootstrap",
				"com.spectra.agent.plist",
				"/Library/LaunchDaemons/",
				"test-binary",
				"https://example.com",
				"test-token",
				"chmod 0600",
			},
		},
		{
			name:     "freebsd",
			os:       "freebsd",
			wantType: "rc_d",
			wantContains: []string{
				"service spectra_agent start",
				"/usr/local/etc/rc.d/spectra_agent",
				"sysrc spectra_agent_enable=YES",
				"test-binary",
				"https://example.com",
				"test-token",
				"chmod 0600",
			},
		},
		{
			name:     "windows",
			os:       "windows",
			wantType: "windows_service",
			wantContains: []string{
				"sc.exe create SpectraAgent",
				`C:\Spectra\spectra-agent.exe`,
				"test-binary",
				"https://example.com",
				"test-token",
			},
		},
		{
			name:     "unknown platform falls back to manual",
			os:       "plan9",
			wantType: "manual",
			wantContains: []string{
				"-register",
				"-server https://example.com",
				"-token test-token",
				"test-binary",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &platformInfo{OS: tt.os, Filename: "test-binary"}
			got := generateInstallInstructions(p, "https://example.com", "test-token", "")

			if got.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", got.Type, tt.wantType)
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(got.Steps, want) {
					t.Errorf("Steps missing %q\nsteps:\n%s", want, got.Steps)
				}
			}
		})
	}
}

func TestGenerateUpgradeInstructions(t *testing.T) {
	tests := []struct {
		name         string
		os           string
		wantType     string
		wantContains []string
	}{
		{
			name:         "linux",
			os:           "linux",
			wantType:     "systemd",
			wantContains: []string{"systemctl stop spectra-agent", "systemctl start spectra-agent", "test-binary"},
		},
		{
			name:         "darwin",
			os:           "darwin",
			wantType:     "launchd",
			wantContains: []string{"launchctl bootout", "launchctl bootstrap", "test-binary"},
		},
		{
			name:         "freebsd",
			os:           "freebsd",
			wantType:     "rc_d",
			wantContains: []string{"service spectra_agent stop", "service spectra_agent start", "test-binary"},
		},
		{
			name:         "windows",
			os:           "windows",
			wantType:     "windows_service",
			wantContains: []string{"sc.exe stop SpectraAgent", "sc.exe start SpectraAgent", "test-binary"},
		},
		{
			name:         "unknown platform falls back to manual",
			os:           "plan9",
			wantType:     "manual",
			wantContains: []string{"Replace the binary"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &platformInfo{OS: tt.os, Filename: "test-binary"}
			got := generateUpgradeInstructions(p)

			if got.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", got.Type, tt.wantType)
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(got.Steps, want) {
					t.Errorf("Steps missing %q\nsteps:\n%s", want, got.Steps)
				}
			}
		})
	}
}

func TestGenerateUninstallInstructions(t *testing.T) {
	tests := []struct {
		name         string
		os           string
		wantType     string
		wantContains []string
	}{
		{
			name:     "linux",
			os:       "linux",
			wantType: "systemd",
			wantContains: []string{
				"systemctl disable --now spectra-agent",
				"rm -f /etc/systemd/system/spectra-agent.service",
				"rm -rf /etc/spectra",
				"userdel spectra",
				"groupdel spectra",
			},
		},
		{
			name:     "darwin",
			os:       "darwin",
			wantType: "launchd",
			wantContains: []string{
				"launchctl bootout",
				"com.spectra.agent.plist",
				"rm -f /usr/local/bin/spectra-agent",
				"rm -rf /usr/local/etc/spectra",
			},
		},
		{
			name:     "freebsd",
			os:       "freebsd",
			wantType: "rc_d",
			wantContains: []string{
				"service spectra_agent stop",
				"sysrc -x spectra_agent_enable",
				"rm -f /usr/local/etc/rc.d/spectra_agent",
			},
		},
		{
			name:     "windows",
			os:       "windows",
			wantType: "windows_service",
			wantContains: []string{
				"sc.exe stop SpectraAgent",
				"sc.exe delete SpectraAgent",
				"Remove-Item",
				"-Recurse",
			},
		},
		{
			name:         "unknown platform falls back to manual",
			os:           "plan9",
			wantType:     "manual",
			wantContains: []string{"Stop and remove"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &platformInfo{OS: tt.os, Filename: "test-binary"}
			got := generateUninstallInstructions(p)

			if got.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", got.Type, tt.wantType)
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(got.Steps, want) {
					t.Errorf("Steps missing %q\nsteps:\n%s", want, got.Steps)
				}
			}
		})
	}
}

func TestInstallInstructionsContent(t *testing.T) {
	tests := []struct {
		name           string
		os             string
		wantContentHas string
	}{
		{"linux systemd unit", "linux", "[Unit]"},
		{"darwin plist", "darwin", "<?xml version"},
		{"freebsd rc.d script", "freebsd", "PROVIDE: spectra_agent"},
		{"windows has empty content", "windows", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &platformInfo{OS: tt.os, Filename: "test-binary"}
			got := generateInstallInstructions(p, "https://example.com", "test-token", "")

			if tt.wantContentHas != "" && !strings.Contains(got.Content, tt.wantContentHas) {
				t.Errorf("Content missing %q\ncontent:\n%s", tt.wantContentHas, got.Content)
			}
		})
	}
}

func TestGenerateInstallInstructionsWithCACert(t *testing.T) {
	const caCert = "-----BEGIN CERTIFICATE-----\ntest-cert\n-----END CERTIFICATE-----\n"

	tests := []struct {
		name         string
		os           string
		wantType     string
		wantContains []string
	}{
		{
			name:     "linux",
			os:       "linux",
			wantType: "systemd",
			wantContains: []string{
				"Install the CA certificate",
				"/etc/spectra/ca.crt",
				`"ca_cert": "/etc/spectra/ca.crt"`,
				caCert,
			},
		},
		{
			name:     "darwin",
			os:       "darwin",
			wantType: "launchd",
			wantContains: []string{
				"Install the CA certificate",
				"/usr/local/etc/spectra/ca.crt",
				`"ca_cert": "/usr/local/etc/spectra/ca.crt"`,
				caCert,
			},
		},
		{
			name:     "freebsd",
			os:       "freebsd",
			wantType: "rc_d",
			wantContains: []string{
				"Install the CA certificate",
				"/usr/local/etc/spectra/ca.crt",
				`"ca_cert": "/usr/local/etc/spectra/ca.crt"`,
				caCert,
			},
		},
		{
			name:     "windows",
			os:       "windows",
			wantType: "windows_service",
			wantContains: []string{
				"Install the CA certificate",
				`C:\Spectra\ca.crt`,
				`"ca_cert": "C:\Spectra\ca.crt"`,
				caCert,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &platformInfo{OS: tt.os, Filename: "test-binary"}
			got := generateInstallInstructions(p, "https://example.com", "test-token", caCert)

			if got.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", got.Type, tt.wantType)
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(got.Steps, want) {
					t.Errorf("Steps missing %q\nsteps:\n%s", want, got.Steps)
				}
			}
		})
	}
}

func TestGenerateInstallInstructionsWithoutCACert(t *testing.T) {
	tests := []struct {
		name string
		os   string
	}{
		{"linux", "linux"},
		{"darwin", "darwin"},
		{"freebsd", "freebsd"},
		{"windows", "windows"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &platformInfo{OS: tt.os, Filename: "test-binary"}
			got := generateInstallInstructions(p, "https://example.com", "test-token", "")

			unwanted := []string{
				"Install the CA certificate",
				`"ca_cert"`,
				"ca.crt",
			}

			for _, s := range unwanted {
				if strings.Contains(got.Steps, s) {
					t.Errorf("Steps unexpectedly contain %q\nsteps:\n%s", s, got.Steps)
				}
			}
		})
	}
}
