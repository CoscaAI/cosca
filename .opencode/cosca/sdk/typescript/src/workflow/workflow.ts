/**
 * Cosca SDK — Workflow Engine Client
 * 
 * Client for discovering, executing, and monitoring Cosca workflows.
 */

import { AosClient } from '../client/client';
import {
  Workflow,
  WorkflowStep,
  WorkflowCategory,
  QualityGate,
} from '../types';

export class WorkflowClient {
  constructor(private client: AosClient) {}

  /**
   * Discover all available workflows.
   */
  async list(category?: WorkflowCategory): Promise<Workflow[]> {
    const path = category
      ? `/workflows?category=${category}`
      : '/workflows';
    return this.client.get<Workflow[]>(path);
  }

  /**
   * Get a specific workflow by name.
   */
  async get(name: string): Promise<Workflow> {
    return this.client.get<Workflow>(`/workflows/${name}`);
  }

  /**
   * Execute a workflow with given inputs.
   */
  async execute(
    name: string,
    inputs: Record<string, unknown>
  ): Promise<{ executionId: string; status: string }> {
    return this.client.post(`/workflows/${name}/execute`, inputs);
  }

  /**
   * Check execution status.
   */
  async getStatus(executionId: string): Promise<{
    status: string;
    currentStep: string;
    progress: number;
    startedAt: string;
  }> {
    return this.client.get(`/executions/${executionId}`);
  }

  /**
   * Execute a quality gate check.
   */
  async runQualityGate(
    gateNumber: string,
    artifacts: Record<string, unknown>
  ): Promise<QualityGate> {
    return this.client.post(`/quality-gates/${gateNumber}`, artifacts);
  }

  /**
   * Get workflow steps for manual execution guidance.
   */
  async getSteps(name: string): Promise<WorkflowStep[]> {
    const workflow = await this.get(name);
    return workflow.steps;
  }
}
