// minilab-dashboard/cmd/app/main.go
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/robotjoosen/minilab-dashboard/pkg/agentclient"
	"github.com/robotjoosen/minilab-dashboard/pkg/discovery"
	"github.com/robotjoosen/minilab-dashboard/pkg/poller"
	"github.com/robotjoosen/minilab-dashboard/pkg/registration"
	"github.com/robotjoosen/minilab-dashboard/pkg/store"
	"github.com/robotjoosen/minilab-dashboard/pkg/tlsconf"
	"github.com/robotjoosen/minilab-dashboard/pkg/web"
)

func main() {
	e := loadEnv()
	initLog(e.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	clientTLS, serverTLS, err := tlsconf.Load(e.TLSCAFile, e.TLSCertFile, e.TLSKeyFile)
	if err != nil {
		slog.Error("failed to load TLS config", slog.String("error", err.Error()))
		os.Exit(1)
	}
	scheme := "http"
	if clientTLS != nil {
		agentclient.UseTLS(clientTLS)
		scheme = "https"
	}

	s, err := store.Open(e.SQLitePath)
	if err != nil {
		slog.Error("failed to open store", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer s.Close()

	go runDiscoveryLoop(ctx, s, scheme, e.MDNSAgentServiceName, e.DiscoveryInterval)
	go runPollLoop(ctx, s, e.PollInterval, e.UnreachableThreshold)

	mux := http.NewServeMux()
	mux.Handle("/", &web.IndexHandler{Store: s})
	mux.Handle("/watch", &web.ToggleHandler{Store: s})

	httpServer := &http.Server{
		Addr:              e.HTTPListenAddr,
		Handler:           mux,
		TLSConfig:         serverTLS,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	httpErr := make(chan error, 1)
	go func() {
		var err error
		if serverTLS != nil {
			err = httpServer.ListenAndServeTLS("", "") // cert comes from TLSConfig
		} else {
			err = httpServer.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
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
func runDiscoveryLoop(ctx context.Context, s *store.Store, scheme, serviceName string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	discoverOnce := func() {
		err := discovery.Browse(serviceName, 5*time.Second, func(entry discovery.Entry) {
			if entry.Addr == "" {
				slog.Warn("skipping agent advertised without an IPv4 address", slog.String("name", entry.Name))
				return
			}
			address := fmt.Sprintf("%s://%s", scheme, net.JoinHostPort(entry.Addr, strconv.Itoa(entry.Port)))
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

		// Poll in parallel: a dead device costs a full request timeout, and doing them one
		// after another would let a few of them delay every healthy device's update.
		var wg sync.WaitGroup
		for _, d := range devices {
			if d.Kind != "agent" {
				continue
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := poller.PollDevice(s, agentclient.FetchMetrics, d, threshold, time.Now()); err != nil {
					slog.Error("poll failed", slog.String("device", d.Name), slog.String("error", err.Error()))
				}
			}()
		}
		wg.Wait()
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
