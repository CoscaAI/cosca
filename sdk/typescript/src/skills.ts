// SPDX-License-Identifier: MIT
// Copyright © 2024 Cosca Contributors

import { AxiosInstance } from 'axios';
import { Skill, AosClientConfig } from './types';

/**
 * SkillsAPI provides methods for discovering, inspecting, and installing
 * Cosca skills.
 *
 * Skills define specialised capabilities that agents can use to perform
 * their tasks.
 */
export class SkillsAPI {
  /**
   * @internal
   */
  constructor(
    private readonly http: AxiosInstance,
    _config: Required<AosClientConfig>,
  ) {}

  /**
   * Returns all available skills registered in the runtime.
   *
   * @returns Array of skills.
   */
  async list(): Promise<Skill[]> {
    const { data } = await this.http.get<Skill[]>('/v1/skills');
    return data;
  }

  /**
   * Finds skills matching the given query string.
   *
   * The search is case-insensitive and matches against name, description,
   * and category fields.
   *
   * @param query - Search query string.
   * @returns Array of matching skills.
   */
  async search(query: string): Promise<Skill[]> {
    if (!query) {
      throw new Error('Search query is required');
    }

    const { data } = await this.http.get<Skill[]>(
      `/v1/skills/search?q=${encodeURIComponent(query)}`,
    );
    return data;
  }

  /**
   * Returns a single skill by name (case-insensitive).
   *
   * @param name - The skill name to retrieve.
   * @returns The matching skill.
   * @throws If no skill matches the given name.
   */
  async get(name: string): Promise<Skill> {
    if (!name) {
      throw new Error('Skill name is required');
    }

    const { data } = await this.http.get<Skill>(
      `/v1/skills/${encodeURIComponent(name)}`,
    );
    return data;
  }

  /**
   * Installs a skill from a source (file path or URL).
   *
   * The skill is parsed, registered, and persisted for future use.
   *
   * @param name   - The skill name to register under.
   * @param source - File path or URL to install from.
   * @returns The installed skill.
   */
  async install(name: string, source: string): Promise<Skill> {
    if (!name) {
      throw new Error('Skill name is required');
    }
    if (!source) {
      throw new Error('Source is required for installation');
    }

    const { data } = await this.http.post<Skill>(
      `/v1/skills/${encodeURIComponent(name)}/install`,
      { source },
    );
    return data;
  }
}
