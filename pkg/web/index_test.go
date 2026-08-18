package web_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robotjoosen/minilab-dashboard/pkg/domain"
	"github.com/robotjoosen/minilab-dashboard/pkg/store"
	"github.com/robotjoosen/minilab-dashboard/pkg/web"
)

func TestIndexHandlerRendersDevicesAndServices(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	defer s.Close()

	deviceID, _ := s.UpsertDevice("rocket", "http://192.168.8.203:9100", "agent")
	_ = s.UpsertDiscoveredServices(deviceID, []domain.Service{{Name: "ollama", Type: "docker", Version: "0.4.2"}})

	handler := &web.IndexHandler{Store: s}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "rocket") || !strings.Contains(body, "ollama") {
		t.Fatalf("expected body to contain device and service names, got:\n%s", body)
	}
}
