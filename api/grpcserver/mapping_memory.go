package grpcserver

import (
	"time"

	cospb "github.com/CoscaAI/cosca/api/grpc/pb"
	"github.com/CoscaAI/cosca/internal/memory"
)

// pbToMemoryRecord converts a protobuf StoreRequest into a domain
// memory.MemoryRecord ready for persistence. TTL is parsed from the
// proto string field via time.ParseDuration; if parsing fails, TTL
// is left at zero so the engine applies its configured default.
func pbToMemoryRecord(req *cospb.StoreRequest) memory.MemoryRecord {
	record := memory.MemoryRecord{
		Type:     memory.MemoryType(req.GetType()),
		Layer:    memory.MemoryLayer(req.GetLayer()),
		Scope:    req.GetScope(),
		Content:  req.GetContent(),
		Priority: int(req.GetPriority()),
		Metadata: req.GetMetadata(),
	}

	if req.GetTtl() != "" {
		if ttl, err := time.ParseDuration(req.GetTtl()); err == nil {
			record.TTL = ttl
		}
	}

	return record
}

// pbToSearchOptions converts a protobuf MemorySearchRequest into domain
// search.SearchOptions consumed by the engine. String-based type and layer
// filters from the proto are mapped to their MemoryType/MemoryLayer domain
// constants without intermediate validation — the engine is lenient about
// unknown values.
func pbToSearchOptions(req *cospb.MemorySearchRequest) memory.SearchOptions {
	opts := memory.SearchOptions{
		Limit: int(req.GetLimit()),
	}

	// Clamp to the hard cap (M9b — DoS hardening): limit ≤ 100.
	const maxMemoryLimit = 100
	if opts.Limit <= 0 {
		opts.Limit = 20
	}
	if opts.Limit > maxMemoryLimit {
		opts.Limit = maxMemoryLimit
	}

	for _, t := range req.GetTypes() {
		opts.Types = append(opts.Types, memory.MemoryType(t))
	}
	for _, l := range req.GetLayers() {
		opts.Layers = append(opts.Layers, memory.MemoryLayer(l))
	}

	return opts
}

// memoryRecordToPb converts a domain memory.MemoryRecord into its protobuf
// representation. CreatedAt is formatted as RFC 3339 (ISO 8601).
func memoryRecordToPb(rec memory.MemoryRecord) *cospb.MemoryRecord {
	return &cospb.MemoryRecord{
		Id:        rec.ID,
		Type:      string(rec.Type),
		Layer:     string(rec.Layer),
		Content:   rec.Content,
		Priority:  int32(rec.Priority),
		CreatedAt: rec.CreatedAt.Format(time.RFC3339),
		Metadata:  rec.Metadata,
	}
}

// memoryRecordsToPb converts a slice of domain MemoryRecord into a slice of
// protobuf MemoryRecord pointers.
func memoryRecordsToPb(recs []memory.MemoryRecord) []*cospb.MemoryRecord {
	result := make([]*cospb.MemoryRecord, 0, len(recs))
	for _, rec := range recs {
		result = append(result, memoryRecordToPb(rec))
	}
	return result
}

// layerStatsToPb converts the engine's map[MemoryLayer]LayerStats into the
// protobuf wire format, mapping Count → RecordCount and TotalSize → SizeBytes.
func layerStatsToPb(stats map[memory.MemoryLayer]memory.LayerStats) map[string]*cospb.LayerStats {
	result := make(map[string]*cospb.LayerStats, len(stats))
	for layer, s := range stats {
		result[string(layer)] = &cospb.LayerStats{
			RecordCount: int32(s.Count),
			SizeBytes:   int64(s.TotalSize),
		}
	}
	return result
}
