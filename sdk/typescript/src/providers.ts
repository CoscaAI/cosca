// SPDX-License-Identifier: MIT
// Copyright © 2024 Cosca Contributors

import { AxiosInstance } from 'axios';
import { Provider, TestResult, ProviderStatus, AosClientConfig } from './types';

/**
 * ProvidersAPI provides methods for managing AI/LLM providers in the Cosca
 * Runtime.
 *
 * Supports listing, inspecting, connectivity-testing, and activating
 * providers such as OpenAI, Anthropic, Ollama, etc.
 */
export class ProvidersAPI {
  /**
   * @internal
   */
  constructor(
    private readonly http: AxiosInstance,
    _config: Required<AosClientConfig>,
  ) {}

  /**
   * Returns all available AI providers with their current status and
   * configuration.
   *
   * @returns Array of providers.
   */
  async list(): Promise<Provider[]> {
    const { data } = await this.http.get<Provider[]>('/v1/providers');
    return data;
  }

  /**
   * Returns detailed information about a specific provider by name.
   *
   * @param name - The provider name to retrieve.
   * @returns The matching provider.
   * @throws If no provider matches the given name.
   */
  async get(name: string): Promise<Provider> {
    if (!name) {
      throw new Error('Provider name is required');
    }

    const { data } = await this.http.get<Provider>(
      `/v1/providers/${encodeURIComponent(name)}`,
    );
    return data;
  }

  /**
   * Performs a connectivity test against the specified provider.
   *
   * Verifies the provider is reachable and returns timing and status
   * information.
   *
   * @param name - The provider name to test.
   * @returns Test result with response time, model, and status.
   */
  async test(name: string): Promise<TestResult> {
    if (!name) {
      throw new Error('Provider name is required');
    }

    const { data } = await this.http.post<TestResult>(
      `/v1/providers/${encodeURIComponent(name)}/test`,
    );
    return data;
  }

  /**
   * Sets the specified provider (and optional model) as the active AI
   * provider for the runtime.
   *
   * @param name  - The provider to activate.
   * @param model - Optional model name to set as default.
   * @returns Updated provider subsystem status.
   */
  async setActive(name: string, model?: string): Promise<ProviderStatus> {
    if (!name) {
      throw new Error('Provider name is required');
    }

    const { data } = await this.http.put<ProviderStatus>(
      '/v1/providers/active',
      { provider: name, model: model ?? '' },
    );
    return data;
  }
}
