package tlsconf_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
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
func (c ca) writeLeaf(t *testing.T, dir, name string) (certPath, keyPath string) {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: name},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
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

func TestLoadOffAndPartial(t *testing.T) {
	if c, s, err := tlsconf.Load("", "", ""); c != nil || s != nil || err != nil {
		t.Fatalf("expected TLS off, got %v %v %v", c, s, err)
	}
	if _, _, err := tlsconf.Load("ca.crt", "", ""); err == nil {
		t.Fatal("expected an error for a partial TLS config")
	}
}

func TestMutualTLS(t *testing.T) {
	dir := t.TempDir()
	good, evil := newCA(t), newCA(t)
	caPath := good.writeCert(t, dir)
	srvCert, srvKey := good.writeLeaf(t, dir, "agent")
	cliCert, cliKey := good.writeLeaf(t, dir, "dashboard")
	evilCert, evilKey := evil.writeLeaf(t, dir, "evil")

	// The "agent": a server that requires a client certificate from our CA.
	_, serverCfg, err := tlsconf.Load(caPath, srvCert, srvKey)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.TLS = serverCfg
	srv.StartTLS()
	defer srv.Close()

	clientCfg, _, err := tlsconf.Load(caPath, cliCert, cliKey)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := (&http.Client{Transport: &http.Transport{TLSClientConfig: clientCfg}}).Get(srv.URL)
	if err != nil {
		t.Fatalf("trusted client rejected: %v", err)
	}
	resp.Body.Close()

	// A client certificate from another CA must be refused by the server.
	evilClient, _, _ := tlsconf.Load(caPath, evilCert, evilKey)
	if resp, err := (&http.Client{Transport: &http.Transport{TLSClientConfig: evilClient}}).Get(srv.URL); err == nil {
		resp.Body.Close()
		t.Fatal("server accepted a client certificate from an untrusted CA")
	}

	// A server certificate from another CA must be refused by our client.
	evilCA := evil.writeCert(t, t.TempDir())
	evilSrvCert, evilSrvKey := evil.writeLeaf(t, dir, "evilsrv")
	_, evilServerCfg, _ := tlsconf.Load(evilCA, evilSrvCert, evilSrvKey)
	bad := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	bad.TLS = evilServerCfg
	bad.StartTLS()
	defer bad.Close()
	if resp, err := (&http.Client{Transport: &http.Transport{TLSClientConfig: clientCfg}}).Get(bad.URL); err == nil {
		resp.Body.Close()
		t.Fatal("client accepted a server certificate from an untrusted CA")
	}
}
