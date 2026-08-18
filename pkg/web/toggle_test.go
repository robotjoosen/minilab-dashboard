package web_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/robotjoosen/minilab-dashboard/pkg/domain"
	"github.com/robotjoosen/minilab-dashboard/pkg/store"
	"github.com/robotjoosen/minilab-dashboard/pkg/web"
)

func TestToggleHandlerTogglesWatch(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	defer s.Close()

	deviceID, _ := s.UpsertDevice("rocket", "http://192.168.8.203:9100", "agent")
	_ = s.UpsertDiscoveredServices(deviceID, []domain.Service{{Name: "ollama", Type: "docker"}})
	services, _ := s.ListServicesForDevice(deviceID)

	handler := &web.ToggleHandler{Store: s}

	req := httptest.NewRequest(http.MethodPost, "/watch?service_id="+itoa(services[0].ID), nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rec.Code)
	}

	updated, _ := s.ListServicesForDevice(deviceID)
	if !updated[0].Watched {
		t.Fatalf("expected service to be watched after toggle, got %+v", updated[0])
	}
}

func TestToggleHandlerRejectsGet(t *testing.T) {
	handler := &web.ToggleHandler{}

	req := httptest.NewRequest(http.MethodGet, "/watch?service_id=1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}
