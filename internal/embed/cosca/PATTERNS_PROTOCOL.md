# PATTERNS PROTOCOL — Padrões de código vs Buscar GitHub

> **Versão**: 1.0.0 | **Status**: canônico | **Dono**: cosca-kernel
> **Criado**: 2026-08-16, por ordem do Don — consolidar quando usar o padrão da
> casa e quando minerar o GitHub.
> **Propósito**: referência operacional ÚNICA para decidir entre o padrão interno
> e a mineração de código externo. Complementa `DEVELOPMENT_DOCTRINE.md` (25
> mandamentos) e `CONVENTIONS.md` (contrato de skill).

---

## 1. Os PADRÕES da família (o que JÁ temos)

Antes de olhar pra fora, olhe pra dentro. A casa tem três camadas de padrão:

| Camada | Onde | O que é |
|--------|------|---------|
| **Doutrina** | `DEVELOPMENT_DOCTRINE.md` | os 25 mandamentos (regras de quem constrói) |
| **Contrato** | `CONVENTIONS.md` | formato canônico de skill/doc |
| **Padrões minerados** | `knowledge/patterns/` (22 padrões) | o que já extraímos do GitHub |

Os 22 padrões estão indexados em `knowledge/patterns/INDEX.md`, por categoria:
arquitetura (3), design (3), Go (3), integração (2), AI (1), enterprise (10).
Cada um tem **confiança** (0.85–0.96) e segue o formato canônico §4.

---

## 2. A DECISÃO — interno primeiro, minerar se lacuna

A regra é simples (o "vs" resolvido):

```
1. BUSCAR interno  → knowledge/patterns/INDEX.md + DEVELOPMENT_DOCTRINE
        │
        ├── ACHOU → usa o padrão da casa (não reinventa)
        │
        └── NÃO ACHOU (lacuna) → MINERAR o GitHub
                                     │
                                     └── extrai + registra (vira padrão da casa)
```

- **Nunca minerar sem lacuna** — se a casa já tem o padrão, minerar é desperdício.
- **Nunca reimplementar sem minerar** — se há lacuna, o GitHub já resolveu antes;
  minerar antes de inventar.

---

## 3. A MINERAÇÃO — como extrair do GitHub

O processo (provado em L131, L132, L247, L248, L250, L251):

```
1. ESCOPO      → o que minerar (repo, org, ecossistema)
2. CLONE       → clone de REFERÊNCIA (fora do versionamento, como /eigen/)
3. ANÁLISE     → deep-mine: lê o código, entende a arquitetura
4. EXTRAÇÃO    → isola os padrões (não o código em si, mas o princípio)
5. REGISTRO    → knowledge/patterns/<nome>.md + INDEX.md (formato §4)
6. APLICAÇÃO   → implementa na casa (Tier 1/2/3)
```

**Foco da mineração**: extrair o **padrão** (o princípio reutilizável), não
copiar código. Exemplo real: minerar Kubernetes deu 17 padrões (informer,
workqueue keyed, reconcile loop, spec/status…) — princípios, não linhas.

---

## 4. O FORMATO canônico do padrão

Todo padrão registrado segue (CONVENTIONS §Quality):

```markdown
- **Intent**       — que problema resolve?
- **Context**      — quando/onde aplicar?
- **Solution**     — a descrição + guia de implementação
- **Consequences** — trade-offs, benefícios, riscos
- **Known Uses**   — onde já foi aplicado com sucesso?
- **Related Patterns** — complementares/alternativos
```

+ **Confidence** (0.0–1.0): 0.90+ canônico · 0.80+ com ressalva · 0.70+ corroborar · <0.70 proposta.

---

## 5. GOTCHAS

1. **Clone de referência não entra no git** — `/eigen/` foi gitignorado; clone
   de mineração carrega o próprio `.git` e não pertence ao repositório (L29).
2. **Minerar é extrair princípio, não copiar** — copiar código alheio sem
   entender viola a doutrina; extrair o padrão a enriquece.
3. **Lacuna ≠ preguiça** — só minerar quando o INDEX.md + DOCTRINE não cobrem.
4. **Confiança calibra o uso** — padrão <0.80 não vira canônico sem validação
   cross-agent (P13).
5. **Registrar no INDEX** — padrão sem entrada no INDEX.md fica órfão (L208:
   "fios soltos do conhecimento").

---

## 6. HISTÓRICO

| Versão | Data | Mudança |
|--------|------|---------|
| 1.0.0 | 2026-08-16 | Criado por ordem do Don — padrões de código vs buscar GitHub |
