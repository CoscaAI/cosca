/**
 * Cosca SDK — Main Entry Point
 * 
 * Export all public API surfaces for the Cosca TypeScript SDK.
 */

export { AosClient } from './client/client';
export { WorkflowClient } from './workflow/workflow';

export * from './types';

/**
 * Create a new Cosca client with the given configuration.
 * 
 * @example
 * ```typescript
 * import { createClient } from '@cosca/sdk';
 * 
 * const cosca = createClient({ apiKey: 'sk-xxx' });
 * const workflows = await cosca.workflows.list();
 * ```
 */
export function createClient(config?: import('./types').AosConfig) {
  const client = new AosClient(config);
  return {
    client,
    workflows: new WorkflowClient(client),
    health: () => client.health(),
  };
}

export default createClient;
