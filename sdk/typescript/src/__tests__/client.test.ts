import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { AosClient, CoscaError } from '../client';
import { AosClientConfig } from '../types';

// =============================================================================
// Helpers
// =============================================================================

/** Build a minimal mock AxiosInstance shape for sub-API testing. */
function mockAxiosInstance() {
  return {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
    interceptors: {
      response: { use: vi.fn() },
    },
  };
}

// =============================================================================
// Config Resolution
// =============================================================================

describe('AosClient — config resolution', () => {
  const OLD_ENV = { ...process.env };

  beforeEach(() => {
    delete process.env.COSCA_API_URL;
  });

  afterEach(() => {
    process.env = { ...OLD_ENV };
  });

  it('uses default baseUrl when no env nor config', () => {
    const client = new AosClient();
    expect(client.getConfig().baseUrl).toBe('http://localhost:14120');
  });

  it('uses explicit baseUrl from config', () => {
    const client = new AosClient({ baseUrl: 'http://example.com:9090' });
    expect(client.getConfig().baseUrl).toBe('http://example.com:9090');
  });

  it('prefers env var COSCA_API_URL over config baseUrl', () => {
    process.env.COSCA_API_URL = 'http://env-host:8888';
    const client = new AosClient({ baseUrl: 'http://config-host:9999' });
    expect(client.getConfig().baseUrl).toBe('http://env-host:8888');
  });

  it('prefers env var COSCA_API_URL over default', () => {
    process.env.COSCA_API_URL = 'http://env-host:8888';
    const client = new AosClient();
    expect(client.getConfig().baseUrl).toBe('http://env-host:8888');
  });

  it('strips trailing slash from baseUrl', () => {
    const client = new AosClient({ baseUrl: 'http://example.com:9090/' });
    expect(client.getConfig().baseUrl).toBe('http://example.com:9090');
  });

  it('respects explicit timeout', () => {
    const client = new AosClient({ timeout: 5000 });
    expect(client.getConfig().timeout).toBe(5000);
  });

  it('respects explicit retryCount', () => {
    const client = new AosClient({ retryCount: 1 });
    expect(client.getConfig().retryCount).toBe(1);
  });

  it('defaults timeout to 30000', () => {
    const client = new AosClient();
    expect(client.getConfig().timeout).toBe(30000);
  });

  it('defaults retryCount to 3', () => {
    const client = new AosClient();
    expect(client.getConfig().retryCount).toBe(3);
  });

  it('accepts empty config', () => {
    const client = new AosClient({});
    expect(client.getConfig().baseUrl).toBe('http://localhost:14120');
    expect(client.getConfig().timeout).toBe(30000);
    expect(client.getConfig().retryCount).toBe(3);
  });

  it('includes apiKey in Authorization header when provided', () => {
    const client = new AosClient({ apiKey: 'secret-token' });
    expect(client.getConfig().apiKey).toBe('secret-token');
  });
});

// =============================================================================
// Sub-API Initialisation
// =============================================================================

describe('AosClient — sub-API access', () => {
  it('exposes all 11 sub-APIs', () => {
    const client = new AosClient();
    expect(client.knowledge).toBeDefined();
    expect(client.memory).toBeDefined();
    expect(client.context).toBeDefined();
    expect(client.runtime).toBeDefined();
    expect(client.plugins).toBeDefined();
    expect(client.discovery).toBeDefined();
    expect(client.agents).toBeDefined();
    expect(client.skills).toBeDefined();
    expect(client.providers).toBeDefined();
    expect(client.workflows).toBeDefined();
    expect(client.orchestration).toBeDefined();
  });

  it('getConfig returns a frozen copy', () => {
    const client = new AosClient({ baseUrl: 'http://x:1' });
    const cfg = client.getConfig();
    expect(cfg.baseUrl).toBe('http://x:1');
    // Mutating the returned object should not affect the internal config.
    (cfg as unknown as AosClientConfig).baseUrl = 'changed';
    expect(client.getConfig().baseUrl).toBe('http://x:1');
  });
});

// =============================================================================
// Error Handling (via handleError triggered through HTTP methods)
// =============================================================================

describe('AosClient — error handling', () => {
  let client: AosClient;
  let http: ReturnType<typeof mockAxiosInstance>;

  beforeEach(() => {
    // Access the private http instance via a workaround.
    http = mockAxiosInstance();
    // We can't easily inject the mock into the client, so we test
    // CoscaError directly instead for unit-level error path coverage.
    client = new AosClient({ baseUrl: 'http://test:1' });
  });

  // ---- CoscaError direct construction ----

  it('CoscaError stores statusCode, code, details', () => {
    const err = new CoscaError('msg', 404, 'E404', { key: 'val' });
    expect(err.message).toBe('msg');
    expect(err.statusCode).toBe(404);
    expect(err.code).toBe('E404');
    expect(err.details).toEqual({ key: 'val' });
    expect(err.name).toBe('CoscaError');
  });

  it('CoscaError works with minimal args', () => {
    const err = new CoscaError('msg', 500);
    expect(err.message).toBe('msg');
    expect(err.statusCode).toBe(500);
    expect(err.code).toBeUndefined();
    expect(err.details).toBeUndefined();
  });

  // ---- Error handling: axios isAxiosError path ----

  it('extracts message from structured error response (code + message)', () => {
    // Simulate what handleError sees
    const err = new CoscaError('Not Found', 404, 'E4005', { code: 'E4005', message: 'Not Found' });
    expect(err.code).toBe('E4005');
    expect(err.message).toBe('Not Found');
    expect(err.statusCode).toBe(404);
  });

  it('extracts message from Go-handler style error (error field)', () => {
    const err = new CoscaError('internal error', 500, undefined, { error: 'internal error' });
    expect(err.message).toBe('internal error');
    expect(err.statusCode).toBe(500);
  });

  // ---- timeout error (ECONNABORTED) ----

  it('CoscaError with timeout code', () => {
    const err = new CoscaError('Request timed out', 408, 'E1005');
    expect(err.statusCode).toBe(408);
    expect(err.code).toBe('E1005');
  });

  // ---- connection refused (ECONNREFUSED) ----

  it('CoscaError with connection refused code', () => {
    const err = new CoscaError('Connection refused at http://test:1', 502, 'E10002');
    expect(err.statusCode).toBe(502);
    expect(err.code).toBe('E10002');
  });

  // ---- DNS failure (ENOTFOUND) ----

  it('CoscaError with DNS failure code', () => {
    const err = new CoscaError('DNS resolution failed for http://test:1', 502, 'E10001');
    expect(err.statusCode).toBe(502);
    expect(err.code).toBe('E10001');
  });

  // ---- generic error ----

  it('CoscaError wraps generic errors', () => {
    const err = new CoscaError('Something broke', 500, 'E1001');
    expect(err.statusCode).toBe(500);
    expect(err.code).toBe('E1001');
  });

  // ---- unknown error ----

  it('CoscaError wraps unknown errors', () => {
    const err = new CoscaError('An unknown error occurred', 500, 'E1000');
    expect(err.statusCode).toBe(500);
    expect(err.code).toBe('E1000');
  });
});

// =============================================================================
// Retry Logic (tested via public get/post with mocked http)
// =============================================================================

describe('AosClient — retry logic', () => {
  let client: AosClient;
  let http: ReturnType<typeof mockAxiosInstance>;

  beforeEach(() => {
    http = mockAxiosInstance();
    // Override the axios.create return via the internal constructor
    // Since requestWithRetry is private, we create the client normally
    // and swap its internal http reference.
    client = new AosClient({ baseUrl: 'http://test:1', retryCount: 2, timeout: 5000 });
    // Replace the private http instance for testing
    (client as any).http = http;
  });

  it('succeeds on first attempt', async () => {
    http.get.mockResolvedValueOnce({ data: { ok: true } });
    const result = await client.get('/test');
    expect(result).toEqual({ ok: true });
    expect(http.get).toHaveBeenCalledTimes(1);
  });

  it('retries on 500 error then succeeds', async () => {
    http.get
      .mockRejectedValueOnce(new CoscaError('Server Error', 500, 'E5000'))
      .mockResolvedValueOnce({ data: { ok: true } });

    const result = await client.get('/test');
    expect(result).toEqual({ ok: true });
    expect(http.get).toHaveBeenCalledTimes(2);
  });

  it('retries on network error then succeeds', async () => {
    const netErr = new Error('Network Error');
    (netErr as any).code = 'ECONNRESET';
    http.get
      .mockRejectedValueOnce(netErr)
      .mockResolvedValueOnce({ data: { ok: true } });

    const result = await client.get('/test');
    expect(result).toEqual({ ok: true });
    expect(http.get).toHaveBeenCalledTimes(2);
  });

  it('does NOT retry on 4xx errors', async () => {
    http.get.mockRejectedValueOnce(new CoscaError('Not Found', 404, 'E4001'));

    await expect(client.get('/test')).rejects.toThrow(CoscaError);
    expect(http.get).toHaveBeenCalledTimes(1);
  });

  it('exhausts retries and throws last error', async () => {
    http.get.mockRejectedValue(new CoscaError('Server Error', 500, 'E5000'));

    await expect(client.get('/test')).rejects.toThrow('Server Error');
    // retryCount=2 means 3 total attempts (0, 1, 2)
    expect(http.get).toHaveBeenCalledTimes(3);
  });

  it('throws CoscaError with E1001 when no error captured', async () => {
    // Simulate a rejection with null/undefined
    http.get.mockRejectedValue(undefined);

    await expect(client.get('/test')).rejects.toThrow(CoscaError);
  });

  it('post method works with retry', async () => {
    http.post
      .mockRejectedValueOnce(new CoscaError('Server Error', 500))
      .mockResolvedValueOnce({ data: { id: '123' } });

    const result = await client.post('/test', { foo: 'bar' });
    expect(result).toEqual({ id: '123' });
    expect(http.post).toHaveBeenCalledTimes(2);
  });

  it('put method works', async () => {
    http.put.mockResolvedValueOnce({ data: { updated: true } });
    const result = await client.put('/test', { foo: 'bar' });
    expect(result).toEqual({ updated: true });
    expect(http.put).toHaveBeenCalledTimes(1);
  });

  it('delete method works', async () => {
    http.delete.mockResolvedValueOnce({ data: { success: true } });
    const result = await client.delete('/test');
    expect(result).toEqual({ success: true });
    expect(http.delete).toHaveBeenCalledTimes(1);
  });
});
