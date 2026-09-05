// SPDX-License-Identifier: MIT
// Copyright © 2024 Cosca Contributors

/**
 * @cosca/sdk — Official TypeScript SDK for the Cosca Enterprise Platform.
 *
 * Provides a programmatic API for interacting with the Cosca Runtime,
 * including knowledge base, memory, context, runtime control, plugin
 * management, and environment discovery.
 *
 * @example
 * ```typescript
 * import { CoscaClient } from '@cosca/sdk';
 *
 * // Auto-detect URL from COSCA_API_URL env var or default to localhost:14120
 * const client = new CoscaClient();
 *
 * // Search the knowledge base
 * const { results, total, durationMs } = await client.knowledge.search(
 *   'authentication',
 *   { limit: 5 },
 * );
 *
 * // Check runtime health
 * const health = await client.runtime.health();
 * console.log(health.healthy);
 * ```
 *
 * @packageDocumentation
 */

// =============================================================================
// Main Client
// =============================================================================

export { AosClient as CoscaClient, AosClient, CoscaError } from './client';

// =============================================================================
// Sub-API Classes
// =============================================================================

export { KnowledgeAPI } from './knowledge';
export { MemoryAPI } from './memory';
export { ContextAPI } from './context';
export { RuntimeAPI } from './runtime';
export { PluginsAPI } from './plugins';
export { DiscoveryAPI } from './discovery';
export { AgentsAPI } from './agents';
export { SkillsAPI } from './skills';
export { ProvidersAPI } from './providers';
export { WorkflowsAPI } from './workflows';
export { OrchestrationAPI } from './orchestration';

// =============================================================================
// Types
// =============================================================================

export type {
  // Client
  AosClientConfig,
  ApiErrorResponse,

  // Knowledge
  KnowledgeSearchOptions,
  KnowledgeSearchResult,
  KnowledgeSearchResponse,
  KnowledgeIndexRequest,
  KnowledgeIndexResponse,
  KnowledgeStatsResponse,
  KnowledgeGraphStats,
  KnowledgeSyncResponse,

  // Memory
  MemoryType,
  MemoryLayer,
  MemoryStoreRequest,
  MemoryStoreResponse,
  MemoryRecord,
  MemorySearchResponse,
  MemoryGetResponse,
  MemoryDeleteResponse,
  MemoryPromoteRequest,
  MemoryPromoteResponse,
  MemoryLayerStats,
  MemoryStatsResponse,

  // Runtime
  RuntimeState,
  HealthStatus,
  RuntimeComponentInfo,
  RuntimeStatusResponse,
  RuntimeHealthResponse,

  // Plugin (stub)
  PluginType,
  PluginPermission,
  PluginInfo,

  // Context (stub)
  ContextScope,
  ContextEntry,
  Context,
  ContextOptions,

  // Discovery (stub)
  DiscoveryReport,
  ProjectInfo,
  WorkspaceInfo,
  EditorInfo,

  // Graph
  Relation,

  // Agents
  AgentTableRow,
  Agent,

  // Skills
  SkillTool,
  Skill,

  // Providers
  Provider,
  TestResult,
  ProviderStatus,

  // Workflows
  WorkflowIO,
  WorkflowStep,
  Workflow,
  WorkflowResult,

  // Orchestration
  RunOptions,
  RunResult,
  StreamEvent,
} from './types';
