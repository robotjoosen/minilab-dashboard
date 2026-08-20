package domain

import "time"

type Device struct {
	ID      int64
	Name    string
	Address string
	Kind    string // "agent" or "esphome"
	Status  string // "online" or "unreachable"
}

type Service struct {
	ID           int64
	DeviceID     int64
	Name         string
	Type         string // "systemd" or "docker"
	Version      string
	Watched      bool
	LastState    string // "up" or "down"
	LastPolledAt time.Time
}

type HostStats struct {
	DeviceID  int64
	CPUUser   float64
	CPUSystem float64
	CPUIdle   float64
	MemUsed   uint64
	MemFree   uint64
	MemTotal  uint64
	PolledAt  time.Time
}

type Temperature struct {
	Sensor  string
	Celsius float64
}
