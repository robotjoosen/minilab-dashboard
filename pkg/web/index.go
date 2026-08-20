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
	MemHuman       string
	MemPercent     int
	CPUUsedPercent int
}

type indexView struct {
	Devices  []indexViewDevice
	Total    int
	Online   int
	EditMode bool
}

func (h *IndexHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	devices, err := h.Store.ListDevices()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	editMode := r.URL.Query().Get("edit") == "1"

	view := indexView{Devices: make([]indexViewDevice, 0, len(devices)), Total: len(devices), EditMode: editMode}
	for _, d := range devices {
		var services []domain.Service
		var err error
		if editMode {
			services, err = h.Store.ListServicesForDevice(d.ID)
		} else {
			services, err = h.Store.ListWatchedServicesForDevice(d.ID)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		host, _ := h.Store.LatestHostStats(d.ID) // zero-value HostStats if never polled yet

		if d.Status == "online" {
			view.Online++
		}

		dv := indexViewDevice{
			Device:   d,
			Services: services,
			Host:     host,
			Polled:   !host.PolledAt.IsZero(),
			MemHuman: humanizeBytePair(host.MemUsed, host.MemTotal),
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

// humanizeBytePair renders used and total as a shared-unit pair ("1.4/7.7 GiB") rather than
// two independently-scaled sizes -- keeps the label short enough to stay on one line next to
// "MEM" at narrower card widths.
func humanizeBytePair(used, total uint64) string {
	const unit = 1024
	if total < unit {
		return fmt.Sprintf("%d/%d B", used, total)
	}

	div, exp := uint64(unit), 0
	for r := total / unit; r >= unit; r /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f/%.1f %ciB", float64(used)/float64(div), float64(total)/float64(div), "KMGTPE"[exp])
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
