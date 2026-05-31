package setup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseOSRelease(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    map[string]string
	}{
		{
			name: "ubuntu",
			content: `NAME="Ubuntu"
ID=ubuntu
ID_LIKE=debian
VERSION_CODENAME=jammy
`,
			want: map[string]string{
				"NAME":             "ubuntu",
				"ID":               "ubuntu",
				"ID_LIKE":          "debian",
				"VERSION_CODENAME": "jammy",
			},
		},
		{
			name: "debian",
			content: `NAME="Debian GNU/Linux"
ID=debian
VERSION_CODENAME=bookworm
`,
			want: map[string]string{
				"NAME":             "debian gnu/linux",
				"ID":               "debian",
				"VERSION_CODENAME": "bookworm",
			},
		},
		{
			name: "rocky",
			content: `NAME="Rocky Linux"
ID="rocky"
ID_LIKE="rhel centos fedora"
VERSION_ID="9.3"
`,
			want: map[string]string{
				"NAME":       "rocky linux",
				"ID":         "rocky",
				"ID_LIKE":    "rhel centos fedora",
				"VERSION_ID": "9.3",
			},
		},
		{
			name: "amazon linux",
			content: `NAME="Amazon Linux"
ID="amzn"
ID_LIKE="centos rhel fedora"
VERSION_ID="2023"
`,
			want: map[string]string{
				"NAME":       "amazon linux",
				"ID":         "amzn",
				"ID_LIKE":    "centos rhel fedora",
				"VERSION_ID": "2023",
			},
		},
		{
			name:    "empty file",
			content: "",
			want:    map[string]string{},
		},
		{
			name: "comments and blank lines",
			content: `# this is a comment
ID=ubuntu

# another comment
VERSION_CODENAME=jammy
`,
			want: map[string]string{
				"ID":               "ubuntu",
				"VERSION_CODENAME": "jammy",
			},
		},
		{
			name: "single quoted values",
			content: `ID='debian'
VERSION_CODENAME='bookworm'
`,
			want: map[string]string{
				"ID":               "debian",
				"VERSION_CODENAME": "bookworm",
			},
		},
		{
			name: "mixed quoting",
			content: `ID="ubuntu"
ID_LIKE='debian'
VERSION_CODENAME=jammy
`,
			want: map[string]string{
				"ID":               "ubuntu",
				"ID_LIKE":          "debian",
				"VERSION_CODENAME": "jammy",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "os-release")
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			got, err := parseOSRelease(path)
			if err != nil {
				t.Fatalf("parseOSRelease: %v", err)
			}

			for k, wantV := range tt.want {
				if gotV, ok := got[k]; !ok {
					t.Errorf("missing key %s", k)
				} else if gotV != wantV {
					t.Errorf("key %s = %q, want %q", k, gotV, wantV)
				}
			}
		})
	}
}

func TestParseOSRelease_NotFound(t *testing.T) {
	_, err := parseOSRelease("/nonexistent/os-release")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestValidIdent(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"spectra", true},
		{"spectra_db", true},
		{"Spectra", true},
		{"db123", true},
		{"_leading", true},
		{"", false},
		{"123start", false},
		{"has space", false},
		{"has-dash", false},
		{"has.dot", false},
		{"semi;colon", false},
		{"user@host", false},
		{"drop'table", false},
		{`"quoted"`, false},
		{"a", true},
		{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false}, // 64 chars
		{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", true},   // 63 chars
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := validIdent(tt.input)
			if got != tt.want {
				t.Errorf("validIdent(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestEscapePassword(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"it's", "it''s"},
		{"a'b'c", "a''b''c"},
		{"no quotes", "no quotes"},
		{"'''", "''''''"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := escapePassword(tt.input)
			if got != tt.want {
				t.Errorf("escapePassword(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"8", true},
		{"9", true},
		{"10", true},
		{"123", true},
		{"", false},
		{"abc", false},
		{"8a", false},
		{"%rhel", false},
		{"8.1", false},
		{" 8", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isNumeric(tt.input)
			if got != tt.want {
				t.Errorf("isNumeric(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
