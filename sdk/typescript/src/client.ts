// SPDX-License-Identifier: MIT
// Copyright © 2024 Cosca Contributors

import axios, { AxiosInstance, AxiosRequestConfig, AxiosResponse } from 'axios';
import { AosClientConfig } from './types';
import { KnowledgeAPI } from './knowledge';
import { MemoryAPI } from './memory';
import { ContextAPI } from './context';
import { RuntimeAPI } from './runtime';
import { PluginsAPI } from './plugins';
import { DiscoveryAPI } from './discovery';
import { AgentsAPI } from './agents';
import { SkillsAPI } from './skills';
import { ProvidersAPI } from './providers';
import { WorkflowsAPI } from './workflows';
import { OrchestrationAPI } from './orchestration';

// =============================================================================
// SDK Error
// =============================================================================

/** Structured error returned by the Cosca SDK. */
export class CoscaError extends Error {
  /** HTTP status code. */
  public readonly statusCode: number;
  /** Cosca error code (e.g., "E2000", "E4001"). */
  public readonly code?: string;
  /** Additional error details. */
  public readonly details?: Record<string, unknown>;

  constructor(message: string, statusCode: number, code?: string, details?: Record<string, unknown>) {
    super(message);
    this.name = 'CoscaError';
    this.statusCode = statusCode;
    this.code = code;
    this.details = details;
  }
}

// =============================================================================
// Default Configuration
// =============================================================================

const DEFAULTS = {
  /** Default base URL used when neither env var nor config is provided. */
  baseUrl: 'http://localhost:14120',
  timeout: 30000,
  retryCount: 3,
};

/** Read the base URL from the COSCA_API_URL environment variable, if set. */
function envBaseUrl(): string | undefined {
  if (typeof process !== 'undefined' && process.env?.COSCA_API_URL) {
    return process.env.COSCA_API_URL;
  }
  return undefined;
}

// =============================================================================
// AosClient
// =============================================================================

/**
 * AosClient is the main entry point for the Cosca TypeScript SDK.
 *
 * It provides access to all sub-APIs (Knowledge, Memory, Context, Runtime,
 * Plugins, Discovery) through lazily-initialised public properties.
 *
 * The client connects to an Cosca Runtime REST API.  The base URL is resolved
 * in this order:
 *
 *  1. `COSCA_API_URL` environment variable
 *  2. `baseUrl` passed in the `AosClientConfig`
 *  3. `http://localhost:14120` (default)
 *
 * @example
 * ```typescript
 * // Auto-detect from env or use default
 * const client = new AosClient();
 *
 * // Explicit base URL
 * const client = new AosClient({
 *   baseUrl: 'http://localhost:14120',
 *   apiKey: 'my-api-key',
 * });
 *
 * const results = await client.knowledge.search('authentication', { limit: 5 });
 * const health = await client.runtime.health();
 * ```
 */
export class AosClient {
  /** The underlying Axios HTTP client. */
  private readonly http: AxiosInstance;
  private readonly config: Required<AosClientConfig>;

  /** Knowledge Base API. */
  public readonly knowledge: KnowledgeAPI;
  /** Memory API. */
  public readonly memory: MemoryAPI;
  /** Context API. */
  public readonly context: ContextAPI;
  /** Runtime API. */
  public readonly runtime: RuntimeAPI;
  /** Plugins API. */
  public readonly plugins: PluginsAPI;
  /** Discovery API. */
  public readonly discovery: DiscoveryAPI;
  /** Agents API. */
  public readonly agents: AgentsAPI;
  /** Skills API. */
  public readonly skills: SkillsAPI;
  /** Providers API. */
  public readonly providers: ProvidersAPI;
  /** Workflows API. */
  public readonly workflows: WorkflowsAPI;
  /** Orchestration API. */
  public readonly orchestration: OrchestrationAPI;

  /**
   * Creates a new Cosca SDK client.
   *
   * @param config - Client configuration (all fields optional).
   */
  constructor(config: AosClientConfig = {}) {
    // Resolve base URL: env → config → default.
    const rawBaseUrl =
      envBaseUrl() ??
      config.baseUrl ??
      DEFAULTS.baseUrl;

    // Normalise base URL: strip trailing slash.
    const baseUrl = rawBaseUrl.replace(/\/+$/, '');

    this.config = {
      baseUrl,
      apiKey: config.apiKey ?? '',
      timeout: config.timeout ?? DEFAULTS.timeout,
      retryCount: config.retryCount ?? DEFAULTS.retryCount,
      headers: config.headers ?? {},
    };

    this.http = axios.create({
      baseURL: baseUrl,
      timeout: this.config.timeout,
      headers: {
        'Content-Type': 'application/json',
        'User-Agent': 'Cosca-SDK-TypeScript/1.0',
        ...this.config.headers,
        ...(this.config.apiKey ? { Authorization: `Bearer ${this.config.apiKey}` } : {}),
      },
    });

    // Attach response interceptor for error handling.
    this.http.interceptors.response.use(
      (response) => response,
      (error) => this.handleError(error),
    );

    // Initialise sub-APIs.
    this.knowledge = new KnowledgeAPI(this.http, this.config);
    this.memory = new MemoryAPI(this.http, this.config);
    this.context = new ContextAPI(this.http, this.config);
    this.runtime = new RuntimeAPI(this.http, this.config);
    this.plugins = new PluginsAPI(this.http, this.config);
    this.discovery = new DiscoveryAPI(this.http, this.config);
    this.agents = new AgentsAPI(this.http, this.config);
    this.skills = new SkillsAPI(this.http, this.config);
    this.providers = new ProvidersAPI(this.http, this.config);
    this.workflows = new WorkflowsAPI(this.http, this.config);
    this.orchestration = new OrchestrationAPI(this.http, this.config);
  }

  /**
   * Returns the client's configuration (read-only).
   */
  getConfig(): Readonly<Required<AosClientConfig>> {
    return { ...this.config };
  }

  // ===========================================================================
  // HTTP Helpers
  // ===========================================================================

  /**
   * Perform a GET request with retry logic.
   *
   * @internal
   */
  async get<T>(path: string, config?: AxiosRequestConfig): Promise<T> {
    return this.requestWithRetry<T>(() => this.http.get<T>(path, config));
  }

  /**
   * Perform a POST request with retry logic.
   *
   * @internal
   */
  async post<T>(path: string, data?: unknown, config?: AxiosRequestConfig): Promise<T> {
    return this.requestWithRetry<T>(() => this.http.post<T>(path, data, config));
  }

  /**
   * Perform a PUT request with retry logic.
   *
   * @internal
   */
  async put<T>(path: string, data?: unknown, config?: AxiosRequestConfig): Promise<T> {
    return this.requestWithRetry<T>(() => this.http.put<T>(path, data, config));
  }

  /**
   * Perform a DELETE request with retry logic.
   *
   * @internal
   */
  async delete<T>(path: string, config?: AxiosRequestConfig): Promise<T> {
    return this.requestWithRetry<T>(() => this.http.delete<T>(path, config));
  }

  // ===========================================================================
  // Retry Logic
  // ===========================================================================

  /**
   * Wraps a request in exponential-backoff retry logic.
   * Only retries on 5xx, network errors, and timeouts.
   */
  private async requestWithRetry<T>(fn: () => Promise<AxiosResponse<T>>): Promise<T> {
    let lastError: Error | null = null;

    for (let attempt = 0; attempt <= this.config.retryCount; attempt++) {
      try {
        const response = await fn();
        return response.data;
      } catch (err) {
        lastError = err as Error;

        // Do not retry if this was the last attempt.
        if (attempt >= this.config.retryCount) {
          break;
        }

        // Only retry on server errors (5xx) and network/timeout errors.
        if (err instanceof CoscaError && err.statusCode < 500) {
          throw err;
        }

        // Exponential backoff: 100ms, 200ms, 400ms, ...
        const delay = Math.min(100 * Math.pow(2, attempt), 5000);
        await this.sleep(delay);
      }
    }

    throw lastError ?? new CoscaError('Request failed', 500, 'E1001');
  }

  /** Small sleep utility. */
  private sleep(ms: number): Promise<void> {
    return new Promise((resolve) => setTimeout(resolve, ms));
  }

  // ===========================================================================
  // Error Handling
  // ===========================================================================

  /**
   * Axios response error interceptor. Normalises errors into CoscaError.
   *
   * Handles two error response formats:
   *  - Go handler style: `{"error": "message"}`
   *  - Structured style:  `{"code": "E4001", "message": "...", ...}`
   */
  private handleError(error: unknown): never {
    if (axios.isAxiosError(error)) {
      const status = error.response?.status ?? 0;
      const data = error.response?.data as Record<string, unknown> | undefined;

      if (data && typeof data === 'object') {
        // Go-handler style: {"error": "message"}
        const apiErrorMsg = data.error as string | undefined;
        // Structured style: {"code": "...", "message": "..."}
        const code = data.code as string | undefined;
        const message = (data.message as string)
          ?? apiErrorMsg
          ?? error.message;

        if (code || apiErrorMsg) {
          throw new CoscaError(message, status, code, data as Record<string, unknown>);
        }
      }

      // Network / timeout errors.
      if (error.code === 'ECONNABORTED') {
        throw new CoscaError('Request timed out', 408, 'E1005');
      }
      if (error.code === 'ECONNREFUSED') {
        throw new CoscaError(`Connection refused at ${this.config.baseUrl}`, 502, 'E10002');
      }
      if (error.code === 'ENOTFOUND') {
        throw new CoscaError(`DNS resolution failed for ${this.config.baseUrl}`, 502, 'E10001');
      }

      throw new CoscaError(
        error.message,
        status,
        'E10000',
        { code: error.code },
      );
    }

    if (error instanceof Error) {
      throw new CoscaError(error.message, 500, 'E1001');
    }

    throw new CoscaError('An unknown error occurred', 500, 'E1000');
  }
}
