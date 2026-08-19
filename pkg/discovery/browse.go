package discovery

import (
	"strings"
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
//
// hashicorp/mdns's client listens on the shared mDNS multicast socket and surfaces every
// PTR/SRV/A/AAAA record it observes during the query window -- not only ones answering this
// specific query (the library itself notes this is unimplemented: see its "TODO: Check that
// response corresponds to serviceAddr?"). On a LAN with any other mDNS traffic -- ESPHome
// devices, printers, unrelated service browses from other apps -- that means entries with no
// relation to serviceName would otherwise reach onEntry. matchesService filters those out.
func Browse(serviceName string, timeout time.Duration, onEntry func(Entry)) error {
	entriesCh := make(chan *mdns.ServiceEntry, 16)
	done := make(chan struct{})

	go func() {
		defer close(done)
		for entry := range entriesCh {
			if !matchesService(entry.Name, serviceName) {
				continue
			}
			addr := ""
			if entry.AddrV4 != nil {
				addr = entry.AddrV4.String()
			}
			onEntry(Entry{Name: entry.Name, Addr: addr, Port: entry.Port})
		}
	}()

	params := mdns.DefaultParams(serviceName)
	params.DisableIPv6 = true
	params.Entries = entriesCh
	params.Timeout = timeout

	err := mdns.Query(params)
	close(entriesCh)
	<-done

	return err
}

// matchesService reports whether entryName -- an mDNS instance name such as
// "rocket._minilab-agent._tcp.local." -- actually belongs to serviceName (e.g.
// "_minilab-agent._tcp"), rather than being unrelated mDNS traffic hashicorp/mdns picked up
// during the query window. Browse always queries the "local" domain (mdns.DefaultParams'
// default, left unchanged here), so the expected suffix is fixed accordingly.
func matchesService(entryName, serviceName string) bool {
	suffix := "." + strings.Trim(serviceName, ".") + ".local."
	return strings.HasSuffix(entryName, suffix)
}
