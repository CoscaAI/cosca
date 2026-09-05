import { describe, it, expect, beforeEach, vi } from 'vitest';
import { AxiosInstance } from 'axios';
import { AgentsAPI } from '../agents';
import { AosClientConfig, Agent } from '../types';

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

function mockAgent(overrides?: Partial<Agent>): Agent {
  return {
    name: 'assistant',
    role: 'General Assistant',
    mission: 'Help users',
    status: 'active',
    version: '1.0.0',
    department: 'core',
    ...overrides,
  };
}

// =============================================================================
// AgentsAPI
// =============================================================================

describe('AgentsAPI', () => {
  let http: ReturnType<typeof mockHttp>;
  let api: AgentsAPI;

  beforeEach(() => {
    http = mockHttp();
    api = new AgentsAPI(http, mockConfig());
  });

  // ---- list() ----

  describe('list()', () => {
    it('returns all agents', async () => {
      const agents = [mockAgent(), mockAgent({ name: 'coder', role: 'Code Assistant', status: 'inactive' })];
      (http.get as any).mockResolvedValue({ data: agents });

      const result = await api.list();

      expect(result).toEqual(agents);
      expect(result).toHaveLength(2);
      expect(http.get).toHaveBeenCalledWith('/v1/agents');
    });

    it('returns empty array when no agents', async () => {
      (http.get as any).mockResolvedValue({ data: [] });

      const result = await api.list();
      expect(result).toEqual([]);
    });
  });

  // ---- search() ----

  describe('search()', () => {
    it('finds agents by query', async () => {
      const agents = [mockAgent({ name: 'coder', role: 'Code Assistant' })];
      (http.get as any).mockResolvedValue({ data: agents });

      const result = await api.search('code');

      expect(result).toEqual(agents);
      expect(http.get).toHaveBeenCalledWith('/v1/agents/search?q=code');
    });

    it('URL-encodes special characters in query', async () => {
      (http.get as any).mockResolvedValue({ data: [] });

      await api.search('agent & assistant');

      expect(http.get).toHaveBeenCalledWith('/v1/agents/search?q=agent%20%26%20assistant');
    });

    it('throws on empty query', async () => {
      await expect(api.search('')).rejects.toThrow('Search query is required');
    });
  });

  // ---- get() ----

  describe('get()', () => {
    it('returns agent by name', async () => {
      const agent = mockAgent();
      (http.get as any).mockResolvedValue({ data: agent });

      const result = await api.get('assistant');

      expect(result).toEqual(agent);
      expect(http.get).toHaveBeenCalledWith('/v1/agents/assistant');
    });

    it('URL-encodes agent name', async () => {
      (http.get as any).mockResolvedValue({ data: mockAgent() });

      await api.get('my agent');

      expect(http.get).toHaveBeenCalledWith('/v1/agents/my%20agent');
    });

    it('throws on empty name', async () => {
      await expect(api.get('')).rejects.toThrow('Agent name is required');
    });
  });

  // ---- error propagation ----

  describe('error propagation', () => {
    it('propagates HTTP errors on list', async () => {
      (http.get as any).mockRejectedValue(new Error('Server Error'));
      await expect(api.list()).rejects.toThrow('Server Error');
    });

    it('propagates HTTP errors on search', async () => {
      (http.get as any).mockRejectedValue(new Error('Timeout'));
      await expect(api.search('query')).rejects.toThrow('Timeout');
    });

    it('propagates HTTP errors on get', async () => {
      (http.get as any).mockRejectedValue(new Error('Not Found'));
      await expect(api.get('unknown')).rejects.toThrow('Not Found');
    });
  });
});
