// minilab-dashboard/pkg/registration/register.go
package registration

import (
	"github.com/robotjoosen/minilab-dashboard/pkg/agentclient"
	"github.com/robotjoosen/minilab-dashboard/pkg/domain"
	"github.com/robotjoosen/minilab-dashboard/pkg/store"
)

func RegisterAgent(s *store.Store, fetch func(baseURL string) (agentclient.Capabilities, error), name, address string) error {
	caps, err := fetch(address)
	if err != nil {
		return err
	}

	deviceID, err := s.UpsertDevice(name, address, "agent")
	if err != nil {
		return err
	}

	services := make([]domain.Service, 0, len(caps.Services))
	for _, svc := range caps.Services {
		services = append(services, domain.Service{Name: svc.Name, Type: svc.Type, Version: svc.Version})
	}

	return s.UpsertDiscoveredServices(deviceID, services)
}
