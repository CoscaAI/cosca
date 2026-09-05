import { describe, it, expect, beforeEach, vi } from 'vitest';
import { AxiosInstance } from 'axios';
import { SkillsAPI } from '../skills';
import { AosClientConfig, Skill } from '../types';

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

function mockSkill(overrides?: Partial<Skill>): Skill {
  return {
    name: 'echo',
    description: 'Echoes input back',
    version: '1.0.0',
    category: 'utility',
    instructions: 'Use echo to repeat text',
    tools: [{ name: 'echo', description: 'Repeat text' }],
    source: '/skills/echo.md',
    ...overrides,
  };
}

// =============================================================================
// SkillsAPI
// =============================================================================

describe('SkillsAPI', () => {
  let http: ReturnType<typeof mockHttp>;
  let api: SkillsAPI;

  beforeEach(() => {
    http = mockHttp();
    api = new SkillsAPI(http, mockConfig());
  });

  // ---- list() ----

  describe('list()', () => {
    it('returns all skills', async () => {
      const skills = [mockSkill(), mockSkill({ name: 'search', description: 'Search knowledge', category: 'knowledge' })];
      (http.get as any).mockResolvedValue({ data: skills });

      const result = await api.list();

      expect(result).toEqual(skills);
      expect(result).toHaveLength(2);
      expect(http.get).toHaveBeenCalledWith('/v1/skills');
    });

    it('returns empty array when no skills', async () => {
      (http.get as any).mockResolvedValue({ data: [] });

      const result = await api.list();
      expect(result).toEqual([]);
    });
  });

  // ---- search() ----

  describe('search()', () => {
    it('finds skills by query', async () => {
      const skills = [mockSkill({ name: 'search', category: 'knowledge' })];
      (http.get as any).mockResolvedValue({ data: skills });

      const result = await api.search('search');

      expect(result).toEqual(skills);
      expect(http.get).toHaveBeenCalledWith('/v1/skills/search?q=search');
    });

    it('throws on empty query', async () => {
      await expect(api.search('')).rejects.toThrow('Search query is required');
    });
  });

  // ---- get() ----

  describe('get()', () => {
    it('returns skill by name', async () => {
      const skill = mockSkill();
      (http.get as any).mockResolvedValue({ data: skill });

      const result = await api.get('echo');

      expect(result).toEqual(skill);
      expect(http.get).toHaveBeenCalledWith('/v1/skills/echo');
    });

    it('URL-encodes skill name', async () => {
      (http.get as any).mockResolvedValue({ data: mockSkill() });

      await api.get('my skill');

      expect(http.get).toHaveBeenCalledWith('/v1/skills/my%20skill');
    });

    it('throws on empty name', async () => {
      await expect(api.get('')).rejects.toThrow('Skill name is required');
    });
  });

  // ---- install() ----

  describe('install()', () => {
    it('installs a skill from source', async () => {
      const installed = mockSkill({ name: 'new-skill', source: '/path/to/skill.md' });
      (http.post as any).mockResolvedValue({ data: installed });

      const result = await api.install('new-skill', '/path/to/skill.md');

      expect(result).toEqual(installed);
      expect(http.post).toHaveBeenCalledWith('/v1/skills/new-skill/install', { source: '/path/to/skill.md' });
    });

    it('installs from URL', async () => {
      (http.post as any).mockResolvedValue({ data: mockSkill() });

      await api.install('remote-skill', 'https://example.com/skill.md');

      expect(http.post).toHaveBeenCalledWith('/v1/skills/remote-skill/install', {
        source: 'https://example.com/skill.md',
      });
    });

    it('throws on empty name', async () => {
      await expect(api.install('', '/path')).rejects.toThrow('Skill name is required');
    });

    it('throws on empty source', async () => {
      await expect(api.install('name', '')).rejects.toThrow('Source is required for installation');
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

    it('propagates HTTP errors on install', async () => {
      (http.post as any).mockRejectedValue(new Error('Already Exists'));
      await expect(api.install('name', '/path')).rejects.toThrow('Already Exists');
    });
  });
});
