package metricsparse_test

import (
	"testing"

	"github.com/robotjoosen/minilab-dashboard/pkg/metricsparse"
)

const sample = `minilab_host_cpu_percent{mode="user"} 12.4
minilab_host_cpu_percent{mode="system"} 3.1
minilab_host_cpu_percent{mode="idle"} 84.5
minilab_host_memory_bytes{state="used"} 1893000000
minilab_host_memory_bytes{state="free"} 500000000
minilab_host_memory_bytes{state="total"} 2393000000
minilab_service_up{name="nodered.service",type="systemd",version="2026-08-01T10:00:00Z"} 1
minilab_service_up{name="ollama",type="docker",version="0.4.2"} 1
minilab_service_up{name="rabbitmq",type="docker",version="3-management"} 0
`

func TestParse(t *testing.T) {
	got, err := metricsparse.Parse(sample)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if got.Host.CPUUser != 12.4 || got.Host.CPUSystem != 3.1 || got.Host.CPUIdle != 84.5 {
		t.Fatalf("unexpected host cpu: %+v", got.Host)
	}
	if got.Host.MemUsed != 1893000000 || got.Host.MemFree != 500000000 || got.Host.MemTotal != 2393000000 {
		t.Fatalf("unexpected host mem: %+v", got.Host)
	}

	if len(got.Services) != 3 {
		t.Fatalf("expected 3 services, got %d: %+v", len(got.Services), got.Services)
	}

	byName := map[string]metricsparse.ParsedService{}
	for _, s := range got.Services {
		byName[s.Name] = s
	}

	if s := byName["ollama"]; s.Type != "docker" || s.Version != "0.4.2" || !s.Up {
		t.Fatalf("unexpected ollama: %+v", s)
	}
	if s := byName["rabbitmq"]; s.Up {
		t.Fatalf("expected rabbitmq to be reported down: %+v", s)
	}
}

func TestParseMalformedLine(t *testing.T) {
	_, err := metricsparse.Parse("not_a_valid_metric_line {{{")
	if err == nil {
		t.Fatal("expected an error for malformed input")
	}
}

func TestParseEmptyInput(t *testing.T) {
	got, err := metricsparse.Parse("")
	if err != nil {
		t.Fatalf("Parse(\"\") error = %v", err)
	}
	if len(got.Services) != 0 {
		t.Fatalf("expected no services for empty input, got %+v", got.Services)
	}
}
