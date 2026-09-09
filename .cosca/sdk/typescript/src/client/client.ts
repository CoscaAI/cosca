/**
 * Cosca SDK — HTTP Client
 * 
 * Core HTTP client for communicating with Cosca Runtime API.
 * Supports retry, timeout, authentication, and error handling.
 */

import axios, { AxiosInstance, AxiosRequestConfig, AxiosError } from 'axios';
import { AosConfig, AosError, NotFoundError, ValidationError } from '../types';

export class AosClient {
  private client: AxiosInstance;
  private retryCount: number;

  constructor(config: AosConfig = {}) {
    this.retryCount = config.retryCount ?? 3;
    
    this.client = axios.create({
      baseURL: config.baseUrl || process.env.COSCA_API_URL || 'http://localhost:8080',
      timeout: config.timeout || 30000,
      headers: {
        'Content-Type': 'application/json',
        ...(config.apiKey
          ? { 'Authorization': `Bearer ${config.apiKey}` }
          : {}),
      },
    });

    this.setupInterceptors();
  }

  private setupInterceptors(): void {
    this.client.interceptors.response.use(
      (response) => response,
      async (error: AxiosError) => {
        if (!error.response) {
          throw new AosError(
            'Network error — unable to reach Cosca Runtime',
            'NETWORK_ERROR',
            503
          );
        }

        const { status, data } = error.response;
        const message = (data as any)?.message || error.message;

        switch (status) {
          case 400:
            throw new ValidationError(message);
          case 404:
            throw new NotFoundError(message);
          case 401:
          case 403:
            throw new AosError(message, 'AUTH_ERROR', status);
          default:
            throw new AosError(message, 'INTERNAL_ERROR', status);
        }
      }
    );
  }

  async get<T>(path: string, config?: AxiosRequestConfig): Promise<T> {
    return this.withRetry(() => this.client.get<T>(path, config));
  }

  async post<T>(path: string, data?: unknown, config?: AxiosRequestConfig): Promise<T> {
    return this.withRetry(() => this.client.post<T>(path, data, config));
  }

  async put<T>(path: string, data?: unknown, config?: AxiosRequestConfig): Promise<T> {
    return this.withRetry(() => this.client.put<T>(path, data, config));
  }

  async delete<T>(path: string, config?: AxiosRequestConfig): Promise<T> {
    return this.withRetry(() => this.client.delete<T>(path, config));
  }

  private async withRetry<T>(
    operation: () => Promise<{ data: T }>
  ): Promise<T> {
    let lastError: Error | null = null;

    for (let attempt = 0; attempt < this.retryCount; attempt++) {
      try {
        const response = await operation();
        return response.data;
      } catch (error) {
        lastError = error as Error;
        
        // Don't retry validation or auth errors
        if (error instanceof ValidationError) throw error;
        if (error instanceof AosError && 
            (error.statusCode === 401 || error.statusCode === 403)) {
          throw error;
        }

        // Exponential backoff
        if (attempt < this.retryCount - 1) {
          await new Promise(r => setTimeout(r, Math.pow(2, attempt) * 1000));
        }
      }
    }

    throw lastError || new AosError('Max retries exceeded', 'RETRY_EXHAUSTED');
  }

  async health(): Promise<{ status: string; version: string }> {
    return this.get('/health');
  }
}
