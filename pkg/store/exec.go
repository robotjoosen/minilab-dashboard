package store

import (
	"database/sql"
	"time"

	"github.com/robotjoosen/minilab-dashboard/pkg/domain"
)

// execer is satisfied by both *sql.DB and *sql.Tx, so each write exists once and can run
// standalone or as part of a larger transaction.
type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// ServiceUpdate is one watched service's new state from a poll.
type ServiceUpdate struct {
	ID             int64
	State, Version string
}

func saveHostStats(x execer, h domain.HostStats) error {
	_, err := x.Exec(`
INSERT INTO host_stats (device_id, cpu_user, cpu_system, cpu_idle, mem_used, mem_free, mem_total, polled_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(device_id) DO UPDATE SET
	cpu_user = excluded.cpu_user, cpu_system = excluded.cpu_system, cpu_idle = excluded.cpu_idle,
	mem_used = excluded.mem_used, mem_free = excluded.mem_free, mem_total = excluded.mem_total,
	polled_at = excluded.polled_at
`, h.DeviceID, h.CPUUser, h.CPUSystem, h.CPUIdle, h.MemUsed, h.MemFree, h.MemTotal, h.PolledAt)
	return err
}

// replaceHostTemperatures makes the stored sensors exactly match temperatures, so a sensor
// that disappears from the agent disappears from the page too.
func replaceHostTemperatures(x execer, deviceID int64, temperatures []domain.Temperature) error {
	if _, err := x.Exec(`DELETE FROM host_temperatures WHERE device_id = ?`, deviceID); err != nil {
		return err
	}
	for _, t := range temperatures {
		if _, err := x.Exec(`INSERT INTO host_temperatures (device_id, sensor, celsius) VALUES (?, ?, ?)
ON CONFLICT(device_id, sensor) DO UPDATE SET celsius = excluded.celsius`, deviceID, t.Sensor, t.Celsius); err != nil {
			return err
		}
	}
	return nil
}

func updateServiceState(x execer, serviceID int64, state, version string, polledAt time.Time) error {
	_, err := x.Exec(`UPDATE services SET last_state = ?, version = ?, last_polled_at = ? WHERE id = ?`, state, version, polledAt, serviceID)
	return err
}

// SavePoll stores everything a successful poll produced in one transaction, so a crash
// halfway never leaves a device "online" with half-old data -- and it costs one disk sync
// instead of one per row.
func (s *Store) SavePoll(h domain.HostStats, temperatures []domain.Temperature, updates []ServiceUpdate) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE devices SET status = 'online', failed_polls = 0 WHERE id = ?`, h.DeviceID); err != nil {
		return err
	}
	if err := saveHostStats(tx, h); err != nil {
		return err
	}
	if err := replaceHostTemperatures(tx, h.DeviceID, temperatures); err != nil {
		return err
	}
	for _, u := range updates {
		if err := updateServiceState(tx, u.ID, u.State, u.Version, h.PolledAt); err != nil {
			return err
		}
	}

	return tx.Commit()
}
