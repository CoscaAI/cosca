// SPDX-License-Identifier: MIT
// Copyright © 2024 Cosca Contributors

import { AxiosInstance } from 'axios';
import {
  MemoryRecord,
  MemoryStoreRequest,
  MemoryStoreResponse,
  MemorySearchResponse,
  MemoryGetResponse,
  MemoryDeleteResponse,
  MemoryPromoteResponse,
  MemoryStatsResponse,
  AosClientConfig,
} from './types';

/**
 * MemoryAPI provides methods for interacting with the Cosca Memory Engine.
 *
 * Agents can store, retrieve, search, promote, and inspect memory records
 * across different memory layers.
 */
export class MemoryAPI {
  /**
   * @internal
   */
  constructor(
    private readonly http: AxiosInstance,
    _config: Required<AosClientConfig>,
  ) {}

  // ------------------------------------------------------------------
  // Store
  // ------------------------------------------------------------------

  /**
   * Stores a new memory record in the specified memory layer.
   *
   * @param request - The memory record data to persist.
   * @returns The created record identifier and timestamp.
   */
  async store(request: MemoryStoreRequest): Promise<MemoryStoreResponse> {
    if (!request.content) {
      throw new Error('Memory content is required');
    }

    const { data } = await this.http.post<MemoryStoreResponse>(
      '/v1/memory/store',
      request,
    );
    return data;
  }

  // ------------------------------------------------------------------
  // Retrieve
  // ------------------------------------------------------------------

  /**
   * Retrieves a single memory record by ID and layer.
   *
   * @param id    - The memory record ID.
   * @param layer - The memory layer (required).
   * @returns The memory record.
   */
  async retrieve(id: string, layer: string): Promise<MemoryRecord> {
    if (!id) {
      throw new Error('Memory record ID is required');
    }
    if (!layer) {
      throw new Error('Memory layer is required');
    }

    const { data } = await this.http.get<MemoryGetResponse>(
      '/v1/memory/get',
      { params: { id, layer } },
    );
    return data.record;
  }

  // ------------------------------------------------------------------
  // Search
  // ------------------------------------------------------------------

  /**
   * Searches memory records across layers.
   *
   * @param query  - Search query string.
   * @param opts   - Optional filters (types, layers, limit).
   * @returns Search results.
   */
  async search(
    query: string,
    opts?: {
      limit?: number;
      types?: string[];
      layers?: string[];
    },
  ): Promise<MemorySearchResponse> {
    if (!query) {
      throw new Error('Search query is required');
    }

    const params: Record<string, unknown> = { query };
    if (opts?.limit !== undefined) params.limit = opts.limit;
    if (opts?.types?.length) params.types = opts.types;
    if (opts?.layers?.length) params.layers = opts.layers;

    const { data } = await this.http.get<MemorySearchResponse>(
      '/v1/memory/search',
      { params },
    );
    return data;
  }

  // ------------------------------------------------------------------
  // Delete
  // ------------------------------------------------------------------

  /**
   * Deletes a memory record by ID and optional layer.
   *
   * **Note:** The current engine does not fully support deletion via the
   * public API and returns HTTP 501 (Not Implemented) on attempt.
   *
   * @param id    - The memory record ID.
   * @param layer - Optional memory layer (defaults to "session").
   * @returns Deletion confirmation.
   */
  async delete(id: string, layer?: string): Promise<MemoryDeleteResponse> {
    if (!id) {
      throw new Error('Memory record ID is required');
    }

    const params: Record<string, unknown> = { id };
    if (layer) params.layer = layer;

    const { data } = await this.http.delete<MemoryDeleteResponse>(
      '/v1/memory/delete',
      { params },
    );
    return data;
  }

  // ------------------------------------------------------------------
  // Promote
  // ------------------------------------------------------------------

  /**
   * Promotes a memory record from one layer to a higher layer.
   *
   * @param id        - The memory record ID.
   * @param fromLayer - Source layer.
   * @param toLayer   - Target layer.
   * @returns The promoted record.
   */
  async promote(
    id: string,
    fromLayer: string,
    toLayer: string,
  ): Promise<MemoryPromoteResponse> {
    if (!id) {
      throw new Error('Memory record ID is required');
    }
    if (!fromLayer) {
      throw new Error('fromLayer is required');
    }
    if (!toLayer) {
      throw new Error('toLayer is required');
    }

    const { data } = await this.http.post<MemoryPromoteResponse>(
      '/v1/memory/promote',
      { id, from_layer: fromLayer, to_layer: toLayer },
    );
    return data;
  }

  // ------------------------------------------------------------------
  // Statistics
  // ------------------------------------------------------------------

  /**
   * Retrieves per-layer statistics for the memory engine.
   *
   * @returns Per-layer record counts and sizes.
   */
  async stats(): Promise<MemoryStatsResponse> {
    const { data } = await this.http.get<MemoryStatsResponse>('/v1/memory/stats');
    return data;
  }
}
