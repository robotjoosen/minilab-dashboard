// minilab-dashboard/cmd/app/setup.go
package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/robotjoosen/minilab-dashboard/pkg/env"
)

const (
	defaultMode                   = "DEV"
	defaultLogLevel               = "INFO"
	defaultHTTPListenAddr         = ":8080"
	defaultSQLitePath             = "./minilab-dashboard.db"
	defaultMDNSAgentServiceName   = "_minilab-agent._tcp"
	defaultMDNSESPHomeServiceName = "_esphomelib._tcp"
	defaultDiscoveryInterval      = time.Minute
	defaultPollInterval           = 15 * time.Second
	defaultUnreachableThreshold   = 3
)

type Environment struct {
	Mode                   string        `mapstructure:"MODE"`
	LogLevel               slog.Level    `mapstructure:"LOG_LEVEL"`
	HTTPListenAddr         string        `mapstructure:"HTTP_LISTEN_ADDR"`
	SQLitePath             string        `mapstructure:"SQLITE_PATH"`
	MDNSAgentServiceName   string        `mapstructure:"MDNS_AGENT_SERVICE_NAME"`
	MDNSESPHomeServiceName string        `mapstructure:"MDNS_ESPHOME_SERVICE_NAME"`
	DiscoveryInterval      time.Duration `mapstructure:"DISCOVERY_INTERVAL"`
	PollInterval           time.Duration `mapstructure:"POLL_INTERVAL"`
	UnreachableThreshold   int           `mapstructure:"UNREACHABLE_THRESHOLD"`
}

func initLog(level slog.Level) {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
}

func loadEnv() Environment {
	e, err := env.Load[Environment](map[string]any{
		"MODE":                      defaultMode,
		"LOG_LEVEL":                 defaultLogLevel,
		"HTTP_LISTEN_ADDR":          defaultHTTPListenAddr,
		"SQLITE_PATH":               defaultSQLitePath,
		"MDNS_AGENT_SERVICE_NAME":   defaultMDNSAgentServiceName,
		"MDNS_ESPHOME_SERVICE_NAME": defaultMDNSESPHomeServiceName,
		"DISCOVERY_INTERVAL":        defaultDiscoveryInterval,
		"POLL_INTERVAL":             defaultPollInterval,
		"UNREACHABLE_THRESHOLD":     defaultUnreachableThreshold,
	})
	if err != nil {
		slog.Error("failed to load environment", "err", err.Error())
		os.Exit(1)
	}

	return e
}
