package web

import (
	"database/sql"
	"errors"
	"log/slog"
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

	// Browsers label every request with where it came from; refuse a form posted by
	// another site. (With mTLS on, such a site cannot supply the client cert either.)
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	id, err := strconv.ParseInt(r.URL.Query().Get("service_id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid service_id", http.StatusBadRequest)
		return
	}

	if _, err := h.Store.ToggleWatch(id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "unknown service", http.StatusNotFound)
			return
		}
		slog.Error("toggle watch failed", slog.String("error", err.Error()))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	target := "/"
	if r.URL.Query().Get("edit") == "1" {
		target = "/?edit=1"
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}
