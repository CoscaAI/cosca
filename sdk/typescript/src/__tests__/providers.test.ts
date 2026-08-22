import { describe, it, expect, beforeEach, vi } from 'vitest';
import { AxiosInstance } from 'axios';
import { ProvidersAPI } from '../providers';
import { AosClientConfig, Provider, TestResult, ProviderStatus } from '../types';

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

function mockProvider(overrides?: Partial<Provider>): Provider {
  return {
    name: 'openai',
    status: 'configured',
    model: 'gpt-4o',
    active: true,
    configured: true,
    baseUrl: 'https://api.openai.com/v1',
    apiVersion: 'v1',
    models: ['gpt-4o', 'gpt-4o-mini'],
    capabilities: ['chat', 'embeddings'],
    ...overrides,
  };
}

// =============================================================================
// ProvidersAPI
// =============================================================================

describe('ProvidersAPI', () => {
  let http: ReturnType<typeof mockHttp>;
  let api: ProvidersAPI;

  beforeEach(() => {
    http = mockHttp();
    api = new ProvidersAPI(http, mockConfig());
  });

  // ---- list() ----

  describe('list()', () => {
    it('returns all providers', async () => {
      const providers = [mockProvider(), mockProvider({ name: 'anthropic', active: false })];
      (http.get as any).mockResolvedValue({ data: providers });

      const result = await api.list();

      expect(result).toEqual(providers);
      expect(result).toHaveLength(2);
      expect(http.get).toHaveBeenCalledWith('/v1/providers');
    });

    it('returns empty array when no providers', async () => {
      (http.get as any).mockResolvedValue({ data: [] });

      const result = await api.list();
      expect(result).toEqual([]);
    });
  });

  // ---- get() ----

  describe('get()', () => {
    it('returns provider by name', async () => {
      const provider = mockProvider();
      (http.get as any).mockResolvedValue({ data: provider });

      const result = await api.get('openai');

      expect(result).toEqual(provider);
      expect(http.get).toHaveBeenCalledWith('/v1/providers/openai');
    });

    it('URL-encodes provider name', async () => {
      (http.get as any).mockResolvedValue({ data: mockProvider() });

      await api.get('my provider');

      expect(http.get).toHaveBeenCalledWith('/v1/providers/my%20provider');
    });

    it('throws on empty name', async () => {
      await expect(api.get('')).rejects.toThrow('Provider name is required');
    });
  });

  // ---- test() ----

  describe('test()', () => {
    it('tests provider connectivity', async () => {
      const result: TestResult = {
        responseTime: '250ms',
        model: 'gpt-4o',
        status: 'reachable',
      };
      (http.post as any).mockResolvedValue({ data: result });

      const testResult = await api.test('openai');

      expect(testResult).toEqual(result);
      expect(http.post).toHaveBeenCalledWith('/v1/providers/openai/test');
    });

    it('tests returns timeout status', async () => {
      const result: TestResult = {
        responseTime: '5000ms',
        model: 'unknown',
        status: 'timeout',
      };
      (http.post as any).mockResolvedValue({ data: result });

      const testResult = await api.test('unreachable');
      expect(testResult.status).toBe('timeout');
    });

    it('throws on empty name', async () => {
      await expect(api.test('')).rejects.toThrow('Provider name is required');
    });
  });

  // ---- setActive() ----

  describe('setActive()', () => {
    it('activates a provider', async () => {
      const status: ProviderStatus = {
        active: 'openai',
        configured: 3,
        available: 5,
        statuses: ['openai: ready', 'anthropic: configured'],
      };
      (http.put as any).mockResolvedValue({ data: status });

      const result = await api.setActive('openai');

      expect(result).toEqual(status);
      expect(http.put).toHaveBeenCalledWith('/v1/providers/active', {
        provider: 'openai',
        model: '',
      });
    });

    it('activates with specific model', async () => {
      (http.put as any).mockResolvedValue({ data: {} });

      await api.setActive('openai', 'gpt-4o-mini');

      expect(http.put).toHaveBeenCalledWith('/v1/providers/active', {
        provider: 'openai',
        model: 'gpt-4o-mini',
      });
    });

    it('throws on empty name', async () => {
      await expect(api.setActive('')).rejects.toThrow('Provider name is required');
    });
  });

  // ---- error propagation ----

  describe('error propagation', () => {
    it('propagates HTTP errors on list', async () => {
      (http.get as any).mockRejectedValue(new Error('Server Error'));
      await expect(api.list()).rejects.toThrow('Server Error');
    });

    it('propagates HTTP errors on get', async () => {
      (http.get as any).mockRejectedValue(new Error('Not Found'));
      await expect(api.get('unknown')).rejects.toThrow('Not Found');
    });

    it('propagates HTTP errors on test', async () => {
      (http.post as any).mockRejectedValue(new Error('Connection Refused'));
      await expect(api.test('openai')).rejects.toThrow('Connection Refused');
    });

    it('propagates HTTP errors on setActive', async () => {
      (http.put as any).mockRejectedValue(new Error('Unauthorized'));
      await expect(api.setActive('openai')).rejects.toThrow('Unauthorized');
    });
  });
});
