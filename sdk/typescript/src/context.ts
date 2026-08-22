// SPDX-License-Identifier: MIT
// Copyright © 2024 Cosca Contributors

import { AxiosInstance } from 'axios';
import { Context, ContextOptions } from './types';
import { AosClientConfig } from './types';

/**
 * ContextAPI provides methods for building and managing Cosca Runtime context.
 *
 * **Note:** Context endpoints (/v1/context/*) do not yet have handler
 * implementations in the REST API.  These methods are stubs that will
 * activate once the backend is available.
 */
export class ContextAPI {
  /**
   * @internal
   */
  constructor(
    private readonly http: AxiosInstance,
    _config: Required<AosClientConfig>,
  ) {}

  /**
   * Builds a runtime context from the given query and options.
   *
   * @param query - Context query string.
   * @param opts  - Optional context-building parameters.
   * @returns The constructed context.
   */
  async build(query: string, opts?: ContextOptions): Promise<Context> {
    if (!query) {
      throw new Error('Context query is required');
    }

    const { data } = await this.http.post<Context>(
      '/v1/context/build',
      { query, ...opts },
    );
    return data;
  }

  /**
   * Retrieves the currently active runtime context.
   *
   * @returns The current context.
   */
  async getCurrent(): Promise<Context> {
    const { data } = await this.http.get<Context>('/v1/context/current');
    return data;
  }

  /**
   * Clears all context entries from the current runtime session.
   */
  async clear(): Promise<void> {
    await this.http.delete('/v1/context/current');
  }
}
