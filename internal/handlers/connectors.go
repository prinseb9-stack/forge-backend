package handlers

import (
	"encoding/json"
	"net/http"

	"forge-backend/internal/connectors"
)

// ConnectorInfo is the JSON shape of a single connector in the
// /api/connectors response.
type ConnectorInfo struct {
	ID           string                  `json:"id"`
	Name         string                  `json:"name"`
	Icon         string                  `json:"icon"`
	Description  string                  `json:"description"`
	Category     string                  `json:"category"`
	Capabilities connectors.Capabilities `json:"capabilities"`
}

// ConnectorsResponse is the full /api/connectors response body.
type ConnectorsResponse struct {
	Success    bool            `json:"success"`
	Connectors []ConnectorInfo `json:"connectors"`
	Count      int             `json:"count"`
}

// ConnectorsHandler serves GET /api/connectors.
//
// This endpoint is public (no auth required). It exposes only public
// metadata about platform connectors — no secrets, no user data, no
// OAuth tokens, no per-user state.
type ConnectorsHandler struct{}

func NewConnectorsHandler() *ConnectorsHandler {
	return &ConnectorsHandler{}
}

func (h *ConnectorsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	registered := connectors.All()

	out := make([]ConnectorInfo, 0, len(registered))
	for _, c := range registered {
		out = append(out, ConnectorInfo{
			ID:           c.ID(),
			Name:         c.Name(),
			Icon:         c.Icon(),
			Description:  c.Description(),
			Category:     string(c.Category()),
			Capabilities: c.Capabilities(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ConnectorsResponse{
		Success:    true,
		Connectors: out,
		Count:      len(out),
	})
}
