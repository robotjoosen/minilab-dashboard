package web

import (
	"embed"
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
	Device   domain.Device
	Services []domain.Service
	Host     domain.HostStats
}

func (h *IndexHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	devices, err := h.Store.ListDevices()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	views := make([]indexViewDevice, 0, len(devices))
	for _, d := range devices {
		services, err := h.Store.ListServicesForDevice(d.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		host, _ := h.Store.LatestHostStats(d.ID) // zero-value HostStats if never polled yet

		views = append(views, indexViewDevice{Device: d, Services: services, Host: host})
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := indexTemplate.Execute(w, views); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
