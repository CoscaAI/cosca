/**
 * cosca discover — Scan current project and identify stack
 * 
 * Returns structured project info that AI assistants use to understand context.
 */

import { AosFileSystem } from '../core/cosca-fs';
import { AosConfig } from '../types';

export async function discoverCommand(cosca: AosFileSystem, config: AosConfig, args: string[]): Promise<void> {
  const projectRoot = args[0] || process.cwd();
  const format = config.format;

  const result = cosca.discoverProject(projectRoot);

  // Also check for Cosca-specific files
  const { existsSync } = require('fs');
  const { join } = require('path');

  const aosFiles = {
    'opencode.json': existsSync(join(projectRoot, 'opencode.json')),
    'opencode.jsonc': existsSync(join(projectRoot, 'opencode.jsonc')),
    '.cosca/PROJECT.md': existsSync(join(projectRoot, '.cosca', 'PROJECT.md')),
    'cosca.config.yaml': existsSync(join(projectRoot, 'cosca.config.yaml')),
  };

  if (format === 'json') {
    console.log(JSON.stringify({ ...result, aosFiles }, null, 2));
    return;
  }

  // Text output
  console.log('🔍 Cosca Discovery Results');
  console.log('');
  console.log('📦 Project Stack:');
  console.log(`   Language:  ${result.language || 'unknown'}`);
  console.log(`   Framework: ${result.framework || 'unknown'}`);
  console.log(`   Database:  ${result.database || 'unknown'}`);
  console.log(`   Build:     ${result.buildSystem || 'unknown'}`);
  console.log(`   Test:      ${result.testFramework || 'unknown'}`);
  console.log('');
  console.log('📋 Cosca Files:');
  for (const [file, exists] of Object.entries(aosFiles)) {
    console.log(`   ${exists ? '✅' : '❌'} ${file}`);
  }
  console.log('');
  
  if (result.framework) {
    console.log('💡 Recommended workflows:');
    const wfs = cosca.listWorkflows();
    const relevant = wfs.filter(w => {
      if (result.language === 'typescript' && w.name.includes('code-review')) return true;
      if (result.framework === 'next.js' && w.name.includes('deployment')) return true;
      return false;
    });
    relevant.forEach(w => console.log(`   • ${w.name} — ${w.objective.substring(0, 60)}...`));
  }
}
