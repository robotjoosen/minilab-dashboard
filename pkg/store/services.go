package store

import (
	"strings"
	"time"

	"github.com/robotjoosen/minilab-dashboard/pkg/domain"
)

// UpsertDiscoveredServices makes the device's stored services match the agent's current
// list: new ones are added, known ones updated, and ones the agent no longer reports removed.
// An empty version from the agent never overwrites one the poller already learned.
func (s *Store) UpsertDiscoveredServices(deviceID int64, services []domain.Service) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	args := []any{deviceID}
	placeholders := make([]string, 0, len(services))
	for _, svc := range services {
		if _, err := tx.Exec(`
INSERT INTO services (device_id, name, type, version)
VALUES (?, ?, ?, ?)
ON CONFLICT(device_id, name) DO UPDATE SET type = excluded.type,
	version = CASE WHEN excluded.version != '' THEN excluded.version ELSE services.version END
`, deviceID, svc.Name, svc.Type, svc.Version); err != nil {
			return err
		}
		args = append(args, svc.Name)
		placeholders = append(placeholders, "?")
	}

	// "NOT IN ()" is invalid SQL, so an empty list needs the plain delete.
	del := `DELETE FROM services WHERE device_id = ?`
	if len(placeholders) > 0 {
		del += ` AND name NOT IN (` + strings.Join(placeholders, ",") + `)`
	}
	if _, err := tx.Exec(del, args...); err != nil {
		return err
	}

	return tx.Commit()
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
	return updateServiceState(s.db, serviceID, state, version, polledAt)
}
