// SPDX-License-Identifier: MIT
// Copyright © 2024 Cosca Contributors

import { AxiosInstance } from 'axios';
import { ProjectInfo, WorkspaceInfo, EditorInfo, DiscoveryReport } from './types';
import { AosClientConfig } from './types';

/**
 * DiscoveryAPI provides methods for auto-detecting information about the
 * current project, workspace, and editor environment.
 *
 * **Note:** Discovery endpoints (/v1/discovery/*) do not yet have handler
 * implementations in the REST API.  These methods are stubs that will
 * activate once the backend is available.
 */
export class DiscoveryAPI {
  /**
   * @internal
   */
  constructor(
    private readonly http: AxiosInstance,
    _config: Required<AosClientConfig>,
  ) {}

  /**
   * Detects and returns information about the current project by analysing
   * the file system for project markers (go.mod, package.json, etc.).
   *
   * @returns Detected project information.
   */
  async project(): Promise<ProjectInfo> {
    const { data } = await this.http.get<ProjectInfo>('/v1/discovery/project');
    return data;
  }

  /**
   * Detects and returns information about the workspace environment
   * including OS, shell, terminal, and git state.
   *
   * @returns Detected workspace information.
   */
  async workspace(): Promise<WorkspaceInfo> {
    const { data } = await this.http.get<WorkspaceInfo>('/v1/discovery/workspace');
    return data;
  }

  /**
   * Detects and returns information about the connected editor or IDE,
   * including name, version, capabilities, and connection state.
   *
   * @returns Detected editor information.
   */
  async editor(): Promise<EditorInfo> {
    const { data } = await this.http.get<EditorInfo>('/v1/discovery/editor');
    return data;
  }

  /**
   * Runs all discovery methods and returns a complete report containing
   * project, workspace, and editor information.
   *
   * @returns Complete discovery report.
   */
  async discover(): Promise<DiscoveryReport> {
    const { data } = await this.http.get<DiscoveryReport>('/v1/discovery/all');
    return data;
  }
}
