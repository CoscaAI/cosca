# Relatório: Evolução Knowledge-First do Cosca

**Para:** Don  
**Data:** 2026-08-06  
**Status:** Análise — aguardando aprovação para implementação

---

## 1. Diagnóstico Atual

### 1.1 O que funciona bem

| Componente | Estado |
|---|---|
| `make dep-doctor` (cosca) | 7/7 checks, auto-corrige promoção e sync |
| Backend match (56 deps) | 56/56 verified |
| Frontend match (49 deps) | 49/49 verified |
| Global knowledge base | 105 pacotes, todos validated |
| Aquisição individual | Funciona com `GITHUB_TOKEN` + throttle |
| Busca semântica | Funcional via FTS5 + fallback DNS |
| DNS resolver | Fallback para 8.8.8.8/1.1.1.1 (bypass `::1:53`) |
| SSRF guard | Multi-camada, bem testado |
| Budget circuit breaker | Evita aquisição descontrolada |

### 1.2 Bugs críticos encontrados

| Bug | Impacto | Severidade |
|---|---|---|
| `acquire "*"` batch crasha após ~21 pacotes | Corrompe `.cosca/knowledge/packages/` local com manifestos vazios | **Alta** |
| Ghost copy: binário lê `$CWD/.config/cosca/` em vez de `~/.config/cosca/` | Match/most commands usam store errada | **Alta** |
| `verifyGitHubRepo` falha com 403 no batch mesmo com token válido | Aquisição em lote impossível sem loop manual | **Alta** |
| `autoResolveRepo` cobre só 60 pacotes npm (tabela estática) | Qualquer lib nova fora da tabela = `no repository set` | **Crítica** |
| `ecosystemGuesses` cobre só ~200 entradas | Pacotes novos ficam com ecosystem `unknown` | **Média** |

### 1.3 O que falta para projetos novos funcionarem automaticamente

Hoje, se você criar um projeto com `package.json` contendo uma lib que não está na tabela estática (ex: `"drizzle-orm"`, `"hono"`, `"elysia"`):

```
1. cosca knowledge match → ✗ sem conhecimento (49/49 missing)
2. cosca knowledge acquire "*" → ✗ no repository set for "drizzle-orm"
3. Usuário precisa manualmente:
   a. Descobrir o repo GitHub (drizzle-team/drizzle-orm)
   b. Rodar: cosca knowledge add github:drizzle-team/drizzle-orm --global
   c. Rodar: cosca knowledge acquire drizzle-orm --global --allow-remote
4. Repetir para CADA lib nova
```

Isso é inviável para projetos com 20+ dependências novas.

---

## 2. Proposta: Pipeline de Auto-Resolução

### 2.1 Resolver repositórios automaticamente

A solução de menor esforço e maior impacto: **consultar a API do npm registry**.

```go
// Nova função: resolveNpmRepo()
// GET https://registry.npmjs.org/<package-name>
// Extrai .repository.url → parseia GitHub org/repo
// Extrai .keywords → infere ecosystem
// Extrai .license → preenche manifesto
```

**Por que npm registry?**
- API pública, sem autenticação, gratuita
- ~90% dos pacotes npm têm campo `repository` apontando pro GitHub
- Latência: 50-200ms por chamada
- Resolve os 2 milhões de pacotes npm, não só 60

**Cascata completa de fallback:**

| Ordem | Fonte | Quando usar |
|---|---|---|
| 1 | Tabela estática (`autoResolveRepo`) | Cache rápido, 60 entradas |
| 2 | `registry.npmjs.org/<name>` | Fallback primário, cobre ~90% |
| 3 | GitHub search API (`/search/repositories?q=<name>`) | Quando npm não tem repo |
| 4 | Interactive prompt | "No repo found for X. Enter org/repo:" |

### 2.2 Integrar no `dep-doctor`

O `dep-doctor` já verifica 7 coisas. Adicionar:

| # | Novo check | Auto-fix |
|---|---|---|
| 8 | Dependências não mapeadas | `npm registry` → registrar manifesto → `acquire` → `promote` |
| 9 | Ghost copy sync | Copiar global → `$CWD/.config/cosca/` |
| 10 | Limpeza de lixo local | Remover `.cosca/knowledge/packages/` local se ghost existe |

### 2.3 Corrigir bugs do batch

| Bug | Solução |
|---|---|
| Crash após 21 pacotes | Investigar `verifyGitHubRepo` 403 — provável race condition ou HTTP client exhausto. Adicionar retry com backoff. |
| Ghost copy path | `NewGlobalPackageStore` usa `os.UserHomeDir()` mas algo no runtime resolve pra CWD. Precisamos rastrear e corrigir a raiz. |
| Batch não pula validated | Já corrigido (`PackageStatusValidated` adicionado ao skip). |

---

## 3. Proposta: Agents Knowledge-First

### 3.1 Arquitetura atual

```
User Prompt
  → MAG (memória de sessões passadas)
  → ContextBuilder.knowledge.Search(prompt)  ← JÁ EXISTE
  → ContextBuilder.augmentPrompt()
  → Router (seleciona agente)
  → Executor → provider.Chat() → LLM externo
```

O conhecimento JÁ é buscado antes do LLM. O problema é que **não há decisão sobre se o conhecimento é suficiente** — ele sempre vai pro LLM de qualquer jeito.

### 3.2 Pipeline knowledge-first proposto

```
User Prompt
  → MAG (memória)
  → Knowledge.Search(prompt) → resultados + scores
  → KnowledgeEvaluator:
      ├─ score > 0.85 E fonte é curated? → responde DIRETO (sem LLM)
      ├─ score > 0.60 E fonte é acquired? → injeta no prompt + marca "use este conhecimento"
      └─ score < 0.60 OU conflito detectado? → LLM externo + criar evidence
  → Se LLM foi chamado:
      ├─ Comparar resposta LLM vs conhecimento local
      ├─ Se conflito → proposal Q-XXXX → review
      └─ Armazenar como evidência (nível Observation)
```

### 3.3 Onde injetar (injeção mais limpa)

**Opção recomendada: `KnowledgeEvaluator` como novo estágio no pipeline do orchestrator.**

Arquivo: `internal/orchestration/orchestrator.go`, entre step 3 (ContextBuilder) e step 4 (Router).

```go
// Step 3.5 — Knowledge-First Evaluation (NOVO)
if e.knowledgeEvaluator != nil {
    decision, err := e.knowledgeEvaluator.Evaluate(ctx, pc)
    if decision.SkipLLM {
        return &Result{Response: decision.DirectAnswer, Source: "knowledge"}, nil
    }
    pc.Data.Extra["knowledge_context"] = decision.Context
}
```

**Por que essa opção:**
- Mínima invasão no código existente
- Funciona com ambos os engines (orchestration + chat CLI)
- O `KnowledgeEvaluator` pode ser ligado/desligado por feature flag
- Sub-agents herdam automaticamente (spawn via mesmo executor)

### 3.4 Sub-agents também knowledge-first

Sub-agents (`spawn_agent`) atualmente recebem só `SystemPrompt + Task + Context`. Não têm acesso ao knowledge.

**Mudança:** Adicionar `knowledgeResults` ao `Context` map antes de spawn:

```go
// Em engine/subagent.go, Spawn():
context["knowledge"] = e.lastKnowledgeResults  // injetar resultados da busca
```

E no system prompt de sub-agents, adicionar diretiva:

```
Before calling external tools or making decisions, check the provided
knowledge context. If the knowledge has a definitive answer (score > 0.80),
use it directly. Only fall back to external resources when knowledge is
insufficient or conflicting.
```

---

## 4. Análise de Segurança

### 4.1 Riscos identificados

| Risco | Severidade | Probabilidade | Vale mitigar? |
|---|---|---|---|
| **Prompt injection via README malicioso** | ALTA | BAIXA (precisa de repo com 50+ stars, 90+ dias) | ✅ SIM — adicionar `contenttrust.Suspicious()` na aquisição |
| **Agente confundindo conhecimento adquirido vs curado** | MÉDIA-ALTA | MÉDIA (search results não distinguem origem) | ✅ SIM — flag `CategoryAcquired` nos resultados de busca |
| **Conflito entre conhecimento local e LLM** | MÉDIA | MÉDIA (LLM pode contradizer KB) | ✅ SIM — detector de conflito automático |
| **Conhecimento stale não detectado** | MÉDIA | BAIXA (docs de libs estáveis não mudam muito) | ⚠️ TALVEZ — aging check semanal, não em tempo real |
| **Exaustão do rate limit com npm registry** | BAIXA | BAIXA (registry é ilimitado para GET) | ❌ NÃO — não precisa |
| **SSRF via npm registry redirect** | BAIXA | MUITO BAIXA (registry é controlado pelo npm Inc.) | ❌ NÃO — já coberto pelo SSRF guard existente |
| **Ataque de供应链 via package.json falso** | MÉDIA | MUITO BAIXA (precisa comprometer npm registry) | ⚠️ TALVEZ — verificação de assinatura futura |
| **Aquisção descontrolada em projetos grandes** | BAIXA | BAIXA (budget circuit breaker já existe) | ❌ NÃO — já resolvido |

### 4.2 O que FAZER antes de implementar

1. **Prompt injection scan na aquisição** (crítico, fácil):
   ```go
   // Em acquire.go, após decodeGitHubReadme:
   if contenttrust.Suspicious(string(body)) {
       return nil, fmt.Errorf("acquire: conteúdo suspeito detectado em %s — rejeitado", pkgID)
   }
   ```

2. **Flag de origem nos search results** (importante, médio):
   ```go
   // Adicionar campo Origin ao SearchResult
   Origin string // "curated" | "acquired" | "external"
   ```

3. **NÃO implementar ainda** (complexo, risco baixo):
   - Aging automático (stale detection em tempo real)
   - Verificação de assinatura de pacotes
   - Quarentena automática de aquisições (já existe o quarantine separado)

### 4.3 Opinião: o que vale a pena

| Feature | Vale? | Por quê |
|---|---|---|
| npm registry auto-resolve | ✅ **SIM** | Resolve 90% dos casos com 50 linhas de código |
| KnowledgeEvaluator no pipeline | ✅ **SIM** | Reduz chamadas LLM desnecessárias, aumenta confiabilidade |
| Sub-agents knowledge-first | ✅ **SIM** | Evita que sub-agentes "alucinem" quando KB tem resposta |
| `dep-doctor` auto-acquire | ✅ **SIM** | Elimina intervenção manual em projetos novos |
| Prompt injection scan | ✅ **SIM** | Barreira crítica de segurança com custo zero |
| Origin flag nos resultados | ✅ **SIM** | Transparência sobre fonte do conhecimento |
| Aging automático | ⚠️ **DEPOIS** | Útil mas não urgente; docs de libs estáveis mudam pouco |
| Conflito automático | ⚠️ **DEPOIS** | Importante mas complexo; fazer depois do básico funcionar |

---

## 5. Plano de Implementação (Ordem)

### Fase 1 — Correções (hoje)
1. Corrigir ghost copy path (`os.UserHomeDir()` retornando CWD)
2. Corrigir crash do `acquire "*"` batch
3. Adicionar `contenttrust.Suspicious()` na aquisição

### Fase 2 — Auto-resolução (1-2 dias)
4. Implementar `resolveNpmRepo()` via npm registry API
5. Integrar no `autoResolveRepo` como fallback
6. Integrar no `dep-doctor` como auto-fix

### Fase 3 — Knowledge-first agents (2-3 dias)
7. Criar `KnowledgeEvaluator` como estágio do pipeline
8. Adicionar threshold de confiança (curated > 0.85, acquired > 0.60)
9. Injetar knowledge no contexto de sub-agents
10. Adicionar Origin flag nos search results

### Fase 4 — Refinamento (depois)
11. Detector de conflito knowledge vs LLM
12. Aging check semanal automático
13. Suporte a Python/Rust/Java no discovery

---

## 6. Estimativa de Esforço

| Fase | Linhas de código | Complexidade | Risco |
|---|---|---|---|
| Fase 1 (correções) | ~200 | Média | Baixo |
| Fase 2 (auto-resolve) | ~300 | Baixa | Baixo |
| Fase 3 (knowledge-first) | ~500 | Média | Médio |
| Fase 4 (refinamento) | ~800 | Alta | Médio |
| **Total** | **~1800** | | |

---

## 7. Conclusão

O Cosca já tem 80% da infraestrutura necessária. O conhecimento é buscado antes do LLM, a base é populada, o match funciona, o dep-doctor verifica. O que falta são três coisas:

1. **Corrigir os bugs** que impedem o fluxo automático (batch crash, ghost copy, 403)
2. **Resolver repositórios automaticamente** via npm registry — isso sozinho elimina 90% do atrito
3. **Ensinar os agentes a confiar no conhecimento local primeiro** — um `KnowledgeEvaluator` de ~200 linhas que decide se responde direto ou chama LLM

Com essas três coisas, um projeto novo com 30 dependências npm ficaria assim:

```bash
cd meu-projeto
make dep-doctor  # ou cosca knowledge match → trigger auto-resolve
# → 30/30 verified em 2 minutos, zero intervenção manual
```

Agentes e sub-agentes usariam o conhecimento local para 80%+ das perguntas, reduzindo custo de LLM e aumentando consistência.

**Recomendação: aprovar Fase 1 + Fase 2 primeiro. Fase 3 depois de validar que as correções funcionam.**
