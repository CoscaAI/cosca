// SPDX-License-Identifier: MIT
// Copyright © 2024 Cosca Contributors

import { AxiosInstance } from 'axios';
import { Agent, AosClientConfig } from './types';

/**
 * AgentsAPI provides methods for discovering and inspecting Cosca agents.
 *
 * Agents are specialised AI assistants that perform specific tasks within
 * the Cosca ecosystem.
 */
export class AgentsAPI {
  /**
   * @internal
   */
  constructor(
    private readonly http: AxiosInstance,
    _config: Required<AosClientConfig>,
  ) {}

  /**
   * Returns all available agents registered in the runtime.
   *
   * @returns Array of agents.
   */
  async list(): Promise<Agent[]> {
    const { data } = await this.http.get<Agent[]>('/v1/agents');
    return data;
  }

  /**
   * Finds agents matching the given query string.
   *
   * The search is case-insensitive and matches against name, role,
   * department, and description fields.
   *
   * @param query - Search query string.
   * @returns Array of matching agents.
   */
  async search(query: string): Promise<Agent[]> {
    if (!query) {
      throw new Error('Search query is required');
    }

    const { data } = await this.http.get<Agent[]>(
      `/v1/agents/search?q=${encodeURIComponent(query)}`,
    );
    return data;
  }

  /**
   * Returns a single agent by name (case-insensitive).
   *
   * @param name - The agent name to retrieve.
   * @returns The matching agent.
   * @throws If no agent matches the given name.
   */
  async get(name: string): Promise<Agent> {
    if (!name) {
      throw new Error('Agent name is required');
    }

    const { data } = await this.http.get<Agent>(
      `/v1/agents/${encodeURIComponent(name)}`,
    );
    return data;
  }
}
