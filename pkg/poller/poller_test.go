package poller_test

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/robotjoosen/minilab-dashboard/pkg/domain"
	"github.com/robotjoosen/minilab-dashboard/pkg/poller"
	"github.com/robotjoosen/minilab-dashboard/pkg/store"
)

const sampleMetrics = `minilab_host_cpu_percent{mode="user"} 12.4
minilab_host_cpu_percent{mode="system"} 3.1
minilab_host_cpu_percent{mode="idle"} 84.5
minilab_host_memory_bytes{state="used"} 1893000000
minilab_host_memory_bytes{state="free"} 500000000
minilab_host_memory_bytes{state="total"} 2393000000
minilab_service_up{name="ollama",type="docker",version="0.4.3"} 1
`

func setup(t *testing.T) (*store.Store, domain.Device) {
	t.Helper()

	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	t.Cleanup(func() { s.Close() })

	deviceID, _ := s.UpsertDevice("rocket", "http://192.168.8.203:9100", "agent")
	_ = s.UpsertDiscoveredServices(deviceID, []domain.Service{{Name: "ollama", Type: "docker", Version: "0.4.2"}})

	services, _ := s.ListServicesForDevice(deviceID)
	if _, err := s.ToggleWatch(services[0].ID); err != nil {
		t.Fatalf("ToggleWatch() error = %v", err)
	}

	devices, _ := s.ListDevices()
	return s, devices[0]
}

func TestPollDeviceSuccessUpdatesHostStatsAndWatchedServices(t *testing.T) {
	s, device := setup(t)

	fetch := func(baseURL string) (string, error) { return sampleMetrics, nil }

	if err := poller.PollDevice(s, fetch, device, 3, time.Now()); err != nil {
		t.Fatalf("PollDevice() error = %v", err)
	}

	host, err := s.LatestHostStats(device.ID)
	if err != nil {
		t.Fatalf("LatestHostStats() error = %v", err)
	}
	if host.CPUUser != 12.4 {
		t.Fatalf("unexpected host stats: %+v", host)
	}

	watched, _ := s.ListWatchedServicesForDevice(device.ID)
	if len(watched) != 1 || watched[0].LastState != "up" || watched[0].Version != "0.4.3" {
		t.Fatalf("unexpected watched service state: %+v", watched)
	}

	devices, _ := s.ListDevices()
	if devices[0].Status != "online" {
		t.Fatalf("expected device to remain online, got %q", devices[0].Status)
	}
}

// sampleMetricsNoVersion mirrors a poll where the agent reports a service as up but the
// payload carries no minilab_service_info line for it (e.g. an info line got dropped, or an
// older agent build), so metricsparse.Parse leaves ParsedService.Version empty.
const sampleMetricsNoVersion = `minilab_host_cpu_percent{mode="user"} 12.4
minilab_host_cpu_percent{mode="system"} 3.1
minilab_host_cpu_percent{mode="idle"} 84.5
minilab_host_memory_bytes{state="used"} 1893000000
minilab_host_memory_bytes{state="free"} 500000000
minilab_host_memory_bytes{state="total"} 2393000000
minilab_service_up{name="ollama",type="docker"} 1
`

func TestPollDeviceDoesNotClobberStoredVersionWithEmptyParsedVersion(t *testing.T) {
	s, device := setup(t)

	// Give the watched service a known non-empty version, as registration would have
	// done from /capabilities.
	watched, _ := s.ListWatchedServicesForDevice(device.ID)
	if err := s.UpdateServiceState(watched[0].ID, "up", "0.4.2", time.Now()); err != nil {
		t.Fatalf("UpdateServiceState() error = %v", err)
	}

	fetch := func(baseURL string) (string, error) { return sampleMetricsNoVersion, nil }

	if err := poller.PollDevice(s, fetch, device, 3, time.Now()); err != nil {
		t.Fatalf("PollDevice() error = %v", err)
	}

	watched, _ = s.ListWatchedServicesForDevice(device.ID)
	if len(watched) != 1 || watched[0].LastState != "up" || watched[0].Version != "0.4.2" {
		t.Fatalf("expected version to remain 0.4.2, got %+v", watched)
	}
}

func TestPollDeviceStaysOnlineBelowFailureThreshold(t *testing.T) {
	s, device := setup(t)

	fetch := func(baseURL string) (string, error) { return "", errors.New("connection refused") }

	err := poller.PollDevice(s, fetch, device, 3, time.Now())
	if err == nil || err.Error() != "connection refused" {
		t.Fatalf("PollDevice() error = %v, want %q", err, "connection refused")
	}

	devices, _ := s.ListDevices()
	if devices[0].Status != "online" {
		t.Fatalf("expected device to remain online after a single failure, got %q", devices[0].Status)
	}
}

func TestPollDeviceMarksUnreachableAtThreshold(t *testing.T) {
	s, device := setup(t)

	fetch := func(baseURL string) (string, error) { return "", errors.New("connection refused") }

	for i := 0; i < 3; i++ {
		err := poller.PollDevice(s, fetch, device, 3, time.Now())
		if err == nil || err.Error() != "connection refused" {
			t.Fatalf("PollDevice() error = %v, want %q", err, "connection refused")
		}
	}

	devices, _ := s.ListDevices()
	if devices[0].Status != "unreachable" {
		t.Fatalf("expected device unreachable after 3 failures, got %q", devices[0].Status)
	}
}
