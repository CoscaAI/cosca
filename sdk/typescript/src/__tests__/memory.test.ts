import { describe, it, expect, beforeEach, vi } from 'vitest';
import { AxiosInstance } from 'axios';
import { MemoryAPI } from '../memory';
import { AosClientConfig, MemoryStoreRequest, MemoryStoreResponse, MemorySearchResponse, MemoryGetResponse, MemoryDeleteResponse, MemoryPromoteResponse, MemoryStatsResponse } from '../types';

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

// =============================================================================
// MemoryAPI
// =============================================================================

describe('MemoryAPI', () => {
  let http: ReturnType<typeof mockHttp>;
  let api: MemoryAPI;

  beforeEach(() => {
    http = mockHttp();
    api = new MemoryAPI(http, mockConfig());
  });

  // ---- store() ----

  describe('store()', () => {
    it('stores a memory record', async () => {
      const request: MemoryStoreRequest = {
        type: 'decision',
        layer: 'session',
        content: 'Decided to use PostgreSQL',
        priority: 1,
        ttl: '24h',
        metadata: { key: 'value' },
      };
      const response: MemoryStoreResponse = { id: 'mem-001', createdAt: '2025-01-01T00:00:00Z' };
      (http.post as any).mockResolvedValue({ data: response });

      const result = await api.store(request);

      expect(result).toEqual(response);
      expect(http.post).toHaveBeenCalledWith('/v1/memory/store', request);
    });

    it('stores with minimal fields', async () => {
      const response: MemoryStoreResponse = { id: 'mem-002', createdAt: '2025-01-01T00:00:00Z' };
      (http.post as any).mockResolvedValue({ data: response });

      const result = await api.store({ content: 'Minimal memory' });

      expect(result).toEqual(response);
      expect(http.post).toHaveBeenCalledWith('/v1/memory/store', { content: 'Minimal memory' });
    });

    it('throws on empty content', async () => {
      await expect(api.store({ content: '' })).rejects.toThrow('Memory content is required');
    });

    it('throws on undefined content', async () => {
      await expect(api.store({ content: undefined as any })).rejects.toThrow('Memory content is required');
    });
  });

  // ---- retrieve() ----

  describe('retrieve()', () => {
    it('retrieves a memory record by ID and layer', async () => {
      const record = { id: 'mem-001', type: 'decision', layer: 'session', content: 'test', priority: 0, createdAt: '2025-01-01T00:00:00Z' };
      (http.get as any).mockResolvedValue({ data: { record } });

      const result = await api.retrieve('mem-001', 'session');

      expect(result).toEqual(record);
      expect(http.get).toHaveBeenCalledWith('/v1/memory/get', { params: { id: 'mem-001', layer: 'session' } });
    });

    it('throws on empty ID', async () => {
      await expect(api.retrieve('', 'session')).rejects.toThrow('Memory record ID is required');
    });

    it('throws on empty layer', async () => {
      await expect(api.retrieve('mem-001', '')).rejects.toThrow('Memory layer is required');
    });
  });

  // ---- search() ----

  describe('search()', () => {
    it('searches memory records', async () => {
      const response: MemorySearchResponse = {
        records: [{ id: '1', type: 'pattern', layer: 'session', content: 'test', priority: 0, createdAt: '2025-01-01T00:00:00Z' }],
        total: 1,
      };
      (http.get as any).mockResolvedValue({ data: response });

      const result = await api.search('test query');

      expect(result).toEqual(response);
      expect(http.get).toHaveBeenCalledWith('/v1/memory/search', { params: { query: 'test query' } });
    });

    it('passes optional filters', async () => {
      (http.get as any).mockResolvedValue({ data: { records: [], total: 0 } });

      await api.search('q', { limit: 5, types: ['pattern'], layers: ['session'] });

      expect(http.get).toHaveBeenCalledWith('/v1/memory/search', {
        params: { query: 'q', limit: 5, types: ['pattern'], layers: ['session'] },
      });
    });

    it('throws on empty query', async () => {
      await expect(api.search('')).rejects.toThrow('Search query is required');
    });

    it('handles empty results', async () => {
      (http.get as any).mockResolvedValue({ data: { records: [], total: 0 } });

      const result = await api.search('nothing');
      expect(result.records).toHaveLength(0);
      expect(result.total).toBe(0);
    });
  });

  // ---- delete() ----

  describe('delete()', () => {
    it('deletes a memory record', async () => {
      (http.delete as any).mockResolvedValue({ data: { success: true } });

      const result = await api.delete('mem-001');

      expect(result).toEqual({ success: true });
      expect(http.delete).toHaveBeenCalledWith('/v1/memory/delete', { params: { id: 'mem-001' } });
    });

    it('deletes with optional layer', async () => {
      (http.delete as any).mockResolvedValue({ data: { success: true } });

      await api.delete('mem-001', 'session');

      expect(http.delete).toHaveBeenCalledWith('/v1/memory/delete', { params: { id: 'mem-001', layer: 'session' } });
    });

    it('throws on empty ID', async () => {
      await expect(api.delete('')).rejects.toThrow('Memory record ID is required');
    });
  });

  // ---- promote() ----

  describe('promote()', () => {
    it('promotes a record between layers', async () => {
      const record = { id: 'mem-001', type: 'decision', layer: 'project', content: 'test', priority: 5, createdAt: '2025-01-01T00:00:00Z' };
      (http.post as any).mockResolvedValue({ data: { record } });

      const result = await api.promote('mem-001', 'session', 'project');

      expect(result).toEqual({ record });
      expect(http.post).toHaveBeenCalledWith('/v1/memory/promote', {
        id: 'mem-001',
        from_layer: 'session',
        to_layer: 'project',
      });
    });

    it('throws on empty ID', async () => {
      await expect(api.promote('', 'session', 'project')).rejects.toThrow('Memory record ID is required');
    });

    it('throws on empty fromLayer', async () => {
      await expect(api.promote('mem-001', '', 'project')).rejects.toThrow('fromLayer is required');
    });

    it('throws on empty toLayer', async () => {
      await expect(api.promote('mem-001', 'session', '')).rejects.toThrow('toLayer is required');
    });
  });

  // ---- stats() ----

  describe('stats()', () => {
    it('retrieves memory engine statistics', async () => {
      const response: MemoryStatsResponse = {
        layers: {
          session: { recordCount: 10, sizeBytes: 1024 },
          project: { recordCount: 5, sizeBytes: 512 },
        },
      };
      (http.get as any).mockResolvedValue({ data: response });

      const result = await api.stats();

      expect(result).toEqual(response);
      expect(http.get).toHaveBeenCalledWith('/v1/memory/stats');
    });
  });

  // ---- error propagation ----

  describe('error propagation', () => {
    it('propagates HTTP errors on store', async () => {
      (http.post as any).mockRejectedValue(new Error('Server Error'));
      await expect(api.store({ content: 'test' })).rejects.toThrow('Server Error');
    });

    it('propagates HTTP errors on retrieve', async () => {
      (http.get as any).mockRejectedValue(new Error('Not Found'));
      await expect(api.retrieve('id', 'layer')).rejects.toThrow('Not Found');
    });

    it('propagates HTTP errors on search', async () => {
      (http.get as any).mockRejectedValue(new Error('Internal Error'));
      await expect(api.search('query')).rejects.toThrow('Internal Error');
    });

    it('propagates HTTP errors on delete', async () => {
      (http.delete as any).mockRejectedValue(new Error('Not Allowed'));
      await expect(api.delete('id')).rejects.toThrow('Not Allowed');
    });

    it('propagates HTTP errors on promote', async () => {
      (http.post as any).mockRejectedValue(new Error('Conflict'));
      await expect(api.promote('id', 'a', 'b')).rejects.toThrow('Conflict');
    });

    it('propagates HTTP errors on stats', async () => {
      (http.get as any).mockRejectedValue(new Error('Unavailable'));
      await expect(api.stats()).rejects.toThrow('Unavailable');
    });
  });
});
