package grpcserver

import (
	cospb "github.com/CoscaAI/cosca/api/grpc/pb"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/search"
)

// pbToSearchParams converts a protobuf SearchRequest into domain-level
// search.SearchParams. Defaults are applied for unset fields (Limit=20,
// EnableFTS=true, EnableVector=true).
func pbToSearchParams(req *cospb.SearchRequest) search.SearchParams {
	params := search.DefaultSearchParams()
	params.Query = req.GetQuery()
	params.Limit = int(req.GetLimit())
	params.Offset = int(req.GetOffset())
	params.Types = req.GetTypes()
	params.Path = req.GetPathFilter()
	params.MinScore = req.GetMinScore()

	// Apply defaults when Limit is 0 (not set by client).
	if params.Limit <= 0 {
		params.Limit = 20
	}

	return params
}

// searchResultsToPb converts domain-level search.SearchResults into a
// protobuf SearchResponse, mapping results, facets, and metadata.
func searchResultsToPb(sr *search.SearchResults) *cospb.SearchResponse {
	if sr == nil {
		return &cospb.SearchResponse{}
	}

	results := make([]*cospb.SearchResult, 0, len(sr.Results))
	for _, r := range sr.Results {
		results = append(results, &cospb.SearchResult{
			Id:      r.ID,
			Title:   r.Title,
			Snippet: r.Snippet,
			Score:   r.Score,
			Type:    string(r.Type),
			Path:    r.DocumentPath,
		})
	}

	// Convert facets from map[string]map[string]int to []*cospb.Facet.
	facets := make([]*cospb.Facet, 0, len(sr.Facets))
	for name, counts := range sr.Facets {
		facetCounts := make(map[string]int32, len(counts))
		for k, v := range counts {
			facetCounts[k] = int32(v)
		}
		facets = append(facets, &cospb.Facet{
			Name:   name,
			Counts: facetCounts,
		})
	}

	return &cospb.SearchResponse{
		Results:    results,
		Total:      int32(sr.TotalCount),
		DurationMs: float64(sr.Duration.Milliseconds()),
		Facets:     facets,
	}
}

// statsToPb converts domain-level knowledge.Stats into a protobuf
// StatsResponse. Uptime is converted from time.Duration to seconds.
func statsToPb(stats *knowledge.Stats) *cospb.StatsResponse {
	if stats == nil {
		return &cospb.StatsResponse{}
	}

	return &cospb.StatsResponse{
		DocumentCount: int32(stats.DocumentCount),
		ChunkCount:    int32(stats.ChunkCount),
		EntityCount:   int32(stats.EntityCount),
		VectorCount:   int32(stats.VectorCount),
		DbSizeBytes:   stats.DBSize,
		UptimeSeconds: stats.Uptime.Seconds(),
	}
}

// syncResultToPb converts domain-level knowledge.SyncResult into a protobuf
// SyncResponse. The Added/Updated/Removed slices are mapped to their lengths
// as counts; individual error messages are propagated directly.
func syncResultToPb(result *knowledge.SyncResult) *cospb.SyncResponse {
	if result == nil {
		return &cospb.SyncResponse{}
	}

	return &cospb.SyncResponse{
		Added:   int32(len(result.Added)),
		Updated: int32(len(result.Updated)),
		Removed: int32(len(result.Removed)),
		Errors:  result.Errors,
	}
}
