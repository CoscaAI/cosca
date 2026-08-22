import { describe, it, expect, beforeEach, vi } from 'vitest';
import { AxiosInstance } from 'axios';
import { OrchestrationAPI } from '../orchestration';
import { RunResult, StreamEvent, AosClientConfig } from '../types';

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
// OrchestrationAPI
// =============================================================================

describe('OrchestrationAPI', () => {
  let http: ReturnType<typeof mockHttp>;
  let api: OrchestrationAPI;

  beforeEach(() => {
    http = mockHttp();
    api = new OrchestrationAPI(http, mockConfig());
  });

  // ---- run() ----

  describe('run()', () => {
    it('sends prompt and returns RunResult', async () => {
      const mockResult: RunResult = {
        response: 'Hello, World!',
        agent: 'assistant',
        skillsUsed: ['echo'],
        durationMs: 42,
        memoryId: 'mem-001',
      };

      (http.post as any).mockResolvedValue({ data: mockResult });

      const result = await api.run('Hello', { agent: 'assistant' });

      expect(result).toEqual(mockResult);
      expect(http.post).toHaveBeenCalledWith('/v1/run', {
        prompt: 'Hello',
        agent: 'assistant',
      });
    });

    it('sends provider option when specified', async () => {
      const mockResult: RunResult = {
        response: 'Hi',
        agent: 'assistant',
        durationMs: 10,
        memoryId: 'mem-002',
      };

      (http.post as any).mockResolvedValue({ data: mockResult });

      await api.run('Hi', { provider: 'openai' });

      expect(http.post).toHaveBeenCalledWith('/v1/run', {
        prompt: 'Hi',
        provider: 'openai',
      });
    });

    it('throws on empty prompt', async () => {
      await expect(api.run('')).rejects.toThrow('Prompt is required');
      expect(http.post).not.toHaveBeenCalled();
    });

    it('throws on undefined prompt', async () => {
      await expect(api.run(undefined as any)).rejects.toThrow('Prompt is required');
      expect(http.post).not.toHaveBeenCalled();
    });

    it('works without options', async () => {
      const mockResult: RunResult = {
        response: 'OK',
        agent: 'default',
        durationMs: 5,
        memoryId: 'mem-003',
      };
      (http.post as any).mockResolvedValue({ data: mockResult });

      const result = await api.run('test');

      expect(result).toEqual(mockResult);
      expect(http.post).toHaveBeenCalledWith('/v1/run', { prompt: 'test' });
    });

    it('propagates HTTP errors', async () => {
      (http.post as any).mockRejectedValue(new Error('Network Error'));

      await expect(api.run('test')).rejects.toThrow('Network Error');
    });
  });

  // ---- stream() ----

  describe('stream()', () => {
    it('throws on empty prompt', async () => {
      const gen = api.stream('');
      await expect(gen.next()).rejects.toThrow('Prompt is required');
    });

    it('yields parsed SSE events', async () => {
      const events: StreamEvent[] = [
        { type: 'thinking', content: 'Let me think...' },
        { type: 'response', content: 'Here is the answer' },
        { type: 'done', content: '', durationMs: 100 },
      ];

      // Build a readable stream that emits SSE-formatted chunks
      const chunks = [
        'data: {"type":"thinking","content":"Let me think..."}\n\n',
        'data: {"type":"response","content":"Here is the answer"}\n\n',
        'data: {"type":"done","content":"","durationMs":100}\n\n',
      ];

      async function* chunkIter() {
        for (const c of chunks) {
          yield c;
        }
      }

      (http.post as any).mockResolvedValue({ data: chunkIter() });

      const gen = api.stream('test');
      const received: StreamEvent[] = [];
      for await (const ev of gen) {
        received.push(ev);
      }

      expect(received).toHaveLength(3);
      expect(received[0]).toEqual(events[0]);
      expect(received[1]).toEqual(events[1]);
      expect(received[2]).toEqual(events[2]);
    });

    it('stops on [DONE] marker', async () => {
      const chunks = [
        'data: {"type":"response","content":"partial"}\n\n',
        'data: [DONE]\n\n',
        'data: {"type":"response","content":"should-not-appear"}\n\n',
      ];

      async function* chunkIter() {
        for (const c of chunks) yield c;
      }

      (http.post as any).mockResolvedValue({ data: chunkIter() });

      const gen = api.stream('test');
      const received: StreamEvent[] = [];
      for await (const ev of gen) {
        received.push(ev);
      }

      expect(received).toHaveLength(1);
      expect(received[0].content).toBe('partial');
    });

    it('skips unparseable lines', async () => {
      const chunks = [
        'data: {"type":"thinking","content":"ok"}\n\n',
        'data: not-valid-json\n\n',
        'data: {"type":"response","content":"final"}\n\n',
      ];

      async function* chunkIter() {
        for (const c of chunks) yield c;
      }

      (http.post as any).mockResolvedValue({ data: chunkIter() });

      const gen = api.stream('test');
      const received: StreamEvent[] = [];
      for await (const ev of gen) {
        received.push(ev);
      }

      expect(received).toHaveLength(2);
      expect(received[0].content).toBe('ok');
      expect(received[1].content).toBe('final');
    });

    it('passes agent and provider options to request', async () => {
      async function* chunkIter() {
        yield 'data: [DONE]\n\n';
      }

      (http.post as any).mockResolvedValue({ data: chunkIter() });

      const gen = api.stream('test', { agent: 'coder', provider: 'anthropic' });
      for await (const _ of gen) { /* consume */ }

      expect(http.post).toHaveBeenCalledWith(
        '/v1/run/stream',
        { prompt: 'test', agent: 'coder', provider: 'anthropic' },
        { responseType: 'stream', headers: { Accept: 'text/event-stream' } },
      );
    });

    it('handles empty stream gracefully', async () => {
      async function* chunkIter() {
        // no chunks
      }

      (http.post as any).mockResolvedValue({ data: chunkIter() });

      const gen = api.stream('test');
      const received: StreamEvent[] = [];
      for await (const ev of gen) {
        received.push(ev);
      }

      expect(received).toHaveLength(0);
    });

    it('handles remaining buffer data after stream ends', async () => {
      // Simulate a chunk that is incomplete (no newline) at end
      const chunks = ['data: {"type":"response","content":"last"}'];

      async function* chunkIter() {
        for (const c of chunks) yield c;
      }

      (http.post as any).mockResolvedValue({ data: chunkIter() });

      const gen = api.stream('test');
      const received: StreamEvent[] = [];
      for await (const ev of gen) {
        received.push(ev);
      }

      expect(received).toHaveLength(1);
      expect(received[0].content).toBe('last');
    });
  });
});
