/**
 * Cosca — File System Access Layer
 * 
 * Reads and parses the Cosca Markdown file structure.
 * This is the bridge between Markdown docs and structured data.
 */

import * as fs from 'fs';
import * as path from 'path';
import { glob } from 'glob';
import { AosConfig, Workflow, Skill, Department, MemoryRecord, HealthStatus, DiscoveryResult, WorkflowStep, SkillIO } from '../types';

export class AosFileSystem {
  private aosHome: string;

  constructor(config: AosConfig) {
    this.aosHome = config.aosHome;
  }

  // ─── Path Resolution ──────────────────────────────────

  private resolve(...parts: string[]): string {
    return path.join(this.aosHome, ...parts);
  }

  private exists(...parts: string[]): boolean {
    return fs.existsSync(this.resolve(...parts));
  }

  private readFile(...parts: string[]): string {
    return fs.readFileSync(this.resolve(...parts), 'utf-8');
  }

  // ─── Department Discovery ─────────────────────────────

  listDepartments(): Department[] {
    const deps: Department[] = [];
    const dir = this.resolve('departments');
    
    if (!fs.existsSync(dir)) return deps;

    for (const entry of fs.readdirSync(dir)) {
      const skillPath = path.join(dir, entry, 'SKILL.md');
      if (fs.existsSync(skillPath)) {
        const content = fs.readFileSync(skillPath, 'utf-8');
        deps.push(this.parseDepartment(entry, content));
      }
    }
    return deps;
  }

  getDepartment(name: string): Department | null {
    const skillPath = this.resolve('departments', name, 'SKILL.md');
    if (!fs.existsSync(skillPath)) return null;
    return this.parseDepartment(name, fs.readFileSync(skillPath, 'utf-8'));
  }

  private parseDepartment(name: string, content: string): Department {
    const purpose = this.extractSection(content, 'PURPOSE') || '';
    const responsibilities = this.extractList(content, 'RESPONSIBILITIES');
    const specialists = this.extractSpecialists(content);
    const reportsTo = this.extractReportsTo(content);
    
    return {
      name,
      purpose: purpose.split('\n')[0]?.replace(/^## PURPOSE\s*/i, '').trim() || '',
      responsibilities,
      specialists,
      reportsTo,
    };
  }

  // ─── Workflow Discovery ───────────────────────────────

  listWorkflows(): Workflow[] {
    const wfs: Workflow[] = [];
    const pattern = this.resolve('workflows', '*.md');
    const files = glob.sync(pattern);

    for (const file of files) {
      const content = fs.readFileSync(file, 'utf-8');
      const wf = this.parseWorkflow(path.basename(file, '.md'), content);
      if (wf) wfs.push(wf);
    }
    return wfs;
  }

  getWorkflow(name: string): Workflow | null {
    const wfPath = this.resolve('workflows', `${name}.md`);
    if (!fs.existsSync(wfPath)) return null;
    return this.parseWorkflow(name, fs.readFileSync(wfPath, 'utf-8'));
  }

  private parseWorkflow(name: string, content: string): Workflow | null {
    const objective = this.extractSection(content, 'OBJECTIVE');
    if (!objective) return null;

    return {
      name,
      version: this.extractMetadata(content, 'Version') || '1.0.0',
      category: this.extractMetadata(content, 'Category') || 'general',
      description: objective.replace(/^## OBJECTIVE\s*/i, '').trim(),
      objective: objective.replace(/^## OBJECTIVE\s*/i, '').trim(),
      steps: this.extractWorkflowSteps(content),
      preconditions: this.extractList(content, 'PRECONDITIONS'),
      postconditions: this.extractList(content, 'POSTCONDITIONS'),
      successCriteria: this.extractList(content, 'SUCCESS CRITERIA'),
    };
  }

  // ─── Skill Discovery ──────────────────────────────────

  listSkills(): Skill[] {
    const skills: Skill[] = [];
    const baseDir = this.resolve('skills');
    
    if (!fs.existsSync(baseDir)) return skills;

    for (const category of fs.readdirSync(baseDir)) {
      const catDir = path.join(baseDir, category);
      if (!fs.statSync(catDir).isDirectory()) continue;
      
      for (const file of fs.readdirSync(catDir)) {
        if (!file.endsWith('.md') || file === 'SKILLS_CATALOG.md') continue;
        const content = fs.readFileSync(path.join(catDir, file), 'utf-8');
        skills.push(this.parseSkill(path.basename(file, '.md'), category, content));
      }
    }
    return skills;
  }

  getSkill(name: string): Skill | null {
    const files = glob.sync(this.resolve('skills', '**', `${name}.md`));
    if (files.length === 0) return null;
    
    const content = fs.readFileSync(files[0], 'utf-8');
    const category = path.basename(path.dirname(files[0]));
    return this.parseSkill(name, category, content);
  }

  private parseSkill(name: string, category: string, content: string): Skill {
    return {
      name,
      category,
      version: this.extractMetadata(content, 'Version') || '1.0.0',
      description: this.extractSection(content, 'Description')?.replace(/^## Description\s*/i, '').trim() || '',
      inputs: this.extractSkillIO(content, 'Inputs'),
      outputs: this.extractSkillIO(content, 'Outputs'),
    };
  }

  // ─── Memory Access ────────────────────────────────────

  listMemory(type: string): MemoryRecord[] {
    const records: MemoryRecord[] = [];
    const dir = this.resolve('memory', type);
    
    if (!fs.existsSync(dir)) return records;

    for (const file of fs.readdirSync(dir)) {
      if (!file.endsWith('.md') || file === 'INDEX.md') continue;
      const content = fs.readFileSync(path.join(dir, file), 'utf-8');
      records.push({
        type,
        key: path.basename(file, '.md'),
        tags: this.extractTags(content),
        timestamp: this.extractFrontmatter(content, 'timestamp') || '',
        status: this.extractFrontmatter(content, 'status') || 'active',
        content: content.substring(0, 500),
      });
    }
    return records;
  }

  // ─── Quality Gates ────────────────────────────────────

  getQualityGate(gateNumber: string): string | null {
    const content = this.readFile('QUALITY_GATES.md');
    const gates: Record<string, string> = {};
    
    // Extract gate sections
    const gateRegex = /## Gate (\d[.\d]*) — (.*?)\n([\s\S]*?)(?=\n## Gate |$)/g;
    let match;
    while ((match = gateRegex.exec(content)) !== null) {
      gates[match[1]] = match[0];
    }
    
    return gates[gateNumber] || null;
  }

  // ─── Discovery (Project Scanning) ─────────────────────

  discoverProject(projectRoot: string): DiscoveryResult {
    const result: DiscoveryResult = {
      framework: null,
      language: null,
      database: null,
      dependencies: [],
      buildSystem: null,
      testFramework: null,
      architecturePattern: null,
    };

    // Detect language from package files
    if (this.existsIn(projectRoot, 'package.json')) {
      result.language = 'typescript';
      result.buildSystem = 'npm';
      const pkg = JSON.parse(fs.readFileSync(path.join(projectRoot, 'package.json'), 'utf-8') || '{}');
      result.dependencies = Object.keys(pkg.dependencies || {}).concat(Object.keys(pkg.devDependencies || {}));
      
      // Detect framework
      if (result.dependencies.some(d => d.includes('next'))) result.framework = 'next.js';
      else if (result.dependencies.some(d => d.includes('nestjs'))) result.framework = 'nestjs';
      else if (result.dependencies.some(d => d.includes('express'))) result.framework = 'express';
      
      // Detect test framework
      if (result.dependencies.some(d => d.includes('vitest'))) result.testFramework = 'vitest';
      else if (result.dependencies.some(d => d.includes('jest'))) result.testFramework = 'jest';
      
      // Detect database
      if (result.dependencies.some(d => d.includes('prisma'))) result.database = 'prisma';
      else if (result.dependencies.some(d => d.includes('typeorm'))) result.database = 'typeorm';
      else if (result.dependencies.some(d => d.includes('pg') || d.includes('postgres'))) result.database = 'postgresql';
      else if (result.dependencies.some(d => d.includes('mongoose') || d.includes('mongodb'))) result.database = 'mongodb';
    } 
    else if (this.existsIn(projectRoot, 'pyproject.toml') || this.existsIn(projectRoot, 'requirements.txt')) {
      result.language = 'python';
      result.buildSystem = 'pip';
    }
    else if (this.existsIn(projectRoot, 'go.mod')) {
      result.language = 'go';
    }
    else if (this.existsIn(projectRoot, 'Cargo.toml')) {
      result.language = 'rust';
    }

    return result;
  }

  // ─── Health ───────────────────────────────────────────

  getHealth(): HealthStatus {
    const errors: string[] = [];
    
    const departments = this.listDepartments().length;
    const skills = this.listSkills().length;
    const workflows = this.listWorkflows().length;
    const engines = this.countEngines();
    const memoryRecords = this.countMemoryRecords();
    
    if (departments === 0) errors.push('No departments found');
    if (!this.exists('KERNEL.md')) errors.push('KERNEL.md missing');
    if (!this.exists('QUALITY_GATES.md')) errors.push('QUALITY_GATES.md missing');

    return {
      status: errors.length === 0 ? 'ok' : 'degraded',
      aosVersion: this.extractMetadata(
        fs.readFileSync(this.resolve('KERNEL.md'), 'utf-8'), 
        'Version'
      ) || 'unknown',
      departments,
      skills,
      workflows,
      engines,
      memoryRecords,
      errors,
    };
  }

  // ─── Helpers ──────────────────────────────────────────

  private existsIn(dir: string, file: string): boolean {
    return fs.existsSync(path.join(dir, file));
  }

  private extractSection(content: string, section: string): string | null {
    const regex = new RegExp(`## ${section}[\\s\\S]*?(?=\\n## |\\n---|$)`);
    const match = content.match(regex);
    return match ? match[0].trim() : null;
  }

  private extractMetadata(content: string, field: string): string | null {
    const regex = new RegExp(`\\*\\*${field}\\*\\*:\\s*([^|\\n]+)`);
    const match = content.match(regex);
    return match ? match[1].trim() : null;
  }

  private extractList(content: string, section: string): string[] {
    const sectionContent = this.extractSection(content, section);
    if (!sectionContent) return [];
    
    const items: string[] = [];
    const lines = sectionContent.split('\n');
    for (const line of lines) {
      const trimmed = line.replace(/^- \[ \] /, '').replace(/^- /, '').replace(/^\d+\.\s*/, '').trim();
      if (trimmed && !trimmed.startsWith('#')) items.push(trimmed);
    }
    return items;
  }

  private extractSpecialists(content: string): string[] {
    const specMatch = content.match(/\*\*Specialists\*\*:\s*(.*?)(?:\n|$)/);
    if (specMatch) return specMatch[1].split(',').map(s => s.trim());
    
    // Try table format
    const tableSection = this.extractSection(content, 'SPECIALISTS');
    if (tableSection) {
      const rows = tableSection.split('\n').filter(l => l.includes('|') && !l.includes('---') && !l.includes('Specialist'));
      return rows.map(r => r.split('|')[1]?.trim()).filter(Boolean);
    }
    return [];
  }

  private extractReportsTo(content: string): string[] {
    const match = content.match(/\*\*Reports To\*\*:\s*(.*?)(?:\n|$)/);
    if (match) return match[1].split(',').map(s => s.trim());
    return [];
  }

  private extractWorkflowSteps(content: string): WorkflowStep[] {
    const steps: WorkflowStep[] = [];
    const stepRegex = /### Step \d+: (.*?)\n- \*\*Chief\*\*: (.*?)\n- \*\*Specialists\*\*: (.*?)\n- \*\*Task\*\*: (.*?)\n- \*\*Output\*\*: (.*?)(?=\n|$)/g;
    
    let match;
    while ((match = stepRegex.exec(content)) !== null) {
      steps.push({
        task: match[4].trim(),
        output: match[5].trim(),
        chief: match[2].trim(),
        specialists: match[3].split(',').map(s => s.trim()),
      });
    }
    return steps;
  }

  private extractSkillIO(content: string, section: string): SkillIO[] {
    const sectionContent = this.extractSection(content, section);
    if (!sectionContent) return [];
    
    const ios: SkillIO[] = [];
    const lines = sectionContent.split('\n');
    let inTable = false;
    
    for (const line of lines) {
      if (line.includes('|') && line.includes('---')) { inTable = true; continue; }
      if (inTable && line.includes('|')) {
        const cols = line.split('|').map(c => c.trim());
        if (cols.length >= 4) {
          ios.push({
            name: cols[1] || '',
            type: cols[2] || 'string',
            required: cols[3]?.toLowerCase() === 'yes',
            description: cols[4] || cols[3] || '',
          });
        }
      }
    }
    return ios;
  }

  private extractTags(content: string): string[] {
    const match = content.match(/tags:\s*\[([^\]]+)\]/);
    if (match) return match[1].split(',').map(t => t.trim());
    return [];
  }

  private extractFrontmatter(content: string, field: string): string | null {
    const regex = new RegExp(`^${field}:\\s*(.+)$`, 'm');
    const match = content.match(regex);
    return match ? match[1].trim() : null;
  }

  private countEngines(): number {
    const dir = this.resolve('engines');
    if (!fs.existsSync(dir)) return 0;
    return fs.readdirSync(dir).filter(e => fs.statSync(path.join(dir, e)).isDirectory()).length;
  }

  private countMemoryRecords(): number {
    let count = 0;
    const dir = this.resolve('memory');
    if (!fs.existsSync(dir)) return 0;
    
    for (const store of fs.readdirSync(dir)) {
      const storeDir = path.join(dir, store);
      if (!fs.statSync(storeDir).isDirectory()) continue;
      count += fs.readdirSync(storeDir).filter(f => f.endsWith('.md') && f !== 'INDEX.md').length;
    }
    return count;
  }
}
