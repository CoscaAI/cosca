/**
 * cosca skill — List and inspect skills
 * 
 * AI assistants use this to discover available skills for specific tasks.
 */

import { AosFileSystem } from '../core/cosca-fs';
import { AosConfig } from '../types';

export async function skillCommand(cosca: AosFileSystem, config: AosConfig, args: string[]): Promise<void> {
  const subcommand = args[0] || 'list';
  const format = config.format;

  switch (subcommand) {
    case 'list':
      await listSkills(cosca, format, args.slice(1));
      break;
    case 'get':
      await getSkill(cosca, format, args.slice(1));
      break;
    case 'search':
      await searchSkills(cosca, format, args.slice(1));
      break;
    default:
      console.error('Usage: cosca skill <list|get|search> [options]');
      process.exit(1);
  }
}

async function listSkills(cosca: AosFileSystem, format: string, args: string[]): Promise<void> {
  const category = args[0];
  let skills = cosca.listSkills();

  if (category) {
    skills = skills.filter(s => s.category === category);
  }

  if (format === 'json') {
    console.log(JSON.stringify(skills, null, 2));
    return;
  }

  const grouped: Record<string, typeof skills> = {};
  for (const s of skills) {
    if (!grouped[s.category]) grouped[s.category] = [];
    grouped[s.category].push(s);
  }

  console.log(`📚 Available Skills (${skills.length}):`);
  console.log('');
  for (const [cat, sk] of Object.entries(grouped)) {
    console.log(`  ${cat}:`);
    sk.forEach(s => console.log(`    • ${s.name} — ${s.description.substring(0, 60)}...`));
    console.log('');
  }
}

async function getSkill(cosca: AosFileSystem, format: string, args: string[]): Promise<void> {
  const name = args[0];
  if (!name) {
    console.error('Usage: cosca skill get <name>');
    process.exit(1);
  }

  const skill = cosca.getSkill(name);
  if (!skill) {
    console.error(`Skill '${name}' not found`);
    process.exit(1);
  }

  if (format === 'json') {
    console.log(JSON.stringify(skill, null, 2));
    return;
  }

  console.log(`📚 ${skill.name} (v${skill.version})`);
  console.log(`   Category: ${skill.category}`);
  console.log(`   ${skill.description}`);
  console.log('');
  console.log('   Inputs:');
  skill.inputs.forEach(i => {
    console.log(`   • ${i.name} (${i.type}) ${i.required ? '[required]' : '[optional]'} — ${i.description}`);
  });
  console.log('');
  console.log('   Outputs:');
  skill.outputs.forEach(o => {
    console.log(`   • ${o.name} (${o.type}) — ${o.description}`);
  });
}

async function searchSkills(cosca: AosFileSystem, format: string, args: string[]): Promise<void> {
  const query = args.join(' ').toLowerCase();
  if (!query) {
    console.error('Usage: cosca skill search <query>');
    process.exit(1);
  }

  const all = cosca.listSkills();
  const results = all.filter(s => 
    s.name.toLowerCase().includes(query) ||
    s.description.toLowerCase().includes(query) ||
    s.category.toLowerCase().includes(query)
  );

  if (format === 'json') {
    console.log(JSON.stringify(results, null, 2));
    return;
  }

  console.log(`🔍 Skills matching '${query}':`);
  console.log('');
  if (results.length === 0) {
    console.log('   No matches found.');
    return;
  }
  results.forEach(s => {
    console.log(`   • ${s.name} (${s.category})`);
    console.log(`     ${s.description.substring(0, 80)}`);
    console.log('');
  });
}
