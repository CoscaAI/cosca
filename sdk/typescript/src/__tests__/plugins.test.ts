import { describe, it, expect, beforeEach, vi } from 'vitest';
import { AxiosInstance } from 'axios';
import { PluginsAPI } from '../plugins';
import { AosClientConfig, PluginInfo } from '../types';

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

function mockPlugin(overrides?: Partial<PluginInfo>): PluginInfo {
  return {
    id: 'plugin-001',
    name: 'my-plugin',
    version: '1.0.0',
    description: 'A test plugin',
    author: 'Test Author',
    license: 'MIT',
    type: 'tool',
    apiVersion: 'v1',
    status: 'active',
    enabled: true,
    permissions: ['network', 'filesystem'],
    entrypoint: './index.js',
    runtime: 'node',
    hooks: ['onStart', 'onRequest'],
    config: { key: 'value' },
    installedAt: '2025-01-01T00:00:00Z',
    ...overrides,
  };
}

// =============================================================================
// PluginsAPI
// =============================================================================

describe('PluginsAPI', () => {
  let http: ReturnType<typeof mockHttp>;
  let api: PluginsAPI;

  beforeEach(() => {
    http = mockHttp();
    api = new PluginsAPI(http, mockConfig());
  });

  // ---- install() ----

  describe('install()', () => {
    it('installs a plugin from source', async () => {
      (http.post as any).mockResolvedValue({ data: undefined });

      await api.install('/path/to/plugin');

      expect(http.post).toHaveBeenCalledWith('/v1/plugins/install', { source: '/path/to/plugin' });
    });

    it('installs from URL', async () => {
      (http.post as any).mockResolvedValue({ data: undefined });

      await api.install('https://registry.example.com/plugin');

      expect(http.post).toHaveBeenCalledWith('/v1/plugins/install', {
        source: 'https://registry.example.com/plugin',
      });
    });

    it('throws on empty source', async () => {
      await expect(api.install('')).rejects.toThrow('Plugin source is required');
    });
  });

  // ---- uninstall() ----

  describe('uninstall()', () => {
    it('uninstalls a plugin by ID', async () => {
      (http.delete as any).mockResolvedValue({ data: undefined });

      await api.uninstall('plugin-001');

      expect(http.delete).toHaveBeenCalledWith('/v1/plugins/plugin-001');
    });

    it('throws on empty ID', async () => {
      await expect(api.uninstall('')).rejects.toThrow('Plugin ID is required');
    });
  });

  // ---- list() ----

  describe('list()', () => {
    it('returns all plugins', async () => {
      const plugins = [mockPlugin(), mockPlugin({ id: 'plugin-002', name: 'other-plugin', type: 'hook' })];
      (http.get as any).mockResolvedValue({ data: { plugins } });

      const result = await api.list();

      expect(result).toEqual(plugins);
      expect(result).toHaveLength(2);
      expect(http.get).toHaveBeenCalledWith('/v1/plugins');
    });

    it('returns empty array when no plugins', async () => {
      (http.get as any).mockResolvedValue({ data: { plugins: [] } });

      const result = await api.list();
      expect(result).toEqual([]);
    });
  });

  // ---- get() ----

  describe('get()', () => {
    it('returns plugin by ID', async () => {
      const plugin = mockPlugin();
      (http.get as any).mockResolvedValue({ data: plugin });

      const result = await api.get('plugin-001');

      expect(result).toEqual(plugin);
      expect(http.get).toHaveBeenCalledWith('/v1/plugins/plugin-001');
    });

    it('throws on empty ID', async () => {
      await expect(api.get('')).rejects.toThrow('Plugin ID is required');
    });
  });

  // ---- error propagation ----

  describe('error propagation', () => {
    it('propagates HTTP errors on install', async () => {
      (http.post as any).mockRejectedValue(new Error('Already Installed'));
      await expect(api.install('/path')).rejects.toThrow('Already Installed');
    });

    it('propagates HTTP errors on uninstall', async () => {
      (http.delete as any).mockRejectedValue(new Error('Not Found'));
      await expect(api.uninstall('id')).rejects.toThrow('Not Found');
    });

    it('propagates HTTP errors on list', async () => {
      (http.get as any).mockRejectedValue(new Error('Server Error'));
      await expect(api.list()).rejects.toThrow('Server Error');
    });

    it('propagates HTTP errors on get', async () => {
      (http.get as any).mockRejectedValue(new Error('Not Found'));
      await expect(api.get('unknown')).rejects.toThrow('Not Found');
    });
  });
});
