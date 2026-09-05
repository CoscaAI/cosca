/**
 * Cosca SDK — Core Types
 * 
 * Type definitions for all Cosca framework entities.
 */

// ─── Agent Types ───────────────────────────────────────

export interface Agent {
  id: string;
  name: string;
  type: 'chief' | 'specialist' | 'engine';
  department: string;
  version: string;
  status: 'active' | 'deprecated' | 'draft';
  reportsTo?: string;
  specialists?: string[];
}

export interface Skill {
  id: string;
  name: string;
  category: string;
  version: string;
  description: string;
  inputs: SkillInput[];
  outputs: SkillOutput[];
  process: string[];
}

export interface SkillInput {
  name: string;
  type: string;
  required: boolean;
  description: string;
}

export interface SkillOutput {
  name: string;
  type: string;
  description: string;
}

// ─── Workflow Types ────────────────────────────────────

export type WorkflowCategory =
  | 'init' | 'feature' | 'bug' | 'refactor'
  | 'review' | 'deploy' | 'audit' | 'migration'
  | 'security' | 'compliance' | 'ops' | 'testing'
  | 'maintenance';

export interface Workflow {
  id: string;
  name: string;
  version: string;
  category: WorkflowCategory;
  objective: string;
  inputs: WorkflowInput[];
  outputs: WorkflowOutput[];
  steps: WorkflowStep[];
  preconditions: string[];
  postconditions: string[];
}

export interface WorkflowInput {
  name: string;
  type: string;
  required: boolean;
  description: string;
}

export interface WorkflowOutput {
  name: string;
  type: string;
  description: string;
}

export interface WorkflowStep {
  id: string;
  name: string;
  chief: string;
  specialists: string[];
  task: string;
  output: string;
  dependsOn?: string[];
  parallel?: boolean;
}

// ─── Quality Gate Types ────────────────────────────────

export interface QualityGate {
  gateNumber: string;
  name: string;
  checks: QualityCheck[];
  score: number;
  passed: boolean;
}

export interface QualityCheck {
  name: string;
  threshold: string;
  severity: 'error' | 'warn';
  result: 'pass' | 'fail' | 'skip';
}

// ─── Council Types ─────────────────────────────────────

export interface Council {
  name: string;
  chair: string;
  members: string[];
  meets: string;
  authority: string;
  scope: string;
}

// ─── Memory Types ──────────────────────────────────────

export type MemoryType =
  | 'short' | 'long' | 'project'
  | 'architecture' | 'decision'
  | 'pattern' | 'bug' | 'agent';

export interface MemoryRecord {
  type: MemoryType;
  key: string;
  tags: string[];
  timestamp: string;
  status: 'active' | 'archived' | 'superseded';
  agent: string;
  data: Record<string, unknown>;
}

// ─── ADR Types ─────────────────────────────────────────

export interface ADR {
  id: string;
  title: string;
  status: 'proposed' | 'accepted' | 'deprecated' | 'superseded';
  deciders: string[];
  date: string;
  context: string;
  decision: string;
  consequences: string[];
  alternatives: string[];
}

// ─── Error Types ───────────────────────────────────────

export class AosError extends Error {
  constructor(
    message: string,
    public code: string,
    public statusCode: number = 500
  ) {
    super(message);
    this.name = 'AosError';
  }
}

export class ValidationError extends AosError {
  constructor(message: string) {
    super(message, 'VALIDATION_ERROR', 400);
    this.name = 'ValidationError';
  }
}

export class NotFoundError extends AosError {
  constructor(message: string) {
    super(message, 'NOT_FOUND', 404);
    this.name = 'NotFoundError';
  }
}

// ─── Config Types ──────────────────────────────────────

export interface AosConfig {
  baseUrl?: string;
  apiKey?: string;
  timeout?: number;
  retryCount?: number;
}
