import { describe, it, expect, beforeEach, vi } from 'vitest';
import { AxiosInstance } from 'axios';
import { KnowledgeAPI } from '../knowledge';
import { AosClientConfig, KnowledgeSearchResponse, KnowledgeIndexResponse, KnowledgeStatsResponse, KnowledgeSyncResponse } from '../types';

// =============================================================================
// Helpers
// =============================================================================

function mockHttp() {
  return {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  } as unknown as AxiosInstance;
}

function mockConfig(): Required<AosClientConfig> {
  return {
    baseUrl: 'http://test:1',
    apiKey: '',
    timeout: 5000,
    retryCount: 1,
    headers: {},
  };
}

function mockSearchResponse(overrides?: Partial<KnowledgeSearchResponse>): KnowledgeSearchResponse {
  return {
    results: [
      { id: '1', title: 'Test Doc', snippet: 'matching text', score: 0.95, type: 'file', path: '/docs/test.md' },
    ],
    total: 1,
    durationMs: 12,
    ...overrides,
  };
}

// =============================================================================
// KnowledgeAPI
// =============================================================================

describe('KnowledgeAPI', () => {
  let http: ReturnType<typeof mockHttp>;
  let api: KnowledgeAPI;

  beforeEach(() => {
    http = mockHttp();
    api = new KnowledgeAPI(http, mockConfig());
  });

  // ---- search() ----

  describe('search()', () => {
    it('searches with query string', async () => {
      const response = mockSearchResponse();
      (http.post as any).mockResolvedValue({ data: response });

      const result = await api.search('test query');

      expect(result).toEqual(response);
      expect(http.post).toHaveBeenCalledWith('/v1/knowledge/search', { query: 'test query' });
    });

    it('passes all search options', async () => {
      (http.post as any).mockResolvedValue({ data: mockSearchResponse() });

      await api.search('q', {
        limit: 10,
        offset: 5,
        types: ['file', 'entity'],
        pathFilter: '/docs/',
        minScore: 0.7,
        enableFts: true,
        enableVector: false,
        enableGraph: true,
        enableFacets: true,
        tags: { lang: 'typescript' },
      });

      expect(http.post).toHaveBeenCalledWith('/v1/knowledge/search', {
        query: 'q',
        limit: 10,
        offset: 5,
        types: ['file', 'entity'],
        path_filter: '/docs/',
        min_score: 0.7,
        enable_fts: true,
        enable_vector: false,
        enable_graph: true,
        enable_facets: true,
        tags: { lang: 'typescript' },
      });
    });

    it('throws on empty query', async () => {
      await expect(api.search('')).rejects.toThrow('Search query is required');
      expect(http.post).not.toHaveBeenCalled();
    });

    it('throws on undefined query', async () => {
      await expect(api.search(undefined as any)).rejects.toThrow('Search query is required');
    });

    it('handles empty results', async () => {
      (http.post as any).mockResolvedValue({
        data: { results: [], total: 0, durationMs: 1 },
      });

      const result = await api.search('nothing');
      expect(result.results).toHaveLength(0);
      expect(result.total).toBe(0);
    });
  });

  // ---- index() ----

  describe('index()', () => {
    it('indexes a file path', async () => {
      const response: KnowledgeIndexResponse = { documentsIndexed: 5, chunksIndexed: 20 };
      (http.post as any).mockResolvedValue({ data: response });

      const result = await api.index('/docs/file.md');

      expect(result).toEqual(response);
      expect(http.post).toHaveBeenCalledWith('/v1/knowledge/index', {
        path: '/docs/file.md',
        recursive: false,
      });
    });

    it('indexes recursively when flag is set', async () => {
      (http.post as any).mockResolvedValue({ data: {} });

      await api.index('/docs/', true);

      expect(http.post).toHaveBeenCalledWith('/v1/knowledge/index', {
        path: '/docs/',
        recursive: true,
      });
    });

    it('throws on empty path', async () => {
      await expect(api.index('')).rejects.toThrow('Document path is required');
    });
  });

  // ---- stats() ----

  describe('stats()', () => {
    it('retrieves knowledge engine statistics', async () => {
      const response: KnowledgeStatsResponse = {
        documentCount: 42,
        chunkCount: 150,
        entityCount: 300,
        vectorCount: 500,
        dbSizeBytes: 1024000,
        uptimeSeconds: 3600,
      };
      (http.get as any).mockResolvedValue({ data: response });

      const result = await api.stats();

      expect(result).toEqual(response);
      expect(http.get).toHaveBeenCalledWith('/v1/knowledge/stats');
    });
  });

  // ---- sync() ----

  describe('sync()', () => {
    it('syncs the index', async () => {
      const response: KnowledgeSyncResponse = {
        added: 3,
        updated: 1,
        removed: 2,
        durationMs: 500,
      };
      (http.post as any).mockResolvedValue({ data: response });

      const result = await api.sync();

      expect(result).toEqual(response);
      expect(http.post).toHaveBeenCalledWith('/v1/knowledge/sync');
    });
  });

  // ---- searchByType() ----

  describe('searchByType()', () => {
    it('searches by specific type', async () => {
      (http.post as any).mockResolvedValue({ data: mockSearchResponse() });

      await api.searchByType('agent', 'test query');

      expect(http.post).toHaveBeenCalledWith('/v1/knowledge/search', {
        query: 'test query',
        types: ['agent'],
      });
    });

    it('throws on empty resultType', async () => {
      await expect(api.searchByType('', 'query')).rejects.toThrow('Result type is required');
    });

    it('merges additional options', async () => {
      (http.post as any).mockResolvedValue({ data: mockSearchResponse() });

      await api.searchByType('agent', 'query', { limit: 5, minScore: 0.5 });

      expect(http.post).toHaveBeenCalledWith('/v1/knowledge/search', {
        query: 'query',
        types: ['agent'],
        limit: 5,
        min_score: 0.5,
      });
    });
  });

  // ---- searchFts() ----

  describe('searchFts()', () => {
    it('enables FTS and disables vector/graph', async () => {
      (http.post as any).mockResolvedValue({ data: mockSearchResponse() });

      await api.searchFts('query', { limit: 10 });

      expect(http.post).toHaveBeenCalledWith('/v1/knowledge/search', {
        query: 'query',
        limit: 10,
        enable_fts: true,
        enable_vector: false,
        enable_graph: false,
      });
    });
  });

  // ---- searchVector() ----

  describe('searchVector()', () => {
    it('enables vector and disables FTS/graph', async () => {
      (http.post as any).mockResolvedValue({ data: mockSearchResponse() });

      await api.searchVector('query');

      expect(http.post).toHaveBeenCalledWith('/v1/knowledge/search', {
        query: 'query',
        enable_fts: false,
        enable_vector: true,
        enable_graph: false,
      });
    });
  });

  // ---- searchHybrid() ----

  describe('searchHybrid()', () => {
    it('enables FTS + vector, graph optional', async () => {
      (http.post as any).mockResolvedValue({ data: mockSearchResponse() });

      await api.searchHybrid('query');

      expect(http.post).toHaveBeenCalledWith('/v1/knowledge/search', {
        query: 'query',
        enable_fts: true,
        enable_vector: true,
        enable_graph: false,
      });
    });

    it('enables graph when specified', async () => {
      (http.post as any).mockResolvedValue({ data: mockSearchResponse() });

      await api.searchHybrid('query', { enableGraph: true });

      expect(http.post).toHaveBeenCalledWith('/v1/knowledge/search', {
        query: 'query',
        enable_fts: true,
        enable_vector: true,
        enable_graph: true,
      });
    });
  });

  // ---- error propagation ----

  describe('error propagation', () => {
    it('propagates HTTP errors on search', async () => {
      (http.post as any).mockRejectedValue(new Error('Server Error'));
      await expect(api.search('query')).rejects.toThrow('Server Error');
    });

    it('propagates HTTP errors on index', async () => {
      (http.post as any).mockRejectedValue(new Error('Not Found'));
      await expect(api.index('/path')).rejects.toThrow('Not Found');
    });

    it('propagates HTTP errors on stats', async () => {
      (http.get as any).mockRejectedValue(new Error('Internal Error'));
      await expect(api.stats()).rejects.toThrow('Internal Error');
    });
  });
});
