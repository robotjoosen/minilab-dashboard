package tlsconf_test

import (
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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/robotjoosen/minilab-dashboard/pkg/tlsconf"
)

type ca struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

func newCA(t *testing.T) ca {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test-ca"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, _ := x509.ParseCertificate(der)
	return ca{cert, key}
}

// writeLeaf writes a CA-signed cert (usable as client and server) and returns file paths.
// writeLeaf writes a CA-signed cert with the given usage and returns file paths.
func (c ca) writeLeaf(t *testing.T, dir, name string, usage x509.ExtKeyUsage) (certPath, keyPath string) {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: name},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		ExtKeyUsage: []x509.ExtKeyUsage{usage},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, c.cert, &key.PublicKey, c.key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, _ := x509.MarshalECPrivateKey(key)
	certPath = filepath.Join(dir, name+".crt")
	keyPath = filepath.Join(dir, name+".key")
	os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600)
	os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600)
	return
}

func (c ca) writeCert(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "ca.crt")
	os.WriteFile(p, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.cert.Raw}), 0o600)
	return p
}

func TestOffAndPartial(t *testing.T) {
	if c, err := tlsconf.Client("", "", ""); c != nil || err != nil {
		t.Fatalf("expected client TLS off, got %v %v", c, err)
	}
	if s, err := tlsconf.Server("ca.crt", "", ""); s != nil || err != nil {
		t.Fatalf("expected server TLS off, got %v %v", s, err)
	}
	if _, err := tlsconf.Client("", "a.crt", "a.key"); err == nil {
		t.Fatal("expected an error for a client cert without a CA")
	}
	if _, err := tlsconf.Server("ca.crt", "a.crt", ""); err == nil {
		t.Fatal("expected an error for a server cert without a key")
	}
}

// TestMutualTLS mirrors the minilab-agent scripts/gen-certs.sh certificates: the agent has a
// serverAuth-only certificate, the dashboard's agent-facing certificate is clientAuth-only.
func TestMutualTLS(t *testing.T) {
	dir := t.TempDir()
	good, evil := newCA(t), newCA(t)
	caPath := good.writeCert(t, dir)
	agentCert, agentKey := good.writeLeaf(t, dir, "agent", x509.ExtKeyUsageServerAuth)
	dashCert, dashKey := good.writeLeaf(t, dir, "dashboard", x509.ExtKeyUsageClientAuth)
	evilCert, evilKey := evil.writeLeaf(t, dir, "evil", x509.ExtKeyUsageClientAuth)

	// The "agent" requires a client certificate from our CA (same as minilab-agent).
	agentCfg, err := tlsconf.Server(caPath, agentCert, agentKey)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.TLS = agentCfg
	srv.StartTLS()
	defer srv.Close()

	get := func(cfg *tls.Config, url string) error {
		resp, err := (&http.Client{Transport: &http.Transport{TLSClientConfig: cfg}}).Get(url)
		if err == nil {
			resp.Body.Close()
		}
		return err
	}

	clientCfg, err := tlsconf.Client(caPath, dashCert, dashKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := get(clientCfg, srv.URL); err != nil {
		t.Fatalf("trusted client rejected: %v", err)
	}

	evilClient, _ := tlsconf.Client(caPath, evilCert, evilKey)
	if get(evilClient, srv.URL) == nil {
		t.Fatal("server accepted a client certificate from an untrusted CA")
	}

	evilCA := evil.writeCert(t, t.TempDir())
	evilSrvCert, evilSrvKey := evil.writeLeaf(t, dir, "evilsrv", x509.ExtKeyUsageServerAuth)
	evilServerCfg, _ := tlsconf.Server(evilCA, evilSrvCert, evilSrvKey)
	bad := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	bad.TLS = evilServerCfg
	bad.StartTLS()
	defer bad.Close()
	if get(clientCfg, bad.URL) == nil {
		t.Fatal("client accepted a server certificate from an untrusted CA")
	}
}
