package store_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/robotjoosen/minilab-dashboard/pkg/domain"
	"github.com/robotjoosen/minilab-dashboard/pkg/store"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	t.Cleanup(func() { s.Close() })

	return s
}

func TestUpsertDeviceAndList(t *testing.T) {
	s := openTestStore(t)

	id1, err := s.UpsertDevice("rocket", "http://192.168.8.203:9100", "agent")
	if err != nil {
		t.Fatalf("UpsertDevice() error = %v", err)
	}

	// Re-registering the same device should update, not duplicate.
	id2, err := s.UpsertDevice("rocket", "http://192.168.8.203:9100", "agent")
	if err != nil {
		t.Fatalf("UpsertDevice() second call error = %v", err)
	}
	if id1 != id2 {
		t.Fatalf("expected same device id on re-upsert, got %d and %d", id1, id2)
	}

	devices, err := s.ListDevices()
	if err != nil {
		t.Fatalf("ListDevices() error = %v", err)
	}
	if len(devices) != 1 || devices[0].Name != "rocket" || devices[0].Status != "online" {
		t.Fatalf("unexpected devices: %+v", devices)
	}
}

func TestUpsertDiscoveredServicesAndToggleWatch(t *testing.T) {
	s := openTestStore(t)

	deviceID, _ := s.UpsertDevice("rocket", "http://192.168.8.203:9100", "agent")

	err := s.UpsertDiscoveredServices(deviceID, []domain.Service{
		{Name: "ollama", Type: "docker", Version: "0.4.2"},
		{Name: "nodered.service", Type: "systemd", Version: "2026-08-01T10:00:00Z"},
	})
	if err != nil {
		t.Fatalf("UpsertDiscoveredServices() error = %v", err)
	}

	services, err := s.ListServicesForDevice(deviceID)
	if err != nil {
		t.Fatalf("ListServicesForDevice() error = %v", err)
	}
	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d: %+v", len(services), services)
	}
	for _, svc := range services {
		if svc.Watched {
			t.Fatalf("expected newly discovered services to be unwatched: %+v", svc)
		}
	}

	watched, err := s.ToggleWatch(services[0].ID)
	if err != nil {
		t.Fatalf("ToggleWatch() error = %v", err)
	}
	if !watched {
		t.Fatal("expected ToggleWatch() to report true after toggling an unwatched service")
	}

	watchedServices, err := s.ListWatchedServicesForDevice(deviceID)
	if err != nil {
		t.Fatalf("ListWatchedServicesForDevice() error = %v", err)
	}
	if len(watchedServices) != 1 || watchedServices[0].ID != services[0].ID {
		t.Fatalf("unexpected watched services: %+v", watchedServices)
	}
}

func TestUpdateServiceState(t *testing.T) {
	s := openTestStore(t)

	deviceID, _ := s.UpsertDevice("rocket", "http://192.168.8.203:9100", "agent")
	_ = s.UpsertDiscoveredServices(deviceID, []domain.Service{{Name: "ollama", Type: "docker", Version: "0.4.2"}})

	services, _ := s.ListServicesForDevice(deviceID)
	svcID := services[0].ID

	now := time.Now().UTC().Truncate(time.Second)
	if err := s.UpdateServiceState(svcID, "up", "0.4.3", now); err != nil {
		t.Fatalf("UpdateServiceState() error = %v", err)
	}

	updated, _ := s.ListServicesForDevice(deviceID)
	if updated[0].LastState != "up" || updated[0].Version != "0.4.3" {
		t.Fatalf("unexpected updated service: %+v", updated[0])
	}
}

func TestRecordPollSuccessAndFailure(t *testing.T) {
	s := openTestStore(t)
	deviceID, _ := s.UpsertDevice("beanie", "http://192.168.8.204:9100", "agent")

	for i := 0; i < 2; i++ {
		unreachable, err := s.RecordPollFailure(deviceID, 3)
		if err != nil {
			t.Fatalf("RecordPollFailure() error = %v", err)
		}
		if unreachable {
			t.Fatalf("expected device to still be reachable after %d failures", i+1)
		}
	}

	unreachable, err := s.RecordPollFailure(deviceID, 3)
	if err != nil {
		t.Fatalf("RecordPollFailure() error = %v", err)
	}
	if !unreachable {
		t.Fatal("expected device to be marked unreachable after 3 consecutive failures")
	}

	devices, _ := s.ListDevices()
	if devices[0].Status != "unreachable" {
		t.Fatalf("expected status unreachable, got %q", devices[0].Status)
	}

	if err := s.RecordPollSuccess(deviceID); err != nil {
		t.Fatalf("RecordPollSuccess() error = %v", err)
	}

	devices, _ = s.ListDevices()
	if devices[0].Status != "online" {
		t.Fatalf("expected status online after a success, got %q", devices[0].Status)
	}
}

func TestSaveAndLatestHostStats(t *testing.T) {
	s := openTestStore(t)
	deviceID, _ := s.UpsertDevice("rocket", "http://192.168.8.203:9100", "agent")

	now := time.Now().UTC().Truncate(time.Second)
	want := domain.HostStats{
		DeviceID: deviceID, CPUUser: 12.4, CPUSystem: 3.1, CPUIdle: 84.5,
		MemUsed: 1893000000, MemFree: 500000000, MemTotal: 2393000000, PolledAt: now,
	}

	if err := s.SaveHostStats(want); err != nil {
		t.Fatalf("SaveHostStats() error = %v", err)
	}

	got, err := s.LatestHostStats(deviceID)
	if err != nil {
		t.Fatalf("LatestHostStats() error = %v", err)
	}
	if got.CPUUser != want.CPUUser || got.MemUsed != want.MemUsed {
		t.Fatalf("LatestHostStats() = %+v, want %+v", got, want)
	}

	// Saving again should update in place, not duplicate.
	want.CPUUser = 20.0
	if err := s.SaveHostStats(want); err != nil {
		t.Fatalf("SaveHostStats() second call error = %v", err)
	}
	got, _ = s.LatestHostStats(deviceID)
	if got.CPUUser != 20.0 {
		t.Fatalf("expected updated CPUUser 20.0, got %v", got.CPUUser)
	}
}
