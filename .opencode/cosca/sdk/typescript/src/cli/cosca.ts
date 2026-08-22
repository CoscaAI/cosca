#!/usr/bin/env node

/**
 * Cosca — Command Line Interface
 * 
 * Usage: cosca <command> [options]
 */

import { Command } from 'commander';
import { createClient } from '../index';

const program = new Command();

program
  .name('cosca')
  .description('Cosca Framework — AI Orchestration System CLI')
  .version('1.0.0');

// ─── health ────────────────────────────────────────────

program
  .command('health')
  .description('Check Cosca Runtime health')
  .option('-u, --url <url>', 'Cosca API URL', process.env.COSCA_API_URL)
  .action(async (options) => {
    try {
      const cosca = createClient({ baseUrl: options.url });
      const health = await cosca.health();
      console.log(JSON.stringify(health, null, 2));
    } catch (error: any) {
      console.error(`❌ Health check failed: ${error.message}`);
      process.exit(1);
    }
  });

// ─── workflows ─────────────────────────────────────────

program
  .command('workflows')
  .description('List available workflows')
  .option('-u, --url <url>', 'Cosca API URL')
  .option('-c, --category <category>', 'Filter by category')
  .action(async (options) => {
    try {
      const cosca = createClient({ baseUrl: options.url });
      const workflows = await cosca.workflows.list(options.category as any);
      console.log(JSON.stringify(workflows, null, 2));
    } catch (error: any) {
      console.error(`❌ Failed to list workflows: ${error.message}`);
      process.exit(1);
    }
  });

program
  .command('workflow:get')
  .description('Get workflow details')
  .argument('<name>', 'Workflow name')
  .option('-u, --url <url>', 'Cosca API URL')
  .action(async (name, options) => {
    try {
      const cosca = createClient({ baseUrl: options.url });
      const workflow = await cosca.workflows.get(name);
      console.log(JSON.stringify(workflow, null, 2));
    } catch (error: any) {
      console.error(`❌ Workflow not found: ${error.message}`);
      process.exit(1);
    }
  });

program
  .command('workflow:execute')
  .description('Execute a workflow')
  .argument('<name>', 'Workflow name')
  .option('-u, --url <url>', 'Cosca API URL')
  .option('-i, --inputs <json>', 'Workflow inputs as JSON string')
  .action(async (name, options) => {
    try {
      const cosca = createClient({ baseUrl: options.url });
      const inputs = options.inputs ? JSON.parse(options.inputs) : {};
      const result = await cosca.workflows.execute(name, inputs);
      console.log(JSON.stringify(result, null, 2));
    } catch (error: any) {
      console.error(`❌ Execution failed: ${error.message}`);
      process.exit(1);
    }
  });

// ─── quality-gate ──────────────────────────────────────

program
  .command('quality-gate')
  .description('Run a quality gate check')
  .argument('<gate-number>', 'Gate number (e.g., 2.1, 2.2, 3)')
  .option('-u, --url <url>', 'Cosca API URL')
  .option('-a, --artifacts <json>', 'Artifacts to check as JSON string')
  .action(async (gateNumber, options) => {
    try {
      const cosca = createClient({ baseUrl: options.url });
      const artifacts = options.artifacts ? JSON.parse(options.artifacts) : {};
      const result = await cosca.workflows.runQualityGate(gateNumber, artifacts);
      console.log(JSON.stringify(result, null, 2));
    } catch (error: any) {
      console.error(`❌ Quality gate failed: ${error.message}`);
      process.exit(1);
    }
  });

program.parse(process.argv);

// Show help if no command
if (!process.argv.slice(2).length) {
  program.outputHelp();
}
