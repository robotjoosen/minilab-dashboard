package store

import "github.com/robotjoosen/minilab-dashboard/pkg/domain"

func (s *Store) SaveHostStats(h domain.HostStats) error {
	_, err := s.db.Exec(`
INSERT INTO host_stats (device_id, cpu_user, cpu_system, cpu_idle, mem_used, mem_free, mem_total, polled_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(device_id) DO UPDATE SET
	cpu_user = excluded.cpu_user, cpu_system = excluded.cpu_system, cpu_idle = excluded.cpu_idle,
	mem_used = excluded.mem_used, mem_free = excluded.mem_free, mem_total = excluded.mem_total,
	polled_at = excluded.polled_at
`, h.DeviceID, h.CPUUser, h.CPUSystem, h.CPUIdle, h.MemUsed, h.MemFree, h.MemTotal, h.PolledAt)

	return err
}

func (s *Store) LatestHostStats(deviceID int64) (domain.HostStats, error) {
	h := domain.HostStats{DeviceID: deviceID}
	err := s.db.QueryRow(`
SELECT cpu_user, cpu_system, cpu_idle, mem_used, mem_free, mem_total, polled_at
FROM host_stats WHERE device_id = ?
`, deviceID).Scan(&h.CPUUser, &h.CPUSystem, &h.CPUIdle, &h.MemUsed, &h.MemFree, &h.MemTotal, &h.PolledAt)

	return h, err
}
