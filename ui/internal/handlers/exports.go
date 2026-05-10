package handlers

import (
	"net/http"

	"github.com/GustavoCaso/folio/ui/internal/logging"
	"github.com/GustavoCaso/folio/ui/internal/templates"
)

func (h *Handlers) ListExports(w http.ResponseWriter, r *http.Request) {
	log := logging.LoggerFrom(r.Context())

	records, err := h.store.ListAllExports(r.Context())
	if err != nil {
		log.Error("list exports failed", logging.Err(err))
		templates.ErrorPage("Failed to load export history").Render(r.Context(), w) //nolint:errcheck
		return
	}

	var initErrs []templates.BackendInitError
	for _, e := range h.backendErrors {
		initErrs = append(initErrs, templates.BackendInitError{BackendName: e.BackendName, Err: e.Err})
	}
	if err := templates.Exports(records, initErrs).Render(r.Context(), w); err != nil {
		log.Error("render exports page failed", logging.Err(err))
	}
}
