package web

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"

	"github.com/robotjoosen/minilab-dashboard/pkg/domain"
	"github.com/robotjoosen/minilab-dashboard/pkg/store"
)

//go:embed templates/index.html
var templateFS embed.FS

var indexTemplate = template.Must(template.ParseFS(templateFS, "templates/index.html"))

type IndexHandler struct {
	Store *store.Store
}

type indexViewDevice struct {
	Device         domain.Device
	Services       []domain.Service
	Host           domain.HostStats
	Polled         bool // false until the poller has ever successfully reached this device
	MemUsedHuman   string
	MemTotalHuman  string
	MemPercent     int
	CPUUsedPercent int
}

type indexView struct {
	Devices []indexViewDevice
	Total   int
	Online  int
}

func (h *IndexHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	devices, err := h.Store.ListDevices()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	view := indexView{Devices: make([]indexViewDevice, 0, len(devices)), Total: len(devices)}
	for _, d := range devices {
		services, err := h.Store.ListServicesForDevice(d.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		host, _ := h.Store.LatestHostStats(d.ID) // zero-value HostStats if never polled yet

		if d.Status == "online" {
			view.Online++
		}

		dv := indexViewDevice{
			Device:        d,
			Services:      services,
			Host:          host,
			Polled:        !host.PolledAt.IsZero(),
			MemUsedHuman:  humanizeBytes(host.MemUsed),
			MemTotalHuman: humanizeBytes(host.MemTotal),
		}
		if host.MemTotal > 0 {
			dv.MemPercent = clampPercent(int(float64(host.MemUsed) / float64(host.MemTotal) * 100))
		}
		dv.CPUUsedPercent = clampPercent(int(100 - host.CPUIdle))

		view.Devices = append(view.Devices, dv)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := indexTemplate.Execute(w, view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// humanizeBytes renders n as a human-readable size ("1.8 GiB") rather than a raw byte count.
func humanizeBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}

	div, exp := uint64(unit), 0
	for r := n / unit; r >= unit; r /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// clampPercent keeps a percentage within [0, 100] -- guards against a momentarily
// inconsistent poll (e.g. mem_used briefly exceeding mem_total) producing an out-of-range bar.
func clampPercent(p int) int {
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}
