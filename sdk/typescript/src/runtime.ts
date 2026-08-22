// SPDX-License-Identifier: MIT
// Copyright © 2024 Cosca Contributors

import { AxiosInstance } from 'axios';
import {
  RuntimeStatusResponse,
  RuntimeHealthResponse,
  AosClientConfig,
} from './types';

/**
 * RuntimeAPI provides methods for querying the Cosca Runtime status and health.
 *
 * Supports retrieving the current runtime state (version, uptime, component
 * statuses) and performing lightweight health checks.
 */
export class RuntimeAPI {
  /**
   * @internal
   */
  constructor(
    private readonly http: AxiosInstance,
    _config: Required<AosClientConfig>,
  ) {}

  /**
   * Returns the current runtime status including state, health, uptime,
   * version, and per-component information.
   *
   * @returns Current runtime status.
   */
  async status(): Promise<RuntimeStatusResponse> {
    const { data } = await this.http.get<RuntimeStatusResponse>('/v1/status');
    return data;
  }

  /**
   * Performs a lightweight health check on the runtime.
   *
   * Returns a boolean `healthy` flag and an optional list of non-critical
   * warnings.
   *
   * @returns Health check result.
   */
  async health(): Promise<RuntimeHealthResponse> {
    const { data } = await this.http.get<RuntimeHealthResponse>('/v1/health');
    return data;
  }
}
