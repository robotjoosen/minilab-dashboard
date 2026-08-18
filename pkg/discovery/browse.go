package discovery

import (
	"time"

	"github.com/hashicorp/mdns"
)

type Entry struct {
	Name string
	Addr string
	Port int
}

// Browse performs a single mDNS query for serviceName, invoking onEntry once per responder
// found before timeout elapses.
func Browse(serviceName string, timeout time.Duration, onEntry func(Entry)) error {
	entriesCh := make(chan *mdns.ServiceEntry, 16)
	done := make(chan struct{})

	go func() {
		defer close(done)
		for entry := range entriesCh {
			addr := ""
			if entry.AddrV4 != nil {
				addr = entry.AddrV4.String()
			}
			onEntry(Entry{Name: entry.Name, Addr: addr, Port: entry.Port})
		}
	}()

	params := mdns.DefaultParams(serviceName)
	params.Entries = entriesCh
	params.Timeout = timeout

	err := mdns.Query(params)
	close(entriesCh)
	<-done

	return err
}
