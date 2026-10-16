// minilab-dashboard/pkg/agentclient/client.go
package agentclient

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ServiceInfo and Capabilities intentionally use the same untagged field names as
// minilab-agent's domain.Service and its /capabilities response struct, so encoding/json's
// default case-insensitive matching decodes them correctly without any tags.
type ServiceInfo struct {
	Name    string
	Type    string
	Up      bool
	Version string
}

type Capabilities struct {
	Device   string
	Services []ServiceInfo
}

// maxBody caps what we read from an agent; anything on the LAN can advertise itself over
// mDNS, so a reply must never be able to exhaust the dashboard's memory.
const maxBody = 1 << 20

var httpClient = &http.Client{Timeout: 5 * time.Second}

// UseTLS makes every agent call present the dashboard's client certificate and only accept
// agents whose certificate the configured CA signed (mutual TLS).
func UseTLS(cfg *tls.Config) {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.TLSClientConfig = cfg
	httpClient.Transport = t
}

func FetchCapabilities(baseURL string) (Capabilities, error) {
	resp, err := httpClient.Get(strings.TrimRight(baseURL, "/") + "/capabilities")
	if err != nil {
		return Capabilities{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Capabilities{}, fmt.Errorf("unexpected status %d from %s/capabilities", resp.StatusCode, baseURL)
	}

	var caps Capabilities
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&caps); err != nil {
		return Capabilities{}, err
	}

	return caps, nil
}

func FetchMetrics(baseURL string) (string, error) {
	resp, err := httpClient.Get(strings.TrimRight(baseURL, "/") + "/metrics")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d from %s/metrics", resp.StatusCode, baseURL)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return "", err
	}
	if len(body) > maxBody {
		return "", errors.New("metrics response too large")
	}
	return string(body), nil
}
