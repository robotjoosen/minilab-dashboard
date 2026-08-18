package store

import (
	"time"

	"github.com/robotjoosen/minilab-dashboard/pkg/domain"
)

func (s *Store) UpsertDiscoveredServices(deviceID int64, services []domain.Service) error {
	for _, svc := range services {
		_, err := s.db.Exec(`
INSERT INTO services (device_id, name, type, version)
VALUES (?, ?, ?, ?)
ON CONFLICT(device_id, name) DO UPDATE SET type = excluded.type, version = excluded.version
`, deviceID, svc.Name, svc.Type, svc.Version)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) ToggleWatch(serviceID int64) (bool, error) {
	if _, err := s.db.Exec(`UPDATE services SET watched = 1 - watched WHERE id = ?`, serviceID); err != nil {
		return false, err
	}

	var watched bool
	err := s.db.QueryRow(`SELECT watched FROM services WHERE id = ?`, serviceID).Scan(&watched)
	return watched, err
}

func (s *Store) ListServicesForDevice(deviceID int64) ([]domain.Service, error) {
	rows, err := s.db.Query(`
SELECT id, device_id, name, type, version, watched, last_state, last_polled_at
FROM services WHERE device_id = ? ORDER BY name
`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []domain.Service
	for rows.Next() {
		var svc domain.Service
		var lastPolled *time.Time
		if err := rows.Scan(&svc.ID, &svc.DeviceID, &svc.Name, &svc.Type, &svc.Version, &svc.Watched, &svc.LastState, &lastPolled); err != nil {
			return nil, err
		}
		if lastPolled != nil {
			svc.LastPolledAt = *lastPolled
		}
		services = append(services, svc)
	}

	return services, rows.Err()
}

func (s *Store) ListWatchedServicesForDevice(deviceID int64) ([]domain.Service, error) {
	services, err := s.ListServicesForDevice(deviceID)
	if err != nil {
		return nil, err
	}

	var watched []domain.Service
	for _, svc := range services {
		if svc.Watched {
			watched = append(watched, svc)
		}
	}

	return watched, nil
}

func (s *Store) UpdateServiceState(serviceID int64, state, version string, polledAt time.Time) error {
	_, err := s.db.Exec(`UPDATE services SET last_state = ?, version = ?, last_polled_at = ? WHERE id = ?`, state, version, polledAt, serviceID)
	return err
}
