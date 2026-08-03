package harness

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// selfSignedCert builds a self-signed leaf certificate for dnsName, the same
// shape Portainer's own certificate-generation produces: no CA, signed by its
// own key. Returned as PEM so the same bytes can be handed to both the test
// server (as a tls.Certificate) and ClientWithCA (as the trusted root).
func selfSignedCert(t *testing.T, dnsName string) (certPEM, keyPEM []byte) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{dnsName},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	keyBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	return certPEM, keyPEM
}

func newTLSServer(t *testing.T, certPEM, keyPEM []byte) *httptest.Server {
	t.Helper()
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("load key pair: %v", err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	server.StartTLS()
	return server
}

func TestClientWithCA_PinnedCertificate_VerifiesByServerName(t *testing.T) {
	t.Parallel()
	certPEM, keyPEM := selfSignedCert(t, "localhost")
	server := newTLSServer(t, certPEM, keyPEM)
	t.Cleanup(server.Close)

	client, err := ClientWithCA(certPEM, 5*time.Second)
	if err != nil {
		t.Fatalf("ClientWithCA() error = %v", err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	// server.URL is https://127.0.0.1:<port>: the connection is dialled by
	// IP, not by the "localhost" name the certificate was issued for. If
	// verification still succeeds, ServerName was set from the certificate
	// rather than left to default from the dial address.
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v, want a verified connection to succeed", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestClientWithCA_WrongCertificatePinned_FailsVerification(t *testing.T) {
	t.Parallel()
	// The server presents a genuine self-signed certificate for "localhost",
	// but the client is given a *different* one to trust. This is what
	// proves ClientWithCA actually verifies rather than accepting anything
	// with a valid-looking handshake, the same failure mode InsecureSkipVerify
	// would have hidden.
	serverCertPEM, serverKeyPEM := selfSignedCert(t, "localhost")
	otherCertPEM, _ := selfSignedCert(t, "localhost")
	server := newTLSServer(t, serverCertPEM, serverKeyPEM)
	t.Cleanup(server.Close)

	client, err := ClientWithCA(otherCertPEM, 5*time.Second)
	if err != nil {
		t.Fatalf("ClientWithCA() error = %v", err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := client.Do(req)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err == nil {
		t.Fatal("Do() error = nil, want a certificate verification failure")
	}
}

func TestClientWithCA_NoPEMBlock_ReturnsInformativeError(t *testing.T) {
	t.Parallel()
	_, err := ClientWithCA([]byte("not a certificate"), 5*time.Second)
	if err == nil {
		t.Fatal("ClientWithCA() error = nil, want an error on garbage input")
	}
	if !strings.Contains(err.Error(), "PEM block") {
		t.Errorf("error = %q, want it to name the missing PEM block", err)
	}
}

func TestClientWithCA_NoDNSNames_ReturnsInformativeError(t *testing.T) {
	t.Parallel()
	// A certificate with no DNS SAN at all: nothing to set ServerName to, and
	// silently falling back to some default would defeat the point of
	// deriving it from the certificate.
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	_, err = ClientWithCA(certPEM, 5*time.Second)
	if err == nil {
		t.Fatal("ClientWithCA() error = nil, want an error on a certificate with no DNS SAN")
	}
	if !strings.Contains(err.Error(), "DNS name") {
		t.Errorf("error = %q, want it to name the missing DNS SAN", err)
	}
}
