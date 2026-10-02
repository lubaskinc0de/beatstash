package navidrome

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Share is a Listen Link.
type Share struct {
	ID           string    `json:"id"`
	Description  string    `json:"description"`
	Downloadable bool      `json:"downloadable"`
	ExpiresAt    time.Time `json:"expiresAt"`
	ResourceIDs  string    `json:"resourceIds"`
	ResourceType string    `json:"resourceType"`
}

func (n *Server) Shares(t *testing.T, account Account) []Share {
	t.Helper()

	var shares []Share
	require.NoError(t, n.callAPIAs(account, http.MethodGet, "/api/share", nil, &shares))
	return shares
}
