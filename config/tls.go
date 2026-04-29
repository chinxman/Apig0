package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"strings"
	"time"
)

// TLSMode describes how the gateway should handle TLS.
type TLSMode int

const (
	TLSOff    TLSMode = iota // plain HTTP
	TLSAuto                  // auto-generate self-signed cert
	TLSCustom                // user-provided cert + key paths
)

// TLSConfig holds the resolved TLS settings for the gateway.
type TLSConfig struct {
	Mode     TLSMode
	CertFile string
	KeyFile  string
}

const (
	defaultCertDir  = "certs"
	defaultCertFile = "certs/apig0.pem"
	defaultKeyFile  = "certs/apig0-key.pem"
)

// LoadTLSConfig reads APIG0_TLS and returns the resolved TLS configuration.
//
//	"auto"                         → auto-generate self-signed cert
//	"/path/cert.pem,/path/key.pem" → use provided certs
//	"off" | "" (unset)             → plain HTTP
func LoadTLSConfig() TLSConfig {
	raw := strings.TrimSpace(os.Getenv("APIG0_TLS"))

	switch {
	case raw == "" || strings.EqualFold(raw, "off"):
		return TLSConfig{Mode: TLSOff}

	case strings.EqualFold(raw, "auto"):
		cert, key, err := ensureAutocert()
		if err != nil {
			log.Printf("[tls] auto-cert failed: %v — falling back to HTTP", err)
			return TLSConfig{Mode: TLSOff}
		}
		return TLSConfig{Mode: TLSAuto, CertFile: cert, KeyFile: key}

	default:
		// Expect "cert.pem,key.pem"
		parts := strings.SplitN(raw, ",", 2)
		if len(parts) != 2 {
			log.Printf("[tls] invalid APIG0_TLS format (expected cert,key paths) — falling back to HTTP")
			return TLSConfig{Mode: TLSOff}
		}
		cert := strings.TrimSpace(parts[0])
		key := strings.TrimSpace(parts[1])
		if _, err := os.Stat(cert); err != nil {
			log.Printf("[tls] cert file not found: %s — falling back to HTTP", cert)
			return TLSConfig{Mode: TLSOff}
		}
		if _, err := os.Stat(key); err != nil {
			log.Printf("[tls] key file not found: %s — falling back to HTTP", key)
			return TLSConfig{Mode: TLSOff}
		}
		return TLSConfig{Mode: TLSCustom, CertFile: cert, KeyFile: key}
	}
}

// ensureAutocert checks for existing auto-generated certs or creates new ones.
func ensureAutocert() (certPath, keyPath string, err error) {
	certPath = defaultCertFile
	keyPath = defaultKeyFile

	// Temporary setup should feel disposable and isolated, including its TLS material.
	if shouldRotateTemporaryAutocert() {
		log.Println("[tls] temporary setup detected; generating a fresh auto-cert")
	} else if certOK(certPath, keyPath) {
		return certPath, keyPath, nil
	}

	log.Println("[tls] generating self-signed certificate...")

	// Ensure certs directory exists
	if err := os.MkdirAll(defaultCertDir, 0700); err != nil {
		return "", "", fmt.Errorf("mkdir %s: %w", defaultCertDir, err)
	}

	// Generate ECDSA P-256 key
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("generate key: %w", err)
	}

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{Organization: []string{"Apig0 Gateway"}, CommonName: "apig0"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour), // 1 year
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},

		// Cover common access patterns for internal use
		DNSNames:    []string{"localhost", "apig0", "*.local"},
		IPAddresses: localIPs(),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		return "", "", fmt.Errorf("create certificate: %w", err)
	}

	// Write cert PEM
	certOut, err := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return "", "", fmt.Errorf("write cert: %w", err)
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	certOut.Close()

	// Write key PEM
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return "", "", fmt.Errorf("marshal key: %w", err)
	}
	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", "", fmt.Errorf("write key: %w", err)
	}
	pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	keyOut.Close()

	log.Printf("[tls] self-signed cert generated (valid 1 year): %s", certPath)
	return certPath, keyPath, nil
}

func shouldRotateTemporaryAutocert() bool {
	setup := CurrentSetupConfig()
	return setup.Mode == SetupModeTemporary && !setup.Persisted
}

// certOK returns true if the cert + key files exist and the certificate hasn't expired.
func certOK(certPath, keyPath string) bool {
	if _, err := os.Stat(keyPath); err != nil {
		return false
	}
	raw, err := os.ReadFile(certPath)
	if err != nil {
		return false
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return false
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false
	}
	// Consider it expired if less than 7 days remain
	return time.Until(cert.NotAfter) > 7*24*time.Hour
}

// localIPs returns all non-loopback IPv4 addresses on the machine,
// plus 127.0.0.1, for the certificate SAN list.
func localIPs() []net.IP {
	ips := []net.IP{net.ParseIP("127.0.0.1")}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ips
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil && !ipNet.IP.IsLoopback() {
			ips = append(ips, ipNet.IP)
		}
	}
	return ips
}
