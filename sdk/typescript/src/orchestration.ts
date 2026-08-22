// SPDX-License-Identifier: MIT
// Copyright © 2024 Cosca Contributors

import { AxiosInstance } from 'axios';
import { RunResult, RunOptions, StreamEvent, AosClientConfig } from './types';

/**
 * OrchestrationAPI provides methods for AI orchestration — executing
 * prompts through AI agents and streaming responses in real time via
 * Server-Sent Events (SSE).
 */
export class OrchestrationAPI {
  /**
   * @internal
   */
  constructor(
    private readonly http: AxiosInstance,
    _config: Required<AosClientConfig>,
  ) {}

  /**
   * Executes a prompt through the AI orchestration engine and returns the
   * complete response synchronously.
   *
   * @param prompt - The prompt text to send.
   * @param opts   - Optional agent and provider selection.
   * @returns The run result with response text, agent, skills, and timing.
   */
  async run(prompt: string, opts?: RunOptions): Promise<RunResult> {
    if (!prompt) {
      throw new Error('Prompt is required');
    }

    const body: Record<string, unknown> = {
      prompt,
      ...(opts?.agent ? { agent: opts.agent } : {}),
      ...(opts?.provider ? { provider: opts.provider } : {}),
    };

    const { data } = await this.http.post<RunResult>('/v1/run', body);
    return data;
  }

  /**
   * Executes a prompt through the AI orchestration engine and streams the
   * response back as Server-Sent Events.
   *
   * Returns an async generator that yields {@link StreamEvent} objects as
   * they arrive. The generator completes when the stream ends.
   *
   * @param prompt - The prompt text to send.
   * @param opts   - Optional agent and provider selection.
   * @returns An async generator yielding stream events.
   *
   * @example
   * ```typescript
   * for await (const event of client.orchestration.stream('Hello')) {
   *   if (event.type === 'response') {
   *     process.stdout.write(event.content);
   *   }
   * }
   * ```
   */
  async *stream(
    prompt: string,
    opts?: RunOptions,
  ): AsyncGenerator<StreamEvent, void, unknown> {
    if (!prompt) {
      throw new Error('Prompt is required');
    }

    const body: Record<string, unknown> = {
      prompt,
      ...(opts?.agent ? { agent: opts.agent } : {}),
      ...(opts?.provider ? { provider: opts.provider } : {}),
    };

    const response = await this.http.post('/v1/run/stream', body, {
      responseType: 'stream',
      headers: { Accept: 'text/event-stream' },
    });

    const stream = response.data;
    let buffer = '';

    for await (const chunk of stream) {
      buffer += chunk.toString();

      const lines = buffer.split('\n');
      // Keep the last (potentially incomplete) line in the buffer.
      buffer = lines.pop() ?? '';

      for (const line of lines) {
        const trimmed = line.trim();
        if (!trimmed || !trimmed.startsWith('data: ')) {
          continue;
        }

        const data = trimmed.slice(6); // Strip "data: " prefix.
        if (data === '[DONE]') {
          return;
        }

        try {
          const event: StreamEvent = JSON.parse(data);
          yield event;
        } catch {
          // Skip unparseable lines.
        }
      }
    }

    // Process any remaining buffered data.
    if (buffer.trim().startsWith('data: ')) {
      const data = buffer.trim().slice(6);
      if (data !== '[DONE]') {
        try {
          const event: StreamEvent = JSON.parse(data);
          yield event;
        } catch {
          // Skip unparseable data.
        }
      }
    }
  }
}
