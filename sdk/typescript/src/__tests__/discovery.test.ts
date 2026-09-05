import { describe, it, expect, beforeEach, vi } from 'vitest';
import { AxiosInstance } from 'axios';
import { DiscoveryAPI } from '../discovery';
import { AosClientConfig, ProjectInfo, WorkspaceInfo, EditorInfo, DiscoveryReport } from '../types';

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

function mockProject(overrides?: Partial<ProjectInfo>): ProjectInfo {
  return {
    name: 'my-project',
    path: '/home/user/projects/my-project',
    language: 'TypeScript',
    framework: 'Next.js',
    version: '1.0.0',
    buildSystem: 'npm',
    dependencies: ['react', 'next'],
    entrypoint: 'src/index.ts',
    configFiles: ['package.json', 'tsconfig.json'],
    hasTests: true,
    hasDocker: true,
    hasCI: false,
    confidence: 0.95,
    ...overrides,
  };
}

function mockWorkspace(overrides?: Partial<WorkspaceInfo>): WorkspaceInfo {
  return {
    root: '/home/user/projects/my-project',
    ide: 'vscode',
    shell: '/bin/zsh',
    terminal: 'gnome-terminal',
    os: 'linux',
    arch: 'x64',
    homeDir: '/home/user',
    tempDir: '/tmp',
    gitRoot: '/home/user/projects/my-project',
    gitBranch: 'main',
    ...overrides,
  };
}

function mockEditor(overrides?: Partial<EditorInfo>): EditorInfo {
  return {
    name: 'vscode',
    version: '1.90.0',
    path: '/usr/bin/code',
    pid: 1234,
    connected: true,
    capabilities: ['completions', 'diagnostics'],
    extensions: ['ms-python.python', 'dbaeumer.vscode-eslint'],
    language: 'typescript',
    scheme: 'file',
    ...overrides,
  };
}

// =============================================================================
// DiscoveryAPI
// =============================================================================

describe('DiscoveryAPI', () => {
  let http: ReturnType<typeof mockHttp>;
  let api: DiscoveryAPI;

  beforeEach(() => {
    http = mockHttp();
    api = new DiscoveryAPI(http, mockConfig());
  });

  // ---- project() ----

  describe('project()', () => {
    it('detects project information', async () => {
      const project = mockProject();
      (http.get as any).mockResolvedValue({ data: project });

      const result = await api.project();

      expect(result).toEqual(project);
      expect(http.get).toHaveBeenCalledWith('/v1/discovery/project');
    });

    it('handles unknown project', async () => {
      const project = mockProject({ language: 'Unknown', confidence: 0.1 });
      (http.get as any).mockResolvedValue({ data: project });

      const result = await api.project();
      expect(result.confidence).toBe(0.1);
      expect(result.language).toBe('Unknown');
    });
  });

  // ---- workspace() ----

  describe('workspace()', () => {
    it('detects workspace information', async () => {
      const workspace = mockWorkspace();
      (http.get as any).mockResolvedValue({ data: workspace });

      const result = await api.workspace();

      expect(result).toEqual(workspace);
      expect(http.get).toHaveBeenCalledWith('/v1/discovery/workspace');
    });
  });

  // ---- editor() ----

  describe('editor()', () => {
    it('detects editor information', async () => {
      const editor = mockEditor();
      (http.get as any).mockResolvedValue({ data: editor });

      const result = await api.editor();

      expect(result).toEqual(editor);
      expect(http.get).toHaveBeenCalledWith('/v1/discovery/editor');
    });

    it('handles editor not connected', async () => {
      const editor = mockEditor({ connected: false, name: 'unknown' });
      (http.get as any).mockResolvedValue({ data: editor });

      const result = await api.editor();
      expect(result.connected).toBe(false);
      expect(result.name).toBe('unknown');
    });
  });

  // ---- discover() ----

  describe('discover()', () => {
    it('returns complete discovery report', async () => {
      const report: DiscoveryReport = {
        project: mockProject(),
        workspace: mockWorkspace(),
        editor: mockEditor(),
        capturedAt: '2025-01-01T00:00:00Z',
      };
      (http.get as any).mockResolvedValue({ data: report });

      const result = await api.discover();

      expect(result).toEqual(report);
      expect(result.project.name).toBe('my-project');
      expect(result.workspace.os).toBe('linux');
      expect(result.editor.name).toBe('vscode');
      expect(http.get).toHaveBeenCalledWith('/v1/discovery/all');
    });

    it('handles complete report with minimal data', async () => {
      const report: DiscoveryReport = {
        project: mockProject({ hasTests: false, hasDocker: false }),
        workspace: mockWorkspace({ ide: undefined, gitRoot: undefined }),
        editor: mockEditor({ connected: false, pid: undefined }),
        capturedAt: '2025-01-01T00:00:00Z',
      };
      (http.get as any).mockResolvedValue({ data: report });

      const result = await api.discover();
      expect(result.project.hasTests).toBe(false);
      expect(result.workspace.ide).toBeUndefined();
      expect(result.editor.connected).toBe(false);
    });
  });

  // ---- error propagation ----

  describe('error propagation', () => {
    it('propagates HTTP errors on project', async () => {
      (http.get as any).mockRejectedValue(new Error('Not Found'));
      await expect(api.project()).rejects.toThrow('Not Found');
    });

    it('propagates HTTP errors on workspace', async () => {
      (http.get as any).mockRejectedValue(new Error('Timeout'));
      await expect(api.workspace()).rejects.toThrow('Timeout');
    });

    it('propagates HTTP errors on editor', async () => {
      (http.get as any).mockRejectedValue(new Error('Service Unavailable'));
      await expect(api.editor()).rejects.toThrow('Service Unavailable');
    });

    it('propagates HTTP errors on discover', async () => {
      (http.get as any).mockRejectedValue(new Error('Internal Error'));
      await expect(api.discover()).rejects.toThrow('Internal Error');
    });
  });
});
