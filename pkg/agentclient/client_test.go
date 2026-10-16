// minilab-dashboard/pkg/agentclient/client_test.go
package agentclient_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/robotjoosen/minilab-dashboard/pkg/agentclient"
)

func TestFetchCapabilities(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/capabilities" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"Device":"rocket","Services":[{"Name":"ollama","Type":"docker","Up":true,"Version":"0.4.2"}]}`)
	}))
	defer srv.Close()

	caps, err := agentclient.FetchCapabilities(srv.URL)
	if err != nil {
		t.Fatalf("FetchCapabilities() error = %v", err)
	}

	if caps.Device != "rocket" || len(caps.Services) != 1 || caps.Services[0].Name != "ollama" {
		t.Fatalf("unexpected capabilities: %+v", caps)
	}
}

func TestFetchCapabilitiesNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := agentclient.FetchCapabilities(srv.URL)
	if err == nil {
		t.Fatal("expected an error for a non-200 response")
	}
}

func TestFetchMetrics(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "minilab_host_cpu_percent{mode=\"user\"} 12.4\n")
	}))
	defer srv.Close()

	body, err := agentclient.FetchMetrics(srv.URL)
	if err != nil {
		t.Fatalf("FetchMetrics() error = %v", err)
	}
	if body != "minilab_host_cpu_percent{mode=\"user\"} 12.4\n" {
		t.Fatalf("unexpected metrics body: %q", body)
	}
}

func TestFetchMetricsRejectsOversizedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(make([]byte, 2<<20))
	}))
	defer srv.Close()

	if _, err := agentclient.FetchMetrics(srv.URL); err == nil {
		t.Fatal("expected an error for a body over the size limit")
	}
}
