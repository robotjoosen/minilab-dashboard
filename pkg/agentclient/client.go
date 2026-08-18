// minilab-dashboard/pkg/agentclient/client.go
package agentclient

import (
	"encoding/json"
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

var httpClient = &http.Client{Timeout: 5 * time.Second}

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
	if err := json.NewDecoder(resp.Body).Decode(&caps); err != nil {
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

	body, err := io.ReadAll(resp.Body)
	return string(body), err
}
