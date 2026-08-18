package web

import (
	"net/http"
	"strconv"

	"github.com/robotjoosen/minilab-dashboard/pkg/store"
)

type ToggleHandler struct {
	Store *store.Store
}

func (h *ToggleHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id, err := strconv.ParseInt(r.URL.Query().Get("service_id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid service_id", http.StatusBadRequest)
		return
	}

	if _, err := h.Store.ToggleWatch(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
