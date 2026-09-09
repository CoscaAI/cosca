/**
 * Cosca — Core Types
 */

export interface AosConfig {
  aosHome: string;
  projectRoot?: string;
  format: 'text' | 'json';
}

export interface Workflow {
  name: string;
  version: string;
  category: string;
  description: string;
  objective: string;
  steps: WorkflowStep[];
  preconditions: string[];
  postconditions: string[];
  successCriteria: string[];
}

export interface WorkflowStep {
  chief: string;
  specialists: string[];
  task: string;
  output: string;
}

export interface Skill {
  name: string;
  category: string;
  version: string;
  description: string;
  inputs: SkillIO[];
  outputs: SkillIO[];
}

export interface SkillIO {
  name: string;
  type: string;
  required: boolean;
  description: string;
}

export interface Department {
  name: string;
  purpose: string;
  responsibilities: string[];
  specialists: string[];
  reportsTo: string[];
}

export interface MemoryRecord {
  type: string;
  key: string;
  tags: string[];
  timestamp: string;
  status: string;
  content: string;
}

export interface DiscoveryResult {
  framework: string | null;
  language: string | null;
  database: string | null;
  dependencies: string[];
  buildSystem: string | null;
  testFramework: string | null;
  architecturePattern: string | null;
}

export interface HealthStatus {
  status: 'ok' | 'degraded' | 'error';
  aosVersion: string;
  departments: number;
  skills: number;
  workflows: number;
  engines: number;
  memoryRecords: number;
  errors: string[];
}
