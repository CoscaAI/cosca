# @cosca/sdk — Cosca Framework TypeScript SDK

SDK oficial para o **Cosca (AI Orchestration System) Framework** em TypeScript.

## Instalação

```bash
npm install @cosca/sdk
# ou
yarn add @cosca/sdk
# ou
pnpm add @cosca/sdk
```

## Uso Básico

### Cliente API

```typescript
import { createClient } from '@cosca/sdk';

const cosca = createClient({
  apiKey: process.env.COSCA_API_KEY,
  baseUrl: 'http://localhost:8080',
});

// Verificar health
const health = await cosca.health();
console.log('Cosca Runtime:', health.status);
```

### Workflows

```typescript
// Listar workflows
const workflows = await cosca.workflows.list();
console.log(`Found ${workflows.length} workflows`);

// Executar workflow
const execution = await cosca.workflows.execute('security-audit', {
  scope: 'full',
  target: './src',
});
console.log('Execution ID:', execution.executionId);
```

### Quality Gates

```typescript
const gateResult = await cosca.workflows.runQualityGate('2.2', {
  code: codeChanges,
});
console.log('Gate score:', gateResult.score);
```

## CLI

```bash
# Instalar globalmente
npm install -g @cosca/sdk

# Verificar saúde do runtime
cosca health

# Listar workflows
cosca workflows

# Executar workflow
cosca workflow:execute security-audit --inputs '{"scope":"full"}'

# Executar quality gate
cosca quality-gate 2.2 --artifacts '{"code":"..."}'
```

## API

### `createClient(config?)`

Cria um cliente Cosca configurado.

| Parâmetro | Tipo | Default | Descrição |
|-----------|------|---------|-----------|
| `baseUrl` | string | `http://localhost:8080` | URL do Cosca Runtime |
| `apiKey` | string | `COSCA_API_KEY` env | Chave de autenticação |
| `timeout` | number | 30000 | Timeout em ms |
| `retryCount` | number | 3 | Número de retentativas |

### Tipos

O SDK exporta todos os tipos TypeScript do framework:

- `Agent`, `Skill`, `Workflow`, `WorkflowStep`
- `QualityGate`, `QualityCheck`
- `Council`, `ADR`, `MemoryRecord`
- `AosError`, `ValidationError`, `NotFoundError`

## Desenvolvimento

```bash
git clone https://github.com/cosca-framework/cosca.git
cd cosca/sdk/typescript
npm install
npm run build
npm test
```

## Licença

MIT
