// minilab-dashboard/pkg/registration/register_test.go
package registration_test

import (
	"path/filepath"
	"testing"

	"github.com/robotjoosen/minilab-dashboard/pkg/agentclient"
	"github.com/robotjoosen/minilab-dashboard/pkg/registration"
	"github.com/robotjoosen/minilab-dashboard/pkg/store"
)

func TestRegisterAgentUpsertsDeviceAndServices(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	defer s.Close()

	fetch := func(baseURL string) (agentclient.Capabilities, error) {
		return agentclient.Capabilities{
			Device: "rocket",
			Services: []agentclient.ServiceInfo{
				{Name: "ollama", Type: "docker", Up: true, Version: "0.4.2"},
			},
		}, nil
	}

	if err := registration.RegisterAgent(s, fetch, "rocket", "http://192.168.8.203:9100"); err != nil {
		t.Fatalf("RegisterAgent() error = %v", err)
	}

	devices, err := s.ListDevices()
	if err != nil {
		t.Fatalf("ListDevices() error = %v", err)
	}
	if len(devices) != 1 || devices[0].Name != "rocket" || devices[0].Kind != "agent" {
		t.Fatalf("unexpected devices: %+v", devices)
	}

	services, err := s.ListServicesForDevice(devices[0].ID)
	if err != nil {
		t.Fatalf("ListServicesForDevice() error = %v", err)
	}
	if len(services) != 1 || services[0].Name != "ollama" || services[0].Watched {
		t.Fatalf("unexpected services: %+v", services)
	}
}

// TestRegisterAgentPrefersCapabilitiesDeviceNameOverRawMDNSName covers the real-world case
// where the name passed in is the raw mDNS PTR target (a full FQDN, e.g.
// "rocket._minilab-agent._tcp.local.") rather than the agent's actual hostname. The device
// should be stored under caps.Device, the hostname decoded from /capabilities, not the
// mDNS-derived name.
func TestRegisterAgentPrefersCapabilitiesDeviceNameOverRawMDNSName(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	defer s.Close()

	fetch := func(baseURL string) (agentclient.Capabilities, error) {
		return agentclient.Capabilities{
			Device: "rocket",
			Services: []agentclient.ServiceInfo{
				{Name: "ollama", Type: "docker", Up: true, Version: "0.4.2"},
			},
		}, nil
	}

	rawMDNSName := "rocket._minilab-agent._tcp.local."
	if err := registration.RegisterAgent(s, fetch, rawMDNSName, "http://192.168.8.203:9100"); err != nil {
		t.Fatalf("RegisterAgent() error = %v", err)
	}

	devices, err := s.ListDevices()
	if err != nil {
		t.Fatalf("ListDevices() error = %v", err)
	}
	if len(devices) != 1 || devices[0].Name != "rocket" {
		t.Fatalf("expected device stored under capabilities device name %q, got: %+v", "rocket", devices)
	}
}

// TestRegisterAgentFallsBackToNameWhenCapabilitiesDeviceEmpty covers graceful degradation:
// if /capabilities doesn't report a device hostname, the passed-in name parameter should
// still be used rather than storing an empty device name.
func TestRegisterAgentFallsBackToNameWhenCapabilitiesDeviceEmpty(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	defer s.Close()

	fetch := func(baseURL string) (agentclient.Capabilities, error) {
		return agentclient.Capabilities{
			Services: []agentclient.ServiceInfo{
				{Name: "ollama", Type: "docker", Up: true, Version: "0.4.2"},
			},
		}, nil
	}

	if err := registration.RegisterAgent(s, fetch, "rocket", "http://192.168.8.203:9100"); err != nil {
		t.Fatalf("RegisterAgent() error = %v", err)
	}

	devices, err := s.ListDevices()
	if err != nil {
		t.Fatalf("ListDevices() error = %v", err)
	}
	if len(devices) != 1 || devices[0].Name != "rocket" {
		t.Fatalf("expected fallback to name parameter %q, got: %+v", "rocket", devices)
	}
}
