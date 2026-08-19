package metricsparse

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type ParsedHostStats struct {
	CPUUser   float64
	CPUSystem float64
	CPUIdle   float64
	MemUsed   uint64
	MemFree   uint64
	MemTotal  uint64
}

type ParsedService struct {
	Name    string
	Type    string
	Version string
	Up      bool
}

type ParsedMetrics struct {
	Host     ParsedHostStats
	Services []ParsedService
}

// Parse decodes the Prometheus-text-format payload served by minilab-agent's /metrics
// endpoint. Per-service state is split across two separate metric lines (see the agent's
// pkg/handler/metrics/format.go): minilab_service_up{name,type} carries up/down status with
// no version label, and a separate minilab_service_info{name,type,version} carries the
// version. The two may appear in either order (today the agent emits all the _up lines
// first, then all the _info lines), so entries are merged by service name as they're
// encountered and only flattened into the Services slice once the whole payload has been
// scanned.
func Parse(text string) (ParsedMetrics, error) {
	var result ParsedMetrics
	services := map[string]*ParsedService{}

	serviceFor := func(name string) *ParsedService {
		svc := services[name]
		if svc == nil {
			svc = &ParsedService{Name: name}
			services[name] = svc
		}
		return svc
	}

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		name, labels, value, err := parseLine(line)
		if err != nil {
			return ParsedMetrics{}, fmt.Errorf("parse line %q: %w", line, err)
		}

		switch name {
		case "minilab_host_cpu_percent":
			switch labels["mode"] {
			case "user":
				result.Host.CPUUser = value
			case "system":
				result.Host.CPUSystem = value
			case "idle":
				result.Host.CPUIdle = value
			}
		case "minilab_host_memory_bytes":
			switch labels["state"] {
			case "used":
				result.Host.MemUsed = uint64(value)
			case "free":
				result.Host.MemFree = uint64(value)
			case "total":
				result.Host.MemTotal = uint64(value)
			}
		case "minilab_service_up":
			svc := serviceFor(labels["name"])
			svc.Type = labels["type"]
			svc.Up = value == 1
			// Some callers (and older test fixtures) put a version label directly on
			// the up metric; keep honoring it, but never let an absent label here
			// clobber a version already learned from minilab_service_info.
			if v := labels["version"]; v != "" {
				svc.Version = v
			}
		case "minilab_service_info":
			svc := serviceFor(labels["name"])
			if t := labels["type"]; t != "" {
				svc.Type = t
			}
			if v := labels["version"]; v != "" {
				svc.Version = v
			}
		}
	}

	names := make([]string, 0, len(services))
	for name := range services {
		names = append(names, name)
	}
	sort.Strings(names)

	result.Services = make([]ParsedService, 0, len(names))
	for _, name := range names {
		result.Services = append(result.Services, *services[name])
	}

	return result, nil
}

func parseLine(line string) (name string, labels map[string]string, value float64, err error) {
	openBrace := strings.Index(line, "{")
	if openBrace == -1 {
		return parseLineWithoutLabels(line)
	}

	name = line[:openBrace]

	closeBrace := strings.LastIndex(line, "}")
	if closeBrace == -1 || closeBrace < openBrace {
		return "", nil, 0, fmt.Errorf("malformed labels")
	}

	labels = map[string]string{}
	for _, kv := range splitLabels(line[openBrace+1 : closeBrace]) {
		eq := strings.Index(kv, "=")
		if eq == -1 {
			return "", nil, 0, fmt.Errorf("malformed label pair %q", kv)
		}
		labels[kv[:eq]] = strings.Trim(kv[eq+1:], `"`)
	}

	value, err = strconv.ParseFloat(strings.TrimSpace(line[closeBrace+1:]), 64)
	if err != nil {
		return "", nil, 0, err
	}

	return name, labels, value, nil
}

func parseLineWithoutLabels(line string) (string, map[string]string, float64, error) {
	fields := strings.Fields(line)
	if len(fields) != 2 {
		return "", nil, 0, fmt.Errorf("expected 'name value', got %q", line)
	}

	value, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return "", nil, 0, err
	}

	return fields[0], map[string]string{}, value, nil
}

// splitLabels splits a comma-separated label list, ignoring commas inside quoted values.
func splitLabels(s string) []string {
	var parts []string
	inQuotes := false
	start := 0

	for i, c := range s {
		switch c {
		case '"':
			inQuotes = !inQuotes
		case ',':
			if !inQuotes {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])

	return parts
}
