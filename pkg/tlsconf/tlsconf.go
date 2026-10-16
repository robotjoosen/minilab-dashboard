// Package tlsconf builds the mutual-TLS configs shared by the dashboard's web server and its
// outbound agent client. One private CA signs the dashboard, every agent and every browser
// client certificate; anything not signed by it is rejected in both directions.
package tlsconf

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
)

// Load returns a client config (for calls to agents) and a server config (for the web UI).
// With all three files empty it returns nil, nil, nil: TLS is off. Setting only some of them
// is an error, so a half-configured install can never silently fall back to plain HTTP.
func Load(caFile, certFile, keyFile string) (client, server *tls.Config, err error) {
	if caFile == "" && certFile == "" && keyFile == "" {
		return nil, nil, nil
	}
	if caFile == "" || certFile == "" || keyFile == "" {
		return nil, nil, errors.New("TLS_CA_FILE, TLS_CERT_FILE and TLS_KEY_FILE must all be set together")
	}

	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		return nil, nil, fmt.Errorf("read TLS_CA_FILE: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, nil, errors.New("TLS_CA_FILE contains no valid PEM certificate")
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, nil, fmt.Errorf("load TLS_CERT_FILE/TLS_KEY_FILE: %w", err)
	}

	client = &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{cert},
		// Agents are found by IP over mDNS and DHCP addresses change, so the usual
		// hostname check cannot work. Skip only that check and verify the chain against
		// our private CA ourselves, in VerifyConnection below.
		InsecureSkipVerify: true,
		VerifyConnection: func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return errors.New("agent presented no certificate")
			}
			opts := x509.VerifyOptions{
				Roots:         pool,
				Intermediates: x509.NewCertPool(),
				KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			}
			for _, c := range cs.PeerCertificates[1:] {
				opts.Intermediates.AddCert(c)
			}
			_, err := cs.PeerCertificates[0].Verify(opts)
			return err
		},
	}

	server = &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{cert},
		ClientCAs:    pool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}

	return client, server, nil
}
