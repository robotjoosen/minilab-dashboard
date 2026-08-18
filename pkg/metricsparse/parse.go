package metricsparse

import (
	"fmt"
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

func Parse(text string) (ParsedMetrics, error) {
	var result ParsedMetrics

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
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
			result.Services = append(result.Services, ParsedService{
				Name:    labels["name"],
				Type:    labels["type"],
				Version: labels["version"],
				Up:      value == 1,
			})
		}
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
