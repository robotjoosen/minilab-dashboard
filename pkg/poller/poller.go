package poller

import (
	"time"

	"github.com/robotjoosen/minilab-dashboard/pkg/domain"
	"github.com/robotjoosen/minilab-dashboard/pkg/metricsparse"
	"github.com/robotjoosen/minilab-dashboard/pkg/store"
)

type MetricsFetcher func(baseURL string) (string, error)

// PollDevice fetches /metrics from device, updates host stats and watched services' state on
// success, and records a failure (marking the device unreachable once threshold consecutive
// failures accumulate) if the fetch or parse fails.
func PollDevice(s *store.Store, fetch MetricsFetcher, device domain.Device, threshold int, now time.Time) error {
	text, err := fetch(device.Address)
	if err != nil {
		if _, recErr := s.RecordPollFailure(device.ID, threshold); recErr != nil {
			return recErr
		}
		return err
	}

	parsed, err := metricsparse.Parse(text)
	if err != nil {
		if _, recErr := s.RecordPollFailure(device.ID, threshold); recErr != nil {
			return recErr
		}
		return err
	}

	if err := s.RecordPollSuccess(device.ID); err != nil {
		return err
	}

	if err := s.SaveHostStats(domain.HostStats{
		DeviceID:  device.ID,
		CPUUser:   parsed.Host.CPUUser,
		CPUSystem: parsed.Host.CPUSystem,
		CPUIdle:   parsed.Host.CPUIdle,
		MemUsed:   parsed.Host.MemUsed,
		MemFree:   parsed.Host.MemFree,
		MemTotal:  parsed.Host.MemTotal,
		PolledAt:  now,
	}); err != nil {
		return err
	}

	temperatures := make([]domain.Temperature, len(parsed.Temperatures))
	for i, t := range parsed.Temperatures {
		temperatures[i] = domain.Temperature{Sensor: t.Sensor, Celsius: t.Celsius}
	}
	if err := s.SaveHostTemperatures(device.ID, temperatures); err != nil {
		return err
	}

	watched, err := s.ListWatchedServicesForDevice(device.ID)
	if err != nil {
		return err
	}

	byName := make(map[string]metricsparse.ParsedService, len(parsed.Services))
	for _, svc := range parsed.Services {
		byName[svc.Name] = svc
	}

	for _, w := range watched {
		state := "down"
		version := w.Version

		if svc, ok := byName[w.Name]; ok {
			// Never let an empty parsed version clobber a version we already have
			// stored — metricsparse.Parse only fills Version when the payload
			// actually carried a minilab_service_info line for this service.
			if svc.Version != "" {
				version = svc.Version
			}
			if svc.Up {
				state = "up"
			}
		}

		if err := s.UpdateServiceState(w.ID, state, version, now); err != nil {
			return err
		}
	}

	return nil
}
