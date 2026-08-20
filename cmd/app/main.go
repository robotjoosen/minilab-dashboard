// minilab-dashboard/cmd/app/main.go
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/robotjoosen/minilab-dashboard/pkg/agentclient"
	"github.com/robotjoosen/minilab-dashboard/pkg/discovery"
	"github.com/robotjoosen/minilab-dashboard/pkg/poller"
	"github.com/robotjoosen/minilab-dashboard/pkg/registration"
	"github.com/robotjoosen/minilab-dashboard/pkg/store"
	"github.com/robotjoosen/minilab-dashboard/pkg/web"
)

func main() {
	e := loadEnv()
	initLog(e.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	s, err := store.Open(e.SQLitePath)
	if err != nil {
		slog.Error("failed to open store", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer s.Close()

	go runDiscoveryLoop(ctx, s, e.MDNSAgentServiceName, e.DiscoveryInterval)
	go runPollLoop(ctx, s, e.PollInterval, e.UnreachableThreshold)

	mux := http.NewServeMux()
	mux.Handle("/", &web.IndexHandler{Store: s})
	mux.Handle("/watch", &web.ToggleHandler{Store: s})

	httpServer := &http.Server{Addr: e.HTTPListenAddr, Handler: mux}
	httpErr := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			httpErr <- err
			return
		}
		httpErr <- nil
	}()

	slog.Info("minilab-dashboard started", slog.String("http_addr", e.HTTPListenAddr))

	// A failed bind (e.g. the configured port already in use) must be fatal -- the
	// web UI is the whole point of this process, and discovery/polling running on
	// in the background otherwise leaves systemd reporting "active" indefinitely
	// for a dashboard nobody can reach.
	select {
	case <-ctx.Done():
	case err := <-httpErr:
		if err != nil {
			slog.Error("http server failed to start", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
}

// runDiscoveryLoop periodically browses for agent advertisements and registers any not
// already known to the store.
func runDiscoveryLoop(ctx context.Context, s *store.Store, serviceName string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	discoverOnce := func() {
		err := discovery.Browse(serviceName, 5*time.Second, func(entry discovery.Entry) {
			address := "http://" + entry.Addr + portSuffix(entry.Port)
			if err := registration.RegisterAgent(s, agentclient.FetchCapabilities, entry.Name, address); err != nil {
				slog.Error("failed to register agent", slog.String("name", entry.Name), slog.String("error", err.Error()))
			}
		})
		if err != nil {
			slog.Error("mDNS browse failed", slog.String("error", err.Error()))
		}
	}

	discoverOnce()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			discoverOnce()
		}
	}
}

// runPollLoop periodically polls every known agent device for updated metrics.
func runPollLoop(ctx context.Context, s *store.Store, interval time.Duration, threshold int) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	pollOnce := func() {
		devices, err := s.ListDevices()
		if err != nil {
			slog.Error("failed to list devices for polling", slog.String("error", err.Error()))
			return
		}

		for _, d := range devices {
			if d.Kind != "agent" {
				continue
			}
			if err := poller.PollDevice(s, agentclient.FetchMetrics, d, threshold, time.Now()); err != nil {
				slog.Error("poll failed", slog.String("device", d.Name), slog.String("error", err.Error()))
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pollOnce()
		}
	}
}

func portSuffix(port int) string {
	if port == 0 {
		return ""
	}
	return ":" + itoa(port)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
