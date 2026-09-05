// Package grpcclient provides gRPC clients for Cosca daemon services.
//
// This file implements orchestration port interfaces (KnowledgeSearcher,
// MemoryRetriever, MemoryStorer) that adapt gRPC clients to the
// orchestration engine's consumer-side contracts. When the runtime daemon
// is reachable, the orchestration engine queries knowledge and memory
// through gRPC instead of using local engines — the Single Owner Model.
package grpcclient

import (
	"context"
	"fmt"
	"strings"

	cospb "github.com/CoscaAI/cosca/api/grpc/pb"
	"github.com/CoscaAI/cosca/internal/orchestration"
)

// ─── Knowledge Searcher Adapter ───────────────────────────────────────────────

// knowledgeSearcherAdapter implements orchestration.KnowledgeSearcher by
// delegating to the runtime daemon via KnowledgeService gRPC.
type knowledgeSearcherAdapter struct {
	client *KnowledgeClient
}

// NewKnowledgeSearcherAdapter wraps a KnowledgeClient as an
// orchestration.KnowledgeSearcher.
func NewKnowledgeSearcherAdapter(client *KnowledgeClient) orchestration.KnowledgeSearcher {
	return &knowledgeSearcherAdapter{client: client}
}

func (a *knowledgeSearcherAdapter) Search(ctx context.Context, params orchestration.KnowledgeSearchParams) (*orchestration.KnowledgeSearchResults, error) {
	req := &cospb.SearchRequest{
		Query:    params.Query,
		Limit:    int32(params.Limit),
		Types:    params.Types,
		PathFilter: params.Path,
		MinScore: params.MinScore,
	}
	resp, err := a.client.Search(ctx, req)
	if err != nil {
		return nil, err
	}
	results := make([]orchestration.KnowledgeSearchResult, len(resp.Results))
	for i, r := range resp.Results {
		results[i] = orchestration.KnowledgeSearchResult{
			ID:           r.Id,
			Title:        r.Title,
			Snippet:      r.Snippet,
			Score:        float64(r.Score),
			DocumentPath: r.Path,
		}
	}
	return &orchestration.KnowledgeSearchResults{
		Results:    results,
		TotalCount: int(resp.Total),
	}, nil
}

// ─── Memory Retriever Adapter ─────────────────────────────────────────────────

// memoryRetrieverAdapter implements orchestration.MemoryRetriever by
// delegating to the runtime daemon via MemoryService gRPC.
type memoryRetrieverAdapter struct {
	client *MemoryClient
}

// NewMemoryRetrieverAdapter wraps a MemoryClient as an
// orchestration.MemoryRetriever.
func NewMemoryRetrieverAdapter(client *MemoryClient) orchestration.MemoryRetriever {
	return &memoryRetrieverAdapter{client: client}
}

func (a *memoryRetrieverAdapter) Search(ctx context.Context, query string, opts orchestration.MemorySearchOptions) ([]orchestration.MemoryRecord, error) {
	req := &cospb.MemorySearchRequest{
		Query:  query,
		Types:  opts.Types,
		Layers: opts.Layers,
		Limit:  int32(opts.Limit),
	}
	resp, err := a.client.Search(ctx, req)
	if err != nil {
		// When the daemon is down, return an empty result set so the
		// orchestration engine can proceed without memory context.
		if strings.Contains(err.Error(), "não está ativo") {
			return nil, nil
		}
		return nil, err
	}
	records := make([]orchestration.MemoryRecord, len(resp.Records))
	for i, r := range resp.Records {
		records[i] = orchestration.MemoryRecord{
			ID:       r.Id,
			Type:     r.Type,
			Layer:    r.Layer,
			Content:  r.Content,
			Priority: int(r.Priority),
		}
		if r.Metadata != nil {
			records[i].Metadata = r.Metadata
		}
	}
	return records, nil
}

func (a *memoryRetrieverAdapter) Retrieve(ctx context.Context, id, layer string) (*orchestration.MemoryRecord, error) {
	req := &cospb.GetRequest{
		Id:    id,
		Layer: layer,
	}
	resp, err := a.client.Get(ctx, req)
	if err != nil {
		if strings.Contains(err.Error(), "não está ativo") {
			return nil, fmt.Errorf("memory: record %q not available (runtime daemon offline)", id)
		}
		return nil, err
	}
	if resp.Record == nil {
		return nil, fmt.Errorf("memory: record %q not found", id)
	}
	r := resp.Record
	rec := &orchestration.MemoryRecord{
		ID:       r.Id,
		Type:     r.Type,
		Layer:    r.Layer,
		Content:  r.Content,
		Priority: int(r.Priority),
	}
	if r.Metadata != nil {
		rec.Metadata = r.Metadata
	}
	return rec, nil
}

// ─── Memory Storer Adapter ────────────────────────────────────────────────────

// memoryStorerAdapter implements orchestration.MemoryStorer by
// delegating to the runtime daemon via MemoryService gRPC.
type memoryStorerAdapter struct {
	client *MemoryClient
}

// NewMemoryStorerAdapter wraps a MemoryClient as an
// orchestration.MemoryStorer.
func NewMemoryStorerAdapter(client *MemoryClient) orchestration.MemoryStorer {
	return &memoryStorerAdapter{client: client}
}

func (a *memoryStorerAdapter) Store(ctx context.Context, record orchestration.MemoryRecord) (*orchestration.MemoryRecord, error) {
	req := &cospb.StoreRequest{
		Type:    record.Type,
		Layer:   record.Layer,
		Scope:   "",
		Content: record.Content,
		Priority: int32(record.Priority),
		Metadata: record.Metadata,
	}
	resp, err := a.client.Store(ctx, req)
	if err != nil {
		if strings.Contains(err.Error(), "não está ativo") {
			return nil, fmt.Errorf("memory: store failed (runtime daemon offline)")
		}
		return nil, err
	}
	record.ID = resp.Id
	return &record, nil
}
