// minilab-dashboard/cmd/app/setup.go
package main

import (
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/robotjoosen/minilab-dashboard/pkg/env"
)

const (
	defaultLogLevel             = "INFO"
	defaultHTTPListenAddr       = ":8080"
	defaultSQLitePath           = "./minilab-dashboard.db"
	defaultMDNSAgentServiceName = "_minilab-agent._tcp"
	defaultDiscoveryInterval    = time.Minute
	defaultPollInterval         = 15 * time.Second
	defaultUnreachableThreshold = 3
)

type Environment struct {
	LogLevel             slog.Level    `mapstructure:"LOG_LEVEL"`
	HTTPListenAddr       string        `mapstructure:"HTTP_LISTEN_ADDR"`
	SQLitePath           string        `mapstructure:"SQLITE_PATH"`
	MDNSAgentServiceName string        `mapstructure:"MDNS_AGENT_SERVICE_NAME"`
	DiscoveryInterval    time.Duration `mapstructure:"DISCOVERY_INTERVAL"`
	PollInterval         time.Duration `mapstructure:"POLL_INTERVAL"`
	UnreachableThreshold int           `mapstructure:"UNREACHABLE_THRESHOLD"`
	TLSCAFile            string        `mapstructure:"TLS_CA_FILE"`
	TLSClientCertFile    string        `mapstructure:"TLS_CLIENT_CERT_FILE"`
	TLSClientKeyFile     string        `mapstructure:"TLS_CLIENT_KEY_FILE"`
	TLSCertFile          string        `mapstructure:"TLS_CERT_FILE"`
	TLSKeyFile           string        `mapstructure:"TLS_KEY_FILE"`
}

func initLog(level slog.Level) {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
}

func loadEnv() Environment {
	e, err := env.Load[Environment](map[string]any{
		"LOG_LEVEL":               defaultLogLevel,
		"HTTP_LISTEN_ADDR":        defaultHTTPListenAddr,
		"SQLITE_PATH":             defaultSQLitePath,
		"MDNS_AGENT_SERVICE_NAME": defaultMDNSAgentServiceName,
		"DISCOVERY_INTERVAL":      defaultDiscoveryInterval,
		"POLL_INTERVAL":           defaultPollInterval,
		"UNREACHABLE_THRESHOLD":   defaultUnreachableThreshold,
		"TLS_CA_FILE":             "",
		"TLS_CLIENT_CERT_FILE":    "",
		"TLS_CLIENT_KEY_FILE":     "",
		"TLS_CERT_FILE":           "",
		"TLS_KEY_FILE":            "",
	})
	if err != nil {
		slog.Error("failed to load environment", "err", err.Error())
		os.Exit(1)
	}

	if err := e.validate(); err != nil {
		slog.Error("invalid configuration", "err", err.Error())
		os.Exit(1)
	}

	return e
}

// validate rejects values that would otherwise panic later (time.NewTicker panics on a
// non-positive interval) or silently misbehave.
func (e Environment) validate() error {
	if e.DiscoveryInterval <= 0 {
		return errors.New("DISCOVERY_INTERVAL must be greater than zero")
	}
	if e.PollInterval <= 0 {
		return errors.New("POLL_INTERVAL must be greater than zero")
	}
	if e.UnreachableThreshold < 1 {
		return errors.New("UNREACHABLE_THRESHOLD must be at least 1")
	}
	return nil
}
