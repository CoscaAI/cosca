/**
 * cosca init — Initialize Cosca in current project
 * 
 * Creates .cosca/ directory structure and project context.
 * Called automatically on `postinstall` and by AI assistants on session start.
 */

import { AosFileSystem } from '../core/cosca-fs';
import { AosConfig } from '../types';

export async function initCommand(cosca: AosFileSystem, config: AosConfig, args: string[]): Promise<void> {
  const projectRoot = args[0] || process.cwd();
  const format = config.format;

  // Create .cosca/ directory structure
  const aosDir = `${projectRoot}/.cosca`;
  const dirs = [
    `${aosDir}/memory/short`,
    `${aosDir}/memory/long`,
    `${aosDir}/memory/project`,
    `${aosDir}/memory/architecture`,
    `${aosDir}/memory/decision`,
  ];

  const { existsSync, mkdirSync, writeFileSync } = require('fs');
  const { join } = require('path');

  for (const dir of dirs) {
    if (!existsSync(dir)) mkdirSync(dir, { recursive: true });
  }

  // Create project context file
  const contextFile = `${aosDir}/PROJECT.md`;
  if (!existsSync(contextFile)) {
    const discovery = cosca.discoverProject(projectRoot);
    writeFileSync(contextFile, `# Cosca Project Context\n\n` +
      `**Initialized**: ${new Date().toISOString().split('T')[0]}\n` +
      `**Framework**: ${discovery.framework || 'unknown'}\n` +
      `**Language**: ${discovery.language || 'unknown'}\n` +
      `**Database**: ${discovery.database || 'unknown'}\n` +
      `**Build**: ${discovery.buildSystem || 'unknown'}\n` +
      `**Test**: ${discovery.testFramework || 'unknown'}\n\n` +
      `## Dependencies\n${discovery.dependencies.map((d: string) => `- ${d}`).join('\n')}\n`
    );
  }

  if (format === 'json') {
    console.log(JSON.stringify({ status: 'ok', projectRoot, aosDir }));
  } else {
    console.log(`✅ Cosca initialized in ${projectRoot}`);
    console.log(`   📁 ${aosDir}/`);
    console.log(`   📄 ${contextFile}`);
    console.log(`   💡 Run 'cosca discover' to scan project`);
  }
}
