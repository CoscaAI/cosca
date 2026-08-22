import { describe, it, expect, beforeEach, vi } from 'vitest';
import { AxiosInstance } from 'axios';
import { RuntimeAPI } from '../runtime';
import { AosClientConfig, RuntimeStatusResponse, RuntimeHealthResponse } from '../types';

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
// RuntimeAPI
// =============================================================================

describe('RuntimeAPI', () => {
  let http: ReturnType<typeof mockHttp>;
  let api: RuntimeAPI;

  beforeEach(() => {
    http = mockHttp();
    api = new RuntimeAPI(http, mockConfig());
  });

  // ---- status() ----

  describe('status()', () => {
    it('returns runtime status', async () => {
      const response: RuntimeStatusResponse = {
        state: 'running',
        health: 'healthy',
        uptime: '2h 30m',
        version: '1.2.3',
        components: {
          knowledge: { name: 'knowledge', status: 'healthy', uptime: '2h 30m' },
          memory: { name: 'memory', status: 'healthy', uptime: '2h 30m' },
        },
      };
      (http.get as any).mockResolvedValue({ data: response });

      const result = await api.status();

      expect(result).toEqual(response);
      expect(http.get).toHaveBeenCalledWith('/v1/status');
    });

    it('returns degraded status', async () => {
      const response: RuntimeStatusResponse = {
        state: 'running',
        health: 'degraded',
        uptime: '1h',
        version: '1.2.3',
        components: {
          knowledge: { name: 'knowledge', status: 'unhealthy', message: 'DB error' },
        },
      };
      (http.get as any).mockResolvedValue({ data: response });

      const result = await api.status();
      expect(result.health).toBe('degraded');
      expect(result.components?.knowledge.status).toBe('unhealthy');
    });
  });

  // ---- health() ----

  describe('health()', () => {
    it('returns healthy status', async () => {
      const response: RuntimeHealthResponse = { healthy: true };
      (http.get as any).mockResolvedValue({ data: response });

      const result = await api.health();

      expect(result).toEqual(response);
      expect(http.get).toHaveBeenCalledWith('/v1/health');
    });

    it('returns unhealthy with warnings', async () => {
      const response: RuntimeHealthResponse = {
        healthy: false,
        warnings: ['Knowledge engine degraded', 'Memory engine unreachable'],
      };
      (http.get as any).mockResolvedValue({ data: response });

      const result = await api.health();
      expect(result.healthy).toBe(false);
      expect(result.warnings).toHaveLength(2);
    });

    it('returns healthy without warnings', async () => {
      const response: RuntimeHealthResponse = { healthy: true };
      (http.get as any).mockResolvedValue({ data: response });

      const result = await api.health();
      expect(result.warnings).toBeUndefined();
    });
  });

  // ---- error propagation ----

  describe('error propagation', () => {
    it('propagates HTTP errors on status', async () => {
      (http.get as any).mockRejectedValue(new Error('Connection Refused'));
      await expect(api.status()).rejects.toThrow('Connection Refused');
    });

    it('propagates HTTP errors on health', async () => {
      (http.get as any).mockRejectedValue(new Error('Timeout'));
      await expect(api.health()).rejects.toThrow('Timeout');
    });
  });
});
