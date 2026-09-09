/**
 * cosca gate — Run quality gate checks
 * 
 * AI assistants use this to validate code against Cosca quality standards.
 */

import { AosFileSystem } from '../core/cosca-fs';
import { AosConfig } from '../types';

export async function gateCommand(cosca: AosFileSystem, config: AosConfig, args: string[]): Promise<void> {
  const gateNumber = args[0];
  const format = config.format;

  if (!gateNumber) {
    if (format === 'json') {
      console.log(JSON.stringify({
        available: ['0', '1', '2.1', '2.2', '2.3', '2.4', '2.5', '2.6', '3', '4'],
        description: 'Use: cosca gate <number> to run a quality gate check',
      }));
    } else {
      console.log('🔍 Available Quality Gates:');
      console.log('   Gate 0:   Pre-Work (request validation)');
      console.log('   Gate 1:   Pre-Implementation (plan validation)');
      console.log('   Gate 2.1: Architecture Compliance');
      console.log('   Gate 2.2: Code Quality');
      console.log('   Gate 2.3: Security');
      console.log('   Gate 2.4: Performance');
      console.log('   Gate 2.5: Testing');
      console.log('   Gate 2.6: Documentation');
      console.log('   Gate 3:   Pre-Release');
      console.log('   Gate 4:   Post-Release');
      console.log('');
      console.log('Usage: cosca gate <number>');
    }
    return;
  }

  const gateContent = cosca.getQualityGate(gateNumber);
  
  if (!gateContent) {
    console.error(`Quality gate '${gateNumber}' not found`);
    process.exit(1);
  }

  if (format === 'json') {
    // Parse checks from gate content
    const checks: Array<{ name: string; threshold: string; severity: string }> = [];
    const lines = gateContent.split('\n');
    let inTable = false;
    
    for (const line of lines) {
      if (line.includes('|') && line.includes('---')) { inTable = true; continue; }
      if (inTable && line.includes('|')) {
        const cols = line.split('|').map(c => c.trim());
        if (cols.length >= 4) {
          checks.push({
            name: cols[1] || '',
            threshold: cols[2] || '',
            severity: cols[3]?.includes('Error') ? 'error' : 
                     cols[3]?.includes('Warn') ? 'warn' : 'info',
          });
        }
      }
    }

    console.log(JSON.stringify({
      gate: gateNumber,
      checks,
      checkCount: checks.length,
    }, null, 2));
    return;
  }

  console.log(`🔍 Quality Gate ${gateNumber}`);
  console.log(gateContent.substring(0, 600));
  console.log('...');
  console.log('💡 Pass this to your AI assistant to execute the checks.');
}
