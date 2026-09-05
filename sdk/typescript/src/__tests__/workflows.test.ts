import { describe, it, expect, beforeEach, vi } from 'vitest';
import { AxiosInstance } from 'axios';
import { WorkflowsAPI } from '../workflows';
import { AosClientConfig, Workflow, WorkflowResult } from '../types';

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

function mockWorkflow(overrides?: Partial<Workflow>): Workflow {
  return {
    name: 'code-review',
    description: 'Automated code review workflow',
    version: '1.0.0',
    status: 'active',
    enabled: true,
    steps: 3,
    stepList: [
      { name: 'lint', description: 'Run linter', agent: 'coder', timeout: '30s' },
      { name: 'security', description: 'Run security scan', agent: 'security', timeout: '60s' },
      { name: 'report', description: 'Generate report', agent: 'analyst', timeout: '15s' },
    ],
    inputs: [{ name: 'repository', type: 'string', required: true }],
    outputs: [{ name: 'report', type: 'string', required: true }],
    ...overrides,
  };
}

// =============================================================================
// WorkflowsAPI
// =============================================================================

describe('WorkflowsAPI', () => {
  let http: ReturnType<typeof mockHttp>;
  let api: WorkflowsAPI;

  beforeEach(() => {
    http = mockHttp();
    api = new WorkflowsAPI(http, mockConfig());
  });

  // ---- list() ----

  describe('list()', () => {
    it('returns all workflows', async () => {
      const workflows = [mockWorkflow(), mockWorkflow({ name: 'deploy', status: 'inactive', steps: 5 })];
      (http.get as any).mockResolvedValue({ data: workflows });

      const result = await api.list();

      expect(result).toEqual(workflows);
      expect(result).toHaveLength(2);
      expect(http.get).toHaveBeenCalledWith('/v1/workflows');
    });

    it('returns empty array when no workflows', async () => {
      (http.get as any).mockResolvedValue({ data: [] });

      const result = await api.list();
      expect(result).toEqual([]);
    });
  });

  // ---- search() ----

  describe('search()', () => {
    it('finds workflows by query', async () => {
      const workflows = [mockWorkflow()];
      (http.get as any).mockResolvedValue({ data: workflows });

      const result = await api.search('code review');

      expect(result).toEqual(workflows);
      expect(http.get).toHaveBeenCalledWith('/v1/workflows/search?q=code%20review');
    });

    it('throws on empty query', async () => {
      await expect(api.search('')).rejects.toThrow('Search query is required');
    });
  });

  // ---- get() ----

  describe('get()', () => {
    it('returns workflow by name', async () => {
      const workflow = mockWorkflow();
      (http.get as any).mockResolvedValue({ data: workflow });

      const result = await api.get('code-review');

      expect(result).toEqual(workflow);
      expect(http.get).toHaveBeenCalledWith('/v1/workflows/code-review');
    });

    it('URL-encodes workflow name', async () => {
      (http.get as any).mockResolvedValue({ data: mockWorkflow() });

      await api.get('my workflow');

      expect(http.get).toHaveBeenCalledWith('/v1/workflows/my%20workflow');
    });

    it('throws on empty name', async () => {
      await expect(api.get('')).rejects.toThrow('Workflow name is required');
    });
  });

  // ---- run() ----

  describe('run()', () => {
    it('executes a workflow', async () => {
      const result: WorkflowResult = {
        status: 'completed',
        duration: '45s',
        stepsCompleted: 3,
        totalSteps: 3,
        outputs: [{ name: 'report', type: 'string', required: true }],
      };
      (http.post as any).mockResolvedValue({ data: result });

      const runResult = await api.run('code-review');

      expect(runResult).toEqual(result);
      expect(http.post).toHaveBeenCalledWith('/v1/workflows/code-review/run');
    });

    it('returns failed execution result', async () => {
      const result: WorkflowResult = {
        status: 'failed',
        duration: '10s',
        stepsCompleted: 1,
        totalSteps: 3,
      };
      (http.post as any).mockResolvedValue({ data: result });

      const runResult = await api.run('flaky-workflow');
      expect(runResult.status).toBe('failed');
      expect(runResult.stepsCompleted).toBe(1);
    });

    it('throws on empty name', async () => {
      await expect(api.run('')).rejects.toThrow('Workflow name is required');
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

    it('propagates HTTP errors on run', async () => {
      (http.post as any).mockRejectedValue(new Error('Already Running'));
      await expect(api.run('busy')).rejects.toThrow('Already Running');
    });
  });
});
