package store

import "github.com/robotjoosen/minilab-dashboard/pkg/domain"

func (s *Store) SaveHostTemperatures(deviceID int64, temperatures []domain.Temperature) error {
	return replaceHostTemperatures(s.db, deviceID, temperatures)
}

func (s *Store) ListHostTemperatures(deviceID int64) ([]domain.Temperature, error) {
	rows, err := s.db.Query(`SELECT sensor, celsius FROM host_temperatures WHERE device_id = ? ORDER BY sensor`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var temperatures []domain.Temperature
	for rows.Next() {
		var t domain.Temperature
		if err := rows.Scan(&t.Sensor, &t.Celsius); err != nil {
			return nil, err
		}
		temperatures = append(temperatures, t)
	}

	return temperatures, rows.Err()
}
