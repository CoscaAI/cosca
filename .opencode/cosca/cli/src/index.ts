#!/usr/bin/env node

/**
 * Cosca — AI Orchestration System Runtime
 * 
 * Global CLI tool for AI assistants (OpenCode, Claude Code, Codex, etc.)
 * Provides discovery, workflow execution, quality gates, and knowledge access.
 * 
 * Usage: cosca <command> [options]
 *        cosca <command> --json   (JSON output for AI consumption)
 */

import { Command } from 'commander';
import { AosFileSystem } from './core/cosca-fs';
import { AosConfig } from './types';

// ─── Config ─────────────────────────────────────────────

function findAosHome(): string {
  // 1. COSCA_HOME env var
  if (process.env.COSCA_HOME) return process.env.COSCA_HOME;
  
  // 2. Check common locations
  const candidates = [
    '/home/henrique/.config/opencode/cosca',
    process.env.HOME ? `${process.env.HOME}/.config/opencode/cosca` : '',
    process.cwd() + '/cosca',
    __dirname + '/../../..',  // relative to CLI install
  ];
  
  for (const candidate of candidates) {
    const { existsSync } = require('fs');
    const { join } = require('path');
    if (candidate && existsSync(join(candidate, 'KERNEL.md'))) {
      return candidate;
    }
  }
  
  return '/home/henrique/.config/opencode/cosca';
}

function getConfig(): AosConfig {
  const jsonMode = process.argv.includes('--json');
  return {
    aosHome: findAosHome(),
    projectRoot: process.cwd(),
    format: jsonMode ? 'json' : 'text',
  };
}

// ─── Main ───────────────────────────────────────────────

async function main() {
  const config = getConfig();
  const cosca = new AosFileSystem(config);
  const format = config.format;
  const program = new Command();

  program
    .name('cosca')
    .description('Cosca Runtime — AI Orchestration System CLI')
    .version('1.0.0')
    .option('--json', 'Output in JSON format (for AI consumption)');

  // ─── init ─────────────────────────────────────────────
  program
    .command('init')
    .description('Initialize Cosca in current project')
    .argument('[dir]', 'Project directory')
    .action(async (dir) => {
      const { initCommand } = await import('./commands/init');
      await initCommand(cosca, config, dir ? [dir] : []);
    });

  // ─── discover ─────────────────────────────────────────
  program
    .command('discover')
    .description('Scan project and identify technology stack')
    .argument('[dir]', 'Project directory to scan')
    .action(async (dir) => {
      const { discoverCommand } = await import('./commands/discover');
      await discoverCommand(cosca, config, dir ? [dir] : []);
    });

  // ─── workflow ─────────────────────────────────────────
  program
    .command('workflow')
    .description('List, get, or execute workflows')
    .argument('[subcommand]', 'list, get, or run')
    .argument('[args...]', 'Subcommand arguments')
    .action(async (subcommand, args) => {
      const { workflowCommand } = await import('./commands/workflow');
      await workflowCommand(cosca, config, [subcommand, ...args].filter(Boolean));
    });

  // ─── skill ────────────────────────────────────────────
  program
    .command('skill')
    .description('List, get, or search skills')
    .argument('[subcommand]', 'list, get, or search')
    .argument('[args...]', 'Subcommand arguments')
    .action(async (subcommand, args) => {
      const { skillCommand } = await import('./commands/skill');
      await skillCommand(cosca, config, [subcommand, ...args].filter(Boolean));
    });

  // ─── gate ─────────────────────────────────────────────
  program
    .command('gate')
    .description('Run quality gate checks')
    .argument('[gate-number]', 'Gate number (0, 1, 2.1, 2.2, etc.)')
    .action(async (gateNumber) => {
      const { gateCommand } = await import('./commands/gate');
      await gateCommand(cosca, config, gateNumber ? [gateNumber] : []);
    });

  // ─── agent ────────────────────────────────────────────
  program
    .command('agent')
    .description('List agents/chiefs')
    .argument('[name]', 'Agent name')
    .action(async (name) => {
      const departments = cosca.listDepartments();
      
      if (format === 'json') {
        if (name) {
          const dept = cosca.getDepartment(name);
          console.log(JSON.stringify(dept, null, 2));
        } else {
          console.log(JSON.stringify(departments, null, 2));
        }
        return;
      }

      if (name) {
        const dept = cosca.getDepartment(name);
        if (!dept) { console.error(`Department '${name}' not found`); process.exit(1); }
        console.log(`👤 ${name}`);
        console.log(`   Purpose: ${dept.purpose}`);
        console.log(`   Responsibilities:`);
        dept.responsibilities.forEach(r => console.log(`   • ${r}`));
        console.log(`   Specialists: ${dept.specialists.join(', ')}`);
        return;
      }

      console.log(`👥 Cosca Chiefs (${departments.length}):`);
      const grouped: Record<string, typeof departments> = {};
      for (const d of departments) {
        const key = d.reportsTo[0] || 'unknown';
        if (!grouped[key]) grouped[key] = [];
        grouped[key].push(d);
      }
      for (const [group, deps] of Object.entries(grouped)) {
        console.log(`  Reports to ${group}:`);
        deps.forEach(d => console.log(`    • ${d.name} — ${d.purpose.substring(0, 50)}`));
      }
    });

  // ─── memory ───────────────────────────────────────────
  program
    .command('memory')
    .description('Access Cosca memory stores')
    .argument('[store]', 'Memory store (short, long, decision, pattern, bug, agent)')
    .action(async (store) => {
      const stores = ['short', 'long', 'project', 'architecture', 'decision', 'pattern', 'bug', 'agent'];
      
      if (format === 'json') {
        if (store) {
          const records = cosca.listMemory(store);
          console.log(JSON.stringify(records, null, 2));
        } else {
          const result: Record<string, number> = {};
          for (const s of stores) result[s] = cosca.listMemory(s).length;
          console.log(JSON.stringify(result, null, 2));
        }
        return;
      }

      if (store) {
        const records = cosca.listMemory(store);
        if (records.length === 0) {
          console.log(`No records in '${store}' memory.`);
          return;
        }
        console.log(`📝 ${store} memory (${records.length} records):`);
        records.forEach(r => {
          console.log(`   • ${r.key} [${r.status}] — tags: ${r.tags.join(', ')}`);
        });
        return;
      }

      console.log('📝 Cosca Memory Stores:');
      for (const s of stores) {
        const records = cosca.listMemory(s);
        console.log(`   ${s}: ${records.length} records`);
      }
      console.log('');
      console.log('Usage: cosca memory <store-name>');
    });

  // ─── health ───────────────────────────────────────────
  program
    .command('health')
    .description('Check Cosca framework health')
    .action(async () => {
      const health = cosca.getHealth();

      if (format === 'json') {
        console.log(JSON.stringify(health, null, 2));
        return;
      }

      const statusSymbol = health.status === 'ok' ? '✅' : health.status === 'degraded' ? '⚠️' : '❌';
      console.log(`${statusSymbol} Cosca Health Check`);
      console.log(`   Version:    ${health.aosVersion}`);
      console.log(`   Status:     ${health.status}`);
      console.log(`   Cosca Home:   ${config.aosHome}`);
      console.log('');
      console.log('📊 Resources:');
      console.log(`   Departments: ${health.departments}`);
      console.log(`   Skills:      ${health.skills}`);
      console.log(`   Workflows:   ${health.workflows}`);
      console.log(`   Engines:     ${health.engines}`);
      console.log(`   Memory:      ${health.memoryRecords} records`);
      
      if (health.errors.length > 0) {
        console.log('');
        console.log('❌ Errors:');
        health.errors.forEach(e => console.log(`   • ${e}`));
      }
    });

  program.parse(process.argv);

  // Show help if no command
  if (!process.argv.slice(2).length) {
    program.outputHelp();
  }
}

main().catch((error) => {
  console.error('❌ Cosca Error:', error.message);
  process.exit(1);
});
