import { describe, it, expect, beforeEach, vi } from 'vitest';
import { AxiosInstance } from 'axios';
import { ContextAPI } from '../context';
import { AosClientConfig, Context } from '../types';

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

function mockContext(overrides?: Partial<Context>): Context {
  return {
    id: 'ctx-001',
    scope: 'session',
    entries: [
      { id: 'e1', key: 'project', value: 'cosca', scope: 'session', priority: 10, source: 'env', createdAt: '2025-01-01T00:00:00Z' },
    ],
    entryCount: 1,
    tokenCount: 42,
    createdAt: '2025-01-01T00:00:00Z',
    ...overrides,
  };
}

// =============================================================================
// ContextAPI
// =============================================================================

describe('ContextAPI', () => {
  let http: ReturnType<typeof mockHttp>;
  let api: ContextAPI;

  beforeEach(() => {
    http = mockHttp();
    api = new ContextAPI(http, mockConfig());
  });

  // ---- build() ----

  describe('build()', () => {
    it('builds context from query', async () => {
      const ctx = mockContext();
      (http.post as any).mockResolvedValue({ data: ctx });

      const result = await api.build('project context');

      expect(result).toEqual(ctx);
      expect(http.post).toHaveBeenCalledWith('/v1/context/build', { query: 'project context' });
    });

    it('passes context options', async () => {
      const ctx = mockContext();
      (http.post as any).mockResolvedValue({ data: ctx });

      await api.build('query', {
        scope: 'session',
        maxEntries: 50,
        maxTokens: 4096,
        includeMemory: true,
        includeKnowledge: false,
        priorityThreshold: 5,
        agentId: 'agent-1',
      });

      expect(http.post).toHaveBeenCalledWith('/v1/context/build', {
        query: 'query',
        scope: 'session',
        maxEntries: 50,
        maxTokens: 4096,
        includeMemory: true,
        includeKnowledge: false,
        priorityThreshold: 5,
        agentId: 'agent-1',
      });
    });

    it('throws on empty query', async () => {
      await expect(api.build('')).rejects.toThrow('Context query is required');
    });

    it('throws on undefined query', async () => {
      await expect(api.build(undefined as any)).rejects.toThrow('Context query is required');
    });

    it('works without options', async () => {
      (http.post as any).mockResolvedValue({ data: mockContext() });

      const result = await api.build('test');
      expect(result.id).toBe('ctx-001');
    });
  });

  // ---- getCurrent() ----

  describe('getCurrent()', () => {
    it('returns current context', async () => {
      const ctx = mockContext({ id: 'current-ctx' });
      (http.get as any).mockResolvedValue({ data: ctx });

      const result = await api.getCurrent();

      expect(result).toEqual(ctx);
      expect(http.get).toHaveBeenCalledWith('/v1/context/current');
    });

    it('returns empty context', async () => {
      const ctx = mockContext({ entries: [], entryCount: 0, tokenCount: 0 });
      (http.get as any).mockResolvedValue({ data: ctx });

      const result = await api.getCurrent();
      expect(result.entries).toHaveLength(0);
      expect(result.entryCount).toBe(0);
    });
  });

  // ---- clear() ----

  describe('clear()', () => {
    it('clears the current context', async () => {
      (http.delete as any).mockResolvedValue({ data: undefined });

      await api.clear();

      expect(http.delete).toHaveBeenCalledWith('/v1/context/current');
    });
  });

  // ---- error propagation ----

  describe('error propagation', () => {
    it('propagates HTTP errors on build', async () => {
      (http.post as any).mockRejectedValue(new Error('Internal Error'));
      await expect(api.build('query')).rejects.toThrow('Internal Error');
    });

    it('propagates HTTP errors on getCurrent', async () => {
      (http.get as any).mockRejectedValue(new Error('Not Found'));
      await expect(api.getCurrent()).rejects.toThrow('Not Found');
    });

    it('propagates HTTP errors on clear', async () => {
      (http.delete as any).mockRejectedValue(new Error('Forbidden'));
      await expect(api.clear()).rejects.toThrow('Forbidden');
    });
  });
});
