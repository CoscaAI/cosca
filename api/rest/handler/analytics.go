package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/CoscaAI/cosca/internal/audit"
)

// AnalyticsHandler handles analytics-related API endpoints.
type AnalyticsHandler struct {
	auditStore *audit.Store
}

// NewAnalyticsHandler creates a new AnalyticsHandler.
func NewAnalyticsHandler(auditStore *audit.Store) *AnalyticsHandler {
	return &AnalyticsHandler{auditStore: auditStore}
}

// AnalyticsData represents the full analytics response.
type AnalyticsData struct {
	Summary           AnalyticsSummary   `json:"summary"`
	TopQueries        []TopQuery         `json:"topQueries"`
	ZeroResultQueries []ZeroResultQuery  `json:"zeroResultQueries"`
	Trends            []SearchTrendPoint `json:"trends"`
}

// AnalyticsSummary holds aggregate analytics numbers.
type AnalyticsSummary struct {
	TotalSearches      int     `json:"totalSearches"`
	AvgResultsPerQuery float64 `json:"avgResultsPerQuery"`
	ZeroResultRate     float64 `json:"zeroResultRate"`
	TopSearcher        string  `json:"topSearcher"`
}

// TopQuery represents a frequently searched term.
type TopQuery struct {
	Query string `json:"query"`
	Count int    `json:"count"`
}

// ZeroResultQuery represents a search that returned no results.
type ZeroResultQuery struct {
	Query        string `json:"query"`
	Count        int    `json:"count"`
	LastSearched string `json:"lastSearched"`
}

// SearchTrendPoint represents a single data point in the search trends chart.
type SearchTrendPoint struct {
	Date       string  `json:"date"`
	Count      int     `json:"count"`
	AvgResults float64 `json:"avgResults"`
}

// GetAnalytics handles GET /v1/analytics.
func (h *AnalyticsHandler) GetAnalytics(w http.ResponseWriter, r *http.Request) {
	data := AnalyticsData{
		Summary: AnalyticsSummary{
			TopSearcher: "n/a",
		},
		TopQueries:        []TopQuery{},
		ZeroResultQueries: []ZeroResultQuery{},
		Trends:            []SearchTrendPoint{},
	}

	if h.auditStore != nil {
		entries, _, err := h.auditStore.List(1000, 0, audit.AuditFilters{})
		if err == nil {
			queryCounts := make(map[string]int)
			trendCounts := make(map[string]int)

			for _, entry := range entries {
				if entry.Action == "search" || entry.Action == "prompt.execute" || entry.Action == "run" {
					data.Summary.TotalSearches++
					if q := extractQuery(entry.Details); q != "" {
						queryCounts[q]++
					}
					dateKey := time.Unix(entry.Timestamp, 0).Format("2006-01-02")
					trendCounts[dateKey]++
				}
			}

			// Top queries
			type qc struct {
				q string
				c int
			}
			var sorted []qc
			for q, c := range queryCounts {
				sorted = append(sorted, qc{q, c})
			}
			sort.Slice(sorted, func(i, j int) bool { return sorted[i].c > sorted[j].c })
			if len(sorted) > 10 {
				sorted = sorted[:10]
			}
			for _, s := range sorted {
				data.TopQueries = append(data.TopQueries, TopQuery{Query: s.q, Count: s.c})
			}

			// Trends (last 30 days)
			now := time.Now()
			for i := 29; i >= 0; i-- {
				date := now.AddDate(0, 0, -i).Format("2006-01-02")
				data.Trends = append(data.Trends, SearchTrendPoint{
					Date:  date,
					Count: trendCounts[date],
				})
			}

			if data.Summary.TotalSearches > 0 && len(data.TopQueries) > 0 {
				data.Summary.TopSearcher = "system"
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("analytics: encode error: %v", err)
	}
}

// extractQuery attempts to pull a query string from a JSON details blob.
//
// Only the redacted prompt representation is surfaced: prompt_preview
// (truncated to 100 chars) or prompt_hash (SHA-256). The full "prompt"
// field, which older audit entries stored in cleartext, is deliberately
// NEVER returned so analytics never re-exposes user-pasted secrets.
func extractQuery(details string) string {
	if details == "" {
		return ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(details), &m); err != nil {
		return ""
	}
	if q, ok := m["query"].(string); ok && q != "" {
		return q
	}
	if q, ok := m["prompt_preview"].(string); ok && q != "" {
		return q
	}
	if q, ok := m["prompt_hash"].(string); ok && q != "" {
		return q
	}
	return ""
}
