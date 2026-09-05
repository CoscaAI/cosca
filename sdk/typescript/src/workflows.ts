// SPDX-License-Identifier: MIT
// Copyright © 2024 Cosca Contributors

import { AxiosInstance } from 'axios';
import { Workflow, WorkflowResult, AosClientConfig } from './types';

/**
 * WorkflowsAPI provides methods for discovering, inspecting, and executing
 * Cosca workflows.
 *
 * Workflows define repeatable processes with defined steps that agents
 * carry out.
 */
export class WorkflowsAPI {
  /**
   * @internal
   */
  constructor(
    private readonly http: AxiosInstance,
    _config: Required<AosClientConfig>,
  ) {}

  /**
   * Returns all available workflows registered in the runtime.
   *
   * @returns Array of workflows.
   */
  async list(): Promise<Workflow[]> {
    const { data } = await this.http.get<Workflow[]>('/v1/workflows');
    return data;
  }

  /**
   * Finds workflows matching the given query string.
   *
   * The search is case-insensitive and matches against name and description
   * fields.
   *
   * @param query - Search query string.
   * @returns Array of matching workflows.
   */
  async search(query: string): Promise<Workflow[]> {
    if (!query) {
      throw new Error('Search query is required');
    }

    const { data } = await this.http.get<Workflow[]>(
      `/v1/workflows/search?q=${encodeURIComponent(query)}`,
    );
    return data;
  }

  /**
   * Returns a single workflow by name (case-insensitive).
   *
   * @param name - The workflow name to retrieve.
   * @returns The matching workflow.
   * @throws If no workflow matches the given name.
   */
  async get(name: string): Promise<Workflow> {
    if (!name) {
      throw new Error('Workflow name is required');
    }

    const { data } = await this.http.get<Workflow>(
      `/v1/workflows/${encodeURIComponent(name)}`,
    );
    return data;
  }

  /**
   * Executes a workflow by name.
   *
   * The workflow's steps are executed sequentially, and the result contains
   * the completion status, timing, and step counts.
   *
   * @param name - The workflow name to execute.
   * @returns Workflow execution result.
   */
  async run(name: string): Promise<WorkflowResult> {
    if (!name) {
      throw new Error('Workflow name is required');
    }

    const { data } = await this.http.post<WorkflowResult>(
      `/v1/workflows/${encodeURIComponent(name)}/run`,
    );
    return data;
  }
}
