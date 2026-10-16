package store

import "github.com/robotjoosen/minilab-dashboard/pkg/domain"

func (s *Store) SaveHostStats(h domain.HostStats) error { return saveHostStats(s.db, h) }

func (s *Store) LatestHostStats(deviceID int64) (domain.HostStats, error) {
	h := domain.HostStats{DeviceID: deviceID}
	err := s.db.QueryRow(`
SELECT cpu_user, cpu_system, cpu_idle, mem_used, mem_free, mem_total, polled_at
FROM host_stats WHERE device_id = ?
`, deviceID).Scan(&h.CPUUser, &h.CPUSystem, &h.CPUIdle, &h.MemUsed, &h.MemFree, &h.MemTotal, &h.PolledAt)

	return h, err
}
