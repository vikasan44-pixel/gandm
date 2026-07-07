package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"gandm/internal/httpx"
)

// ToolAccessCheck is minimal scaffolding to exercise the RequireTool
// middleware end-to-end. Stage 1 has no real business endpoint to gate yet
// (that comes with cargo requests etc. in later stages) — this route exists
// purely so "access is granted only via tool possession" is provable today.
// By the time this handler runs, RequireTool has already let the request
// through, so a 200 here just confirms the gate opened.
func ToolAccessCheck(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]string{
		"tool_key": chi.URLParam(r, "key"),
		"access":   "granted",
	})
}
