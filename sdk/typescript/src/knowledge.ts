// SPDX-License-Identifier: MIT
// Copyright © 2024 Cosca Contributors

import { AxiosInstance } from 'axios';
import {
  KnowledgeSearchOptions,
  KnowledgeSearchResponse,
  KnowledgeIndexResponse,
  KnowledgeStatsResponse,
  KnowledgeSyncResponse,
  AosClientConfig,
} from './types';

/**
 * KnowledgeAPI provides methods for interacting with the Cosca Knowledge Engine.
 *
 * Supports document/directory indexing, full-text + vector + graph search,
 * statistics, and filesystem synchronisation.
 */
export class KnowledgeAPI {
  /**
   * @internal
   */
  constructor(
    private readonly http: AxiosInstance,
    _config: Required<AosClientConfig>,
  ) {}

  /**
   * Indexes a file or directory into the knowledge base.
   * The document is parsed, chunked, and embedded for search.
   *
   * @param path      - Absolute or relative path to the file or directory.
   * @param recursive - Whether to recursively index subdirectories (defaults to false).
   * @returns Indexing result summary.
   */
  async index(path: string, recursive = false): Promise<KnowledgeIndexResponse> {
    if (!path) {
      throw new Error('Document path is required');
    }

    const { data } = await this.http.post<KnowledgeIndexResponse>(
      '/v1/knowledge/index',
      { path, recursive },
    );
    return data;
  }

  /**
   * Searches the knowledge base using the given query and options.
   *
   * The search combines full-text, vector, and optional graph matching
   * depending on the flags set in the options.
   *
   * @param query - Search query string.
   * @param opts  - Optional search parameters (limit, types, filters, etc.).
   * @returns Search response with results, total count, duration, and facets.
   */
  async search(
    query: string,
    opts?: KnowledgeSearchOptions,
  ): Promise<KnowledgeSearchResponse> {
    return this._search(query, opts);
  }

  /**
   * Performs a full-text-only search (vector and graph disabled).
   *
   * @param query - Search query string.
   * @param opts  - Additional search parameters.
   */
  async searchFts(
    query: string,
    opts?: Omit<KnowledgeSearchOptions, 'enableFts' | 'enableVector' | 'enableGraph'>,
  ): Promise<KnowledgeSearchResponse> {
    return this._search(query, {
      ...opts,
      enableFts: true,
      enableVector: false,
      enableGraph: false,
    });
  }

  /**
   * Performs a vector/semantic-only search (FTS and graph disabled).
   *
   * @param query - Search query string.
   * @param opts  - Additional search parameters.
   */
  async searchVector(
    query: string,
    opts?: Omit<KnowledgeSearchOptions, 'enableFts' | 'enableVector' | 'enableGraph'>,
  ): Promise<KnowledgeSearchResponse> {
    return this._search(query, {
      ...opts,
      enableFts: false,
      enableVector: true,
      enableGraph: false,
    });
  }

  /**
   * Performs a hybrid search combining FTS, vector, and optionally graph.
   *
   * @param query - Search query string.
   * @param opts  - Additional search parameters.
   */
  async searchHybrid(
    query: string,
    opts?: KnowledgeSearchOptions,
  ): Promise<KnowledgeSearchResponse> {
    return this._search(query, {
      enableFts: true,
      enableVector: true,
      enableGraph: opts?.enableGraph ?? false,
      ...opts,
    });
  }

  // ------------------------------------------------------------------
  // Internal search helper
  // ------------------------------------------------------------------

  private async _search(
    query: string,
    opts?: KnowledgeSearchOptions,
  ): Promise<KnowledgeSearchResponse> {
    if (!query) {
      throw new Error('Search query is required');
    }

    const body: Record<string, unknown> = { query };

    if (opts) {
      if (opts.limit !== undefined) body.limit = opts.limit;
      if (opts.offset !== undefined) body.offset = opts.offset;
      if (opts.types?.length) body.types = opts.types;
      if (opts.pathFilter) body.path_filter = opts.pathFilter;
      if (opts.minScore !== undefined) body.min_score = opts.minScore;
      if (opts.enableFts !== undefined) body.enable_fts = opts.enableFts;
      if (opts.enableVector !== undefined) body.enable_vector = opts.enableVector;
      if (opts.enableGraph !== undefined) body.enable_graph = opts.enableGraph;
      if (opts.enableFacets !== undefined) body.enable_facets = opts.enableFacets;
      if (opts.tags && Object.keys(opts.tags).length > 0) body.tags = opts.tags;
    }

    const { data } = await this.http.post<KnowledgeSearchResponse>(
      '/v1/knowledge/search',
      body,
    );
    return data;
  }

  /**
   * Searches for results of a specific type.
   *
   * Convenience wrapper around `search` that sets the `types` filter.
   *
   * @param resultType - Entity type to scope the search (e.g., "agent", "skill").
   * @param query      - Search query string.
   * @param opts       - Additional search parameters.
   */
  async searchByType(
    resultType: string,
    query: string,
    opts?: Omit<KnowledgeSearchOptions, 'types'>,
  ): Promise<KnowledgeSearchResponse> {
    if (!resultType) {
      throw new Error('Result type is required');
    }
    return this._search(query, { ...opts, types: [resultType] });
  }

  /**
   * Retrieves statistics about the knowledge engine.
   *
   * @returns Knowledge engine statistics.
   */
  async stats(): Promise<KnowledgeStatsResponse> {
    const { data } = await this.http.get<KnowledgeStatsResponse>('/v1/knowledge/stats');
    return data;
  }

  /**
   * Synchronises the knowledge index with the filesystem.
   *
   * Detects added, removed, and modified files and updates the index
   * accordingly.
   *
   * @returns Sync result summary.
   */
  async sync(): Promise<KnowledgeSyncResponse> {
    const { data } = await this.http.post<KnowledgeSyncResponse>('/v1/knowledge/sync');
    return data;
  }
}
