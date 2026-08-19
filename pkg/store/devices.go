package store

import "github.com/robotjoosen/minilab-dashboard/pkg/domain"

func (s *Store) UpsertDevice(name, address, kind string) (int64, error) {
	_, err := s.db.Exec(`
INSERT INTO devices (name, address, kind, status, last_seen)
VALUES (?, ?, ?, 'online', CURRENT_TIMESTAMP)
ON CONFLICT(name) DO UPDATE SET address = excluded.address, status = 'online', last_seen = CURRENT_TIMESTAMP, failed_polls = 0
`, name, address, kind)
	if err != nil {
		return 0, err
	}

	var id int64
	err = s.db.QueryRow(`SELECT id FROM devices WHERE name = ?`, name).Scan(&id)
	return id, err
}

func (s *Store) ListDevices() ([]domain.Device, error) {
	rows, err := s.db.Query(`SELECT id, name, address, kind, status FROM devices ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []domain.Device
	for rows.Next() {
		var d domain.Device
		if err := rows.Scan(&d.ID, &d.Name, &d.Address, &d.Kind, &d.Status); err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}

	return devices, rows.Err()
}

func (s *Store) RecordPollSuccess(deviceID int64) error {
	_, err := s.db.Exec(`UPDATE devices SET status = 'online', failed_polls = 0 WHERE id = ?`, deviceID)
	return err
}

func (s *Store) RecordPollFailure(deviceID int64, threshold int) (unreachable bool, err error) {
	if _, err = s.db.Exec(`UPDATE devices SET failed_polls = failed_polls + 1 WHERE id = ?`, deviceID); err != nil {
		return false, err
	}

	var failedPolls int
	if err = s.db.QueryRow(`SELECT failed_polls FROM devices WHERE id = ?`, deviceID).Scan(&failedPolls); err != nil {
		return false, err
	}

	if failedPolls >= threshold {
		if _, err = s.db.Exec(`UPDATE devices SET status = 'unreachable' WHERE id = ?`, deviceID); err != nil {
			return false, err
		}
		return true, nil
	}

	return false, nil
}
