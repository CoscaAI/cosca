// SPDX-License-Identifier: MIT
// Copyright © 2024 Cosca Contributors

import { AxiosInstance } from 'axios';
import { PluginInfo } from './types';
import { AosClientConfig } from './types';

/**
 * PluginsAPI provides methods for managing Cosca Runtime plugins.
 *
 * **Note:** Plugin endpoints (/v1/plugins/*) do not yet have handler
 * implementations in the REST API.  These methods are stubs that will
 * activate once the backend is available.
 */
export class PluginsAPI {
  /**
   * @internal
   */
  constructor(
    private readonly http: AxiosInstance,
    _config: Required<AosClientConfig>,
  ) {}

  /**
   * Installs a plugin from a source URI or registry package name.
   *
   * @param source - Local file path, URL to a plugin archive, or registry
   *                 package name.
   */
  async install(source: string): Promise<void> {
    if (!source) {
      throw new Error('Plugin source is required');
    }

    await this.http.post('/v1/plugins/install', { source });
  }

  /**
   * Uninstalls a previously installed plugin by its identifier.
   *
   * @param id - The plugin ID to uninstall.
   */
  async uninstall(id: string): Promise<void> {
    if (!id) {
      throw new Error('Plugin ID is required');
    }

    await this.http.delete(`/v1/plugins/${id}`);
  }

  /**
   * Lists all plugins currently installed in the runtime.
   *
   * @returns Array of plugin information.
   */
  async list(): Promise<PluginInfo[]> {
    const { data } = await this.http.get<{ plugins: PluginInfo[] }>(
      '/v1/plugins',
    );
    return data.plugins;
  }

  /**
   * Returns detailed information about a specific plugin by ID.
   *
   * @param id - The plugin ID.
   * @returns Plugin information.
   */
  async get(id: string): Promise<PluginInfo> {
    if (!id) {
      throw new Error('Plugin ID is required');
    }

    const { data } = await this.http.get<PluginInfo>(`/v1/plugins/${id}`);
    return data;
  }
}
