/**
 * cosca workflow — List, get, and execute workflows
 * 
 * Primary interface for AI assistants to discover and run Cosca workflows.
 */

import { AosFileSystem } from '../core/cosca-fs';
import { AosConfig } from '../types';

export async function workflowCommand(cosca: AosFileSystem, config: AosConfig, args: string[]): Promise<void> {
  const subcommand = args[0] || 'list';
  const format = config.format;

  switch (subcommand) {
    case 'list':
      await listWorkflows(cosca, format, args.slice(1));
      break;
    case 'get':
      await getWorkflow(cosca, format, args.slice(1));
      break;
    case 'run':
      await runWorkflow(cosca, format, args.slice(1));
      break;
    default:
      console.error(`Unknown subcommand: ${subcommand}`);
      console.log('Usage: cosca workflow <list|get|run> [options]');
      process.exit(1);
  }
}

async function listWorkflows(cosca: AosFileSystem, format: string, args: string[]): Promise<void> {
  const category = args[0];
  let workflows = cosca.listWorkflows();
  
  if (category) {
    workflows = workflows.filter(w => w.category === category);
  }

  if (format === 'json') {
    console.log(JSON.stringify(workflows, null, 2));
    return;
  }

  if (workflows.length === 0) {
    console.log('No workflows found.');
    return;
  }

  console.log(`📋 Available Workflows (${workflows.length}):`);
  console.log('');
  
  const grouped: Record<string, typeof workflows> = {};
  for (const w of workflows) {
    if (!grouped[w.category]) grouped[w.category] = [];
    grouped[w.category].push(w);
  }

  for (const [cat, wfs] of Object.entries(grouped)) {
    console.log(`  ${cat}:`);
    for (const w of wfs) {
      console.log(`    • ${w.name} — ${w.objective.substring(0, 70)}...`);
    }
    console.log('');
  }
}

async function getWorkflow(cosca: AosFileSystem, format: string, args: string[]): Promise<void> {
  const name = args[0];
  if (!name) {
    console.error('Usage: cosca workflow get <name>');
    process.exit(1);
  }

  const workflow = cosca.getWorkflow(name);
  if (!workflow) {
    console.error(`Workflow '${name}' not found`);
    process.exit(1);
  }

  if (format === 'json') {
    console.log(JSON.stringify(workflow, null, 2));
    return;
  }

  console.log(`📋 ${workflow.name} (v${workflow.version})`);
  console.log(`   Category: ${workflow.category}`);
  console.log(`   ${workflow.objective}`);
  console.log('');
  console.log('   Steps:');
  workflow.steps.forEach((s, i) => {
    console.log(`   ${i + 1}. ${s.task} [${s.chief}]`);
  });
  console.log('');
  if (workflow.preconditions.length > 0) {
    console.log('   Preconditions:');
    workflow.preconditions.forEach(p => console.log(`      • ${p}`));
  }
}

async function runWorkflow(cosca: AosFileSystem, format: string, args: string[]): Promise<void> {
  const name = args[0];
  if (!name) {
    console.error('Usage: cosca workflow run <name> [inputs...]');
    process.exit(1);
  }

  const workflow = cosca.getWorkflow(name);
  if (!workflow) {
    console.error(`Workflow '${name}' not found`);
    process.exit(1);
  }

  // Parse inputs from remaining args (key=value pairs)
  const inputs: Record<string, string> = {};
  for (const arg of args.slice(1)) {
    const [k, v] = arg.split('=');
    if (k && v) inputs[k] = v;
  }

  if (format === 'json') {
    console.log(JSON.stringify({
      workflow: name,
      steps: workflow.steps.length,
      inputs,
      status: 'ready',
    }, null, 2));
    return;
  }

  console.log(`🚀 Starting workflow: ${name}`);
  console.log(`   Steps: ${workflow.steps.length}`);
  console.log(`   Inputs: ${JSON.stringify(inputs)}`);
  console.log('');
  console.log('   Execution Plan:');
  workflow.steps.forEach((s, i) => {
    console.log(`   Step ${i + 1}: ${s.task}`);
    console.log(`           Chief: ${s.chief}`);
    console.log(`           Output: ${s.output.substring(0, 50)}`);
    console.log('');
  });
  console.log('💡 Pass this plan to your AI assistant to execute step by step.');
}
