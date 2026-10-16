// Package tlsconf builds the mutual-TLS configs for the dashboard's web server and its
// outbound agent client. One private CA signs everything: agents (serverAuth), the dashboard's
// agent-facing client certificate (clientAuth), the web UI's server certificate (serverAuth)
// and browser client certificates. Anything not signed by it is rejected in both directions.
package tlsconf

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
)

// pool reads the CA bundle that signs every certificate in the lab.
func pool(caFile string) (*x509.CertPool, error) {
	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read TLS_CA_FILE: %w", err)
	}
	p := x509.NewCertPool()
	if !p.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("TLS_CA_FILE contains no valid PEM certificate")
	}
	return p, nil
}

// load returns nil when certFile and keyFile are both empty (that side stays plain HTTP).
// Setting only some of the three files is an error, so a half-configured install can never
// silently fall back to plain HTTP.
func load(caFile, certFile, keyFile string) (*x509.CertPool, *tls.Certificate, error) {
	if certFile == "" && keyFile == "" {
		return nil, nil, nil
	}
	if caFile == "" || certFile == "" || keyFile == "" {
		return nil, nil, errors.New("TLS_CA_FILE and both the certificate and key file of that role must be set together")
	}
	p, err := pool(caFile)
	if err != nil {
		return nil, nil, err
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, nil, fmt.Errorf("load certificate/key: %w", err)
	}
	return p, &cert, nil
}

// Client builds the config for calls to agents: it presents the dashboard's client
// certificate (clientAuth) and only accepts agents whose certificate the CA signed.
// Returns nil, nil when no client certificate is configured (plain HTTP to agents).
func Client(caFile, certFile, keyFile string) (*tls.Config, error) {
	p, cert, err := load(caFile, certFile, keyFile)
	if err != nil || cert == nil {
		return nil, err
	}

	return &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{*cert},
		// Agents are found by IP over mDNS and DHCP addresses change, so the usual
		// hostname check cannot work. Skip only that check and verify the chain against
		// our private CA ourselves, in VerifyConnection below.
		InsecureSkipVerify: true,
		VerifyConnection: func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return errors.New("agent presented no certificate")
			}
			opts := x509.VerifyOptions{
				Roots:         p,
				Intermediates: x509.NewCertPool(),
				KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			}
			for _, c := range cs.PeerCertificates[1:] {
				opts.Intermediates.AddCert(c)
			}
			_, err := cs.PeerCertificates[0].Verify(opts)
			return err
		},
	}, nil
}

// Server builds the config for the web UI: it presents a serverAuth certificate and
// requires every browser to present a client certificate the CA signed.
// Returns nil, nil when no server certificate is configured (plain HTTP).
func Server(caFile, certFile, keyFile string) (*tls.Config, error) {
	p, cert, err := load(caFile, certFile, keyFile)
	if err != nil || cert == nil {
		return nil, err
	}

	return &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{*cert},
		ClientCAs:    p,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}, nil
}
