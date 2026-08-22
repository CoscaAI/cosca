// SPDX-License-Identifier: MIT
// Copyright © 2024 Cosca Contributors

// =============================================================================
// Client Configuration
// =============================================================================

/** Configuration for creating a new Cosca SDK client. */
export interface AosClientConfig {
  /** Base URL of the Cosca Runtime (e.g., "http://localhost:14120").
   *  Falls back to COSCA_API_URL env var, then "http://localhost:14120". */
  baseUrl?: string;
  /** API key for authenticating with the runtime. */
  apiKey?: string;
  /** Request timeout in milliseconds. Defaults to 30000. */
  timeout?: number;
  /** Maximum number of retries for transient failures. Defaults to 3. */
  retryCount?: number;
  /** Additional headers to include with every request. */
  headers?: Record<string, string>;
}

// =============================================================================
// API Error
// =============================================================================

/** Standard error shape returned by Cosca REST API. */
export interface ApiErrorResponse {
  error: string;
}

// =============================================================================
// Knowledge Types
// =============================================================================

/** Options for knowledge-base searches. */
export interface KnowledgeSearchOptions {
  /** Maximum number of results. Defaults to 20. */
  limit?: number;
  /** Pagination offset. Defaults to 0. */
  offset?: number;
  /** Filter by result types. */
  types?: string[];
  /** Filter by path prefix. */
  pathFilter?: string;
  /** Minimum score threshold (0.0-1.0). */
  minScore?: number;
  /** Enable full-text search. Defaults to true. */
  enableFts?: boolean;
  /** Enable vector/semantic search. Defaults to true. */
  enableVector?: boolean;
  /** Enable graph search. Defaults to false. */
  enableGraph?: boolean;
  /** Enable facet aggregation. Defaults to false. */
  enableFacets?: boolean;
  /** Tag-based filters (key-value). */
  tags?: Record<string, string>;
}

/** A single knowledge-base search result. */
export interface KnowledgeSearchResult {
  /** Unique result identifier. */
  id: string;
  /** Result title. */
  title?: string;
  /** Matching text snippet. */
  snippet?: string;
  /** Relevance score (0.0-1.0). */
  score: number;
  /** Result type (file, entity, symbol, etc.). */
  type: string;
  /** Source file path. */
  path?: string;
}

/** Response body for knowledge search. */
export interface KnowledgeSearchResponse {
  /** Array of search result items. */
  results: KnowledgeSearchResult[];
  /** Total number of matching results. */
  total: number;
  /** Search duration in milliseconds. */
  durationMs: number;
  /** Facet counts (key → {facetValue: count}). */
  facets?: Record<string, Record<string, number>>;
}

/** Request body for indexing a document/directory. */
export interface KnowledgeIndexRequest {
  /** File or directory path to index. */
  path: string;
  /** Recursively index subdirectories. */
  recursive?: boolean;
}

/** Response body after indexing. */
export interface KnowledgeIndexResponse {
  /** Number of documents indexed. */
  documentsIndexed: number;
  /** Number of chunks created. */
  chunksIndexed: number;
  /** Per-file errors encountered. */
  errors?: string[];
}

/** Response body for knowledge-engine statistics. */
export interface KnowledgeStatsResponse {
  /** Number of indexed documents. */
  documentCount: number;
  /** Number of indexed chunks. */
  chunkCount: number;
  /** Number of extracted entities. */
  entityCount: number;
  /** Number of stored vectors. */
  vectorCount: number;
  /** SQLite database size in bytes. */
  dbSizeBytes: number;
  /** Engine uptime in seconds. */
  uptimeSeconds: number;
  /** Cache hit/miss statistics. */
  cacheStats?: Record<string, number>;
  /** Knowledge-graph statistics. */
  graphStats?: KnowledgeGraphStats;
  /** Timestamp of the last indexing operation (RFC 3339). */
  lastIndexed?: string;
}

/** Knowledge-graph statistics. */
export interface KnowledgeGraphStats {
  /** Number of graph nodes. */
  nodes: number;
  /** Number of graph edges. */
  edges: number;
  /** Graph density. */
  density: number;
  /** Number of connected components. */
  components: number;
  /** Whether the graph is empty. */
  isEmpty: boolean;
}

/** Response body after syncing the index with the filesystem. */
export interface KnowledgeSyncResponse {
  /** Number of added files. */
  added: number;
  /** Number of updated files. */
  updated: number;
  /** Number of removed files. */
  removed: number;
  /** Per-file errors encountered. */
  errors?: string[];
  /** Sync duration in milliseconds. */
  durationMs: number;
}

// =============================================================================
// Memory Types
// =============================================================================

/** Memory record type. */
export type MemoryType =
  | 'decision'
  | 'pattern'
  | 'bug'
  | 'agent'
  | 'project'
  | 'architecture'
  | 'session';

/** Memory layer (persistence tier). */
export type MemoryLayer =
  | 'session'
  | 'project'
  | 'workspace'
  | 'global'
  | 'temp';

/** Request body for storing a memory record. */
export interface MemoryStoreRequest {
  /** Memory type (decision, pattern, bug, agent, project, architecture, session). */
  type?: string;
  /** Storage layer. Defaults to "session". */
  layer?: string;
  /** Optional scope identifier. */
  scope?: string;
  /** Memory content body (required). */
  content: string;
  /** Priority value (higher = more important). Defaults to 0. */
  priority?: number;
  /** Time-to-live duration string (e.g. "24h", "7d"). */
  ttl?: string;
  /** Arbitrary key-value metadata. */
  metadata?: Record<string, string>;
}

/** Response body after storing a memory record. */
export interface MemoryStoreResponse {
  /** Assigned record ID. */
  id: string;
  /** Creation timestamp (RFC 3339). */
  createdAt: string;
}

/** A single memory record as returned by the API. */
export interface MemoryRecord {
  /** Unique record identifier. */
  id: string;
  /** Memory type. */
  type: string;
  /** Storage layer. */
  layer: string;
  /** Memory content. */
  content: string;
  /** Priority value. */
  priority: number;
  /** Creation timestamp (RFC 3339). */
  createdAt: string;
  /** Arbitrary metadata. */
  metadata?: Record<string, string>;
}

/** Response body for memory search. */
export interface MemorySearchResponse {
  /** Matched memory records. */
  records: MemoryRecord[];
  /** Total number of matches. */
  total: number;
}

/** Response body for single-record retrieval. */
export interface MemoryGetResponse {
  record: MemoryRecord;
}

/** Response body for memory deletion. */
export interface MemoryDeleteResponse {
  success: boolean;
}

/** Request body for promoting a memory record. */
export interface MemoryPromoteRequest {
  /** Memory record ID. */
  id: string;
  /** Source layer. */
  fromLayer: string;
  /** Target layer. */
  toLayer: string;
}

/** Response body after a promote operation. */
export interface MemoryPromoteResponse {
  record: MemoryRecord;
}

/** Per-layer statistics. */
export interface MemoryLayerStats {
  /** Number of records in the layer. */
  recordCount: number;
  /** Approximate size in bytes. */
  sizeBytes: number;
}

/** Response body for memory-engine statistics. */
export interface MemoryStatsResponse {
  layers: Record<string, MemoryLayerStats>;
}

// =============================================================================
// Runtime Types
// =============================================================================

/** Runtime state value. */
export type RuntimeState =
  | 'uninitialized'
  | 'initializing'
  | 'ready'
  | 'running'
  | 'stopping'
  | 'stopped'
  | 'error'
  | 'recovering'
  | 'unknown';

/** Component health status. */
export type HealthStatus =
  | 'unknown'
  | 'healthy'
  | 'degraded'
  | 'unhealthy';

/** Information about a runtime component. */
export interface RuntimeComponentInfo {
  name: string;
  status: string;
  uptime?: string;
  message?: string;
}

/** Response body for /v1/status. */
export interface RuntimeStatusResponse {
  /** Runtime current state. */
  state: string;
  /** Overall health assessment. */
  health: string;
  /** Human-readable uptime duration. */
  uptime: string;
  /** Runtime version string. */
  version: string;
  /** Per-component statuses. */
  components?: Record<string, RuntimeComponentInfo>;
}

/** Response body for /v1/health. */
export interface RuntimeHealthResponse {
  /** Whether the runtime considers itself healthy. */
  healthy: boolean;
  /** Non-critical warning messages. */
  warnings?: string[];
}

// =============================================================================
// Plugin Types (stub — no handlers yet)
// =============================================================================

/** Plugin type categorisation. */
export type PluginType = 'tool' | 'hook' | 'transport' | 'middleware' | 'storage' | 'ui';

/** Plugin permission. */
export type PluginPermission = 'network' | 'filesystem' | 'exec' | 'env' | 'database' | 'secrets';

/** Information about an installed plugin. */
export interface PluginInfo {
  id: string;
  name: string;
  version: string;
  description: string;
  author: string;
  license: string;
  type: PluginType;
  apiVersion: string;
  status: string;
  enabled: boolean;
  permissions?: PluginPermission[];
  entrypoint?: string;
  runtime?: string;
  hooks?: string[];
  config?: Record<string, unknown>;
  installedAt: string;
}

// =============================================================================
// Context Types (stub — no handlers yet)
// =============================================================================

export type ContextScope =
  | 'global'
  | 'project'
  | 'session'
  | 'agent'
  | 'skill'
  | 'workflow'
  | 'step';

export interface ContextEntry {
  id: string;
  key: string;
  value: unknown;
  scope: ContextScope;
  priority: number;
  ttl?: number;
  source?: string;
  createdAt: string;
}

export interface Context {
  id: string;
  scope: ContextScope;
  entries: ContextEntry[];
  entryCount: number;
  tokenCount: number;
  createdAt: string;
  expiresAt?: string;
  metadata?: Record<string, unknown>;
}

export interface ContextOptions {
  scope?: ContextScope;
  maxEntries?: number;
  maxTokens?: number;
  includeMemory?: boolean;
  includeKnowledge?: boolean;
  priorityThreshold?: number;
  agentId?: string;
}

// =============================================================================
// Discovery Types (stub — no handlers yet)
// =============================================================================

export interface DiscoveryReport {
  project: ProjectInfo;
  workspace: WorkspaceInfo;
  editor: EditorInfo;
  capturedAt: string;
}

export interface ProjectInfo {
  name: string;
  path: string;
  language: string;
  framework?: string;
  version?: string;
  buildSystem?: string;
  dependencies?: string[];
  entrypoint?: string;
  configFiles?: string[];
  hasTests: boolean;
  hasDocker: boolean;
  hasCI: boolean;
  confidence: number;
}

export interface WorkspaceInfo {
  root: string;
  ide?: string;
  shell?: string;
  terminal?: string;
  os: string;
  arch: string;
  homeDir: string;
  tempDir: string;
  envVars?: Record<string, string>;
  openBuffers?: string[];
  gitRoot?: string;
  gitBranch?: string;
}

export interface EditorInfo {
  name: string;
  version?: string;
  path?: string;
  pid?: number;
  connected: boolean;
  capabilities?: string[];
  extensions?: string[];
  language?: string;
  scheme?: string;
}

// =============================================================================
// Graph Types (stub — part of knowledge stats now)
// =============================================================================

export interface Relation {
  id: string;
  sourceId: string;
  sourceType: string;
  sourceLabel?: string;
  targetId: string;
  targetType: string;
  targetLabel?: string;
  type: string;
  weight?: number;
  properties?: Record<string, unknown>;
}

// =============================================================================
// Agents Types
// =============================================================================

/** Key-value row in an agent's metadata tables (dependencies, inputs, outputs). */
export interface AgentTableRow {
  key: string;
  value: string;
}

/** An Cosca agent with its capabilities and metadata. */
export interface Agent {
  /** Unique agent identifier. */
  name: string;
  /** Functional role description. */
  role: string;
  /** High-level mission statement. */
  mission?: string;
  /** Current agent status (active, inactive, etc.). */
  status: string;
  /** Agent version string. */
  version: string;
  /** Organisational department. */
  department: string;
  /** Direct report agent name. */
  reportsTo?: string;
  /** Longer description of the agent. */
  description?: string;
  /** Key responsibilities. */
  responsibilities?: string[];
  /** Declared dependencies. */
  dependencies?: AgentTableRow[];
  /** Expected inputs. */
  inputs?: AgentTableRow[];
  /** Expected outputs. */
  outputs?: AgentTableRow[];
}

// =============================================================================
// Skills Types
// =============================================================================

/** A tool defined within a skill. */
export interface SkillTool {
  /** Tool name. */
  name: string;
  /** Tool description. */
  description: string;
}

/** A skill that an agent can use to perform specialised tasks. */
export interface Skill {
  /** Unique skill identifier. */
  name: string;
  /** Skill description. */
  description: string;
  /** Semantic version. */
  version: string;
  /** Classification category. */
  category: string;
  /** Operational instructions. */
  instructions?: string;
  /** Tools provided by this skill. */
  tools?: SkillTool[];
  /** Origin of the skill (file path, URL, etc.). */
  source?: string;
}

// =============================================================================
// Providers Types
// =============================================================================

/** An AI/LLM provider registered in the Cosca Runtime. */
export interface Provider {
  /** Unique provider identifier (e.g. "openai", "anthropic"). */
  name: string;
  /** Connection status (available, configured, etc.). */
  status: string;
  /** Default model for this provider. */
  model: string;
  /** Whether this is the currently active provider. */
  active: boolean;
  /** Whether the provider has valid credentials. */
  configured: boolean;
  /** API base URL. */
  baseUrl: string;
  /** API version. */
  apiVersion: string;
  /** Available models for this provider. */
  models?: string[];
  /** Provider capabilities (chat, embeddings, etc.). */
  capabilities?: string[];
}

/** Outcome of a provider connectivity test. */
export interface TestResult {
  /** Round-trip time of the test request. */
  responseTime: string;
  /** Model tested against. */
  model: string;
  /** Test result (reachable, configured, timeout, etc.). */
  status: string;
}

/** Overall status of the provider subsystem after an activation. */
export interface ProviderStatus {
  /** Name of the currently active provider. */
  active: string;
  /** Count of providers with valid credentials. */
  configured: number;
  /** Total count of available providers. */
  available: number;
  /** Per-provider status strings. */
  statuses: string[];
}

// =============================================================================
// Workflows Types
// =============================================================================

/** A workflow input or output specification. */
export interface WorkflowIO {
  /** Parameter name. */
  name: string;
  /** Parameter data type. */
  type: string;
  /** Whether the parameter is mandatory. */
  required: boolean;
}

/** A single step within a workflow. */
export interface WorkflowStep {
  /** Step display name. */
  name: string;
  /** Step description. */
  description: string;
  /** Agent responsible for this step. */
  agent: string;
  /** Maximum duration allowed for this step. */
  timeout: string;
}

/** A structured process with defined steps executable by the Cosca Runtime. */
export interface Workflow {
  /** Unique workflow identifier. */
  name: string;
  /** Workflow purpose. */
  description: string;
  /** Semantic version. */
  version: string;
  /** Workflow status (active, inactive, disabled). */
  status: string;
  /** Whether the workflow can be executed. */
  enabled: boolean;
  /** Number of steps in the workflow. */
  steps: number;
  /** Detailed step definitions. */
  stepList?: WorkflowStep[];
  /** Expected inputs. */
  inputs?: WorkflowIO[];
  /** Expected outputs. */
  outputs?: WorkflowIO[];
}

/** Outcome of a workflow execution. */
export interface WorkflowResult {
  /** Final status (completed, failed). */
  status: string;
  /** Total wall-clock execution time. */
  duration: string;
  /** How many steps finished successfully. */
  stepsCompleted: number;
  /** Total number of steps in the workflow. */
  totalSteps: number;
  /** Workflow output specifications. */
  outputs?: WorkflowIO[];
}

// =============================================================================
// Orchestration Types
// =============================================================================

/** Options for an AI orchestration run or stream request. */
export interface RunOptions {
  /** Name of the agent to use for this run. */
  agent?: string;
  /** Name of the LLM provider to use. */
  provider?: string;
}

/** Outcome of a synchronous AI orchestration run. */
export interface RunResult {
  /** Final text output from the agent. */
  response: string;
  /** Name of the agent that handled the request. */
  agent: string;
  /** Skills invoked during execution. */
  skillsUsed?: string[];
  /** Total execution time in milliseconds. */
  durationMs: number;
  /** Stored execution memory record ID. */
  memoryId: string;
}

/** A single event emitted during a streaming orchestration run. */
export interface StreamEvent {
  /** Event type (thinking, response, error, done). */
  type: string;
  /** Human-readable event content. */
  content: string;
  /** Total run duration in milliseconds (only present in "done" events). */
  durationMs?: number;
}
