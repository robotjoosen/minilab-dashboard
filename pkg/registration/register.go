// minilab-dashboard/pkg/registration/register.go
package registration

import (
	"github.com/robotjoosen/minilab-dashboard/pkg/agentclient"
	"github.com/robotjoosen/minilab-dashboard/pkg/domain"
	"github.com/robotjoosen/minilab-dashboard/pkg/store"
)

// RegisterAgent fetches an agent's capabilities and upserts the device and its discovered
// services. The stored device name prefers caps.Device (the agent's actual hostname, as
// decoded from its /capabilities response) over the name parameter, since name is typically
// derived from the raw mDNS PTR target (a full FQDN like "rocket._minilab-agent._tcp.local.",
// not a plain hostname — see hashicorp/mdns' ServiceEntry.Name). name is used as a fallback
// when caps.Device is empty, so the function still degrades gracefully.
func RegisterAgent(s *store.Store, fetch func(baseURL string) (agentclient.Capabilities, error), name, address string) error {
	caps, err := fetch(address)
	if err != nil {
		return err
	}

	deviceName := name
	if caps.Device != "" {
		deviceName = caps.Device
	}

	deviceID, err := s.UpsertDevice(deviceName, address, "agent")
	if err != nil {
		return err
	}

	services := make([]domain.Service, 0, len(caps.Services))
	for _, svc := range caps.Services {
		services = append(services, domain.Service{Name: svc.Name, Type: svc.Type, Version: svc.Version})
	}

	return s.UpsertDiscoveredServices(deviceID, services)
}
