package setup

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/nhdewitt/spectra/internal/fileutil"
)

const (
	tlsDir      = "/etc/spectra/tls"
	caCertFile  = "ca.crt"
	caKeyFile   = "ca.key"
	srvCertFile = "server.crt"
	srvKeyFile  = "server.key"

	caValidYears  = 10
	srvValidYears = 5
)

// TLSFiles holds paths to the generated TLS files.
type TLSFiles struct {
	CACert  string
	CAKey   string
	SrvCert string
	SrvKey  string
}

// GenerateTLS creates a self-signed CA and server certificate.
// sans should include any IPs or hostnames the server will be reached at.
// The auto-detected LAN IP, localhost, and 127.0.0.1 are always included.
func GenerateTLS(sans []string) (*TLSFiles, error) {
	if err := os.MkdirAll(tlsDir, 0700); err != nil {
		return nil, fmt.Errorf("create TLS dir: %w", err)
	}

	// Parse SANs into IPs and DNS names
	var ips []net.IP
	var dnsNames []string

	notBefore := time.Now().Add(-5 * time.Minute)

	// ALways include localhost/127.0.0.1/::1
	ips = append(ips, net.ParseIP("127.0.0.1"), net.ParseIP("::1"))
	dnsNames = append(dnsNames, "localhost")

	// Always include detected LAN IP
	if lanIP := detectLANIP(); lanIP != "127.0.0.1" {
		ips = append(ips, net.ParseIP(lanIP))
	}

	// Add caller-provided SANs
	for _, san := range sans {
		if ip := net.ParseIP(san); ip != nil {
			ips = append(ips, ip)
		} else {
			dnsNames = append(dnsNames, san)
		}
	}
	ips = deduplicate(ips, net.IP.String)
	dnsNames = deduplicate(dnsNames, func(s string) string { return s })

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate CA key: %w", err)
	}
	caSerial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generate CA serial: %w", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber: caSerial,
		Subject: pkix.Name{
			Organization: []string{"Spectra"},
			CommonName:   "Spectra CA",
		},
		NotBefore:             notBefore,
		NotAfter:              time.Now().AddDate(caValidYears, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("create CA cert: %w", err)
	}

	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		return nil, fmt.Errorf("create CA cert: %w", err)
	}

	srvKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate server key: %w", err)
	}

	srvSerial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generate server serial: %w", err)
	}

	srvTemplate := &x509.Certificate{
		SerialNumber: srvSerial,
		Subject: pkix.Name{
			Organization: []string{"Spectra"},
			CommonName:   "Spectra Server",
		},
		NotBefore:   notBefore,
		NotAfter:    time.Now().AddDate(srvValidYears, 0, 0),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: ips,
		DNSNames:    dnsNames,
	}

	srvCertDER, err := x509.CreateCertificate(rand.Reader, srvTemplate, caCert, &srvKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("create server cert: %w", err)
	}

	files := &TLSFiles{
		CACert:  filepath.Join(tlsDir, caCertFile),
		CAKey:   filepath.Join(tlsDir, caKeyFile),
		SrvCert: filepath.Join(tlsDir, srvCertFile),
		SrvKey:  filepath.Join(tlsDir, srvKeyFile),
	}

	if err := writePEM(files.CACert, "CERTIFICATE", caCertDER); err != nil {
		return nil, fmt.Errorf("write CA cert: %w", err)
	}
	if err := writeKeyPEM(files.CAKey, caKey); err != nil {
		return nil, fmt.Errorf("write CA key: %w", err)
	}
	if err := writePEM(files.SrvCert, "CERTIFICATE", srvCertDER); err != nil {
		return nil, fmt.Errorf("write server cert: %w", err)
	}
	if err := writeKeyPEM(files.SrvKey, srvKey); err != nil {
		return nil, fmt.Errorf("write server key: %w", err)
	}

	return files, nil
}

func writePEM(path, pemType string, data []byte) error {
	block := &pem.Block{Type: pemType, Bytes: data}
	return fileutil.WriteSecure(path, pem.EncodeToMemory(block))
}

func writeKeyPEM(path string, key *ecdsa.PrivateKey) error {
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	return writePEM(path, "EC PRIVATE KEY", der)
}

func deduplicate[T any](items []T, key func(T) string) []T {
	seen := make(map[string]bool)
	var result []T

	for _, item := range items {
		k := key(item)
		if !seen[k] {
			seen[k] = true
			result = append(result, item)
		}
	}

	return result
}
