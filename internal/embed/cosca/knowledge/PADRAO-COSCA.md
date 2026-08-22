# PADRÃO COSCA — A Doutrina da Casa para Criar Qualquer Projeto

> **Nível:** Doutrina | **Origem:** Don + mineração de 26 repos (2026-08-16) | **Status:** Active v1
> **Propósito:** criar QUALQUER projeto no padrão Cosca — independente, autossuficiente, sem depender de ferramentas ou gostos de terceiros. Tudo que um projeto precisa nasce aqui.
> **Fontes:** ponytail · ui-ux-pro-max · taste-skill · impeccable · design-motion-principles · superpowers · gstack · caveman · humanizer · marketingskills · social-media-skills · graphify · agent-skill-creator · last30days-skill · agent-browser · new-api · buildwithclaude · no-ai-slop · i-have-adhd · Next.js · Tailwind v4 · shadcn/ui · NestJS · GSAP · Remotion · Vercel · Postgres · MySQL · MongoDB · Redis

---

## 1. PROCESSO — como trabalhar

**Princípio: o melhor código é o que nunca foi escrito.** Lazy significa eficiente, não descuidado (ponytail).

Regras:
1. **A Escada (pare no 1º degrau que segura):** precisa existir? → já existe na casa? → stdlib resolve? → recurso nativo cobre? → dependência instalada resolve? → dá pra fazer em 1 linha? → só então: o mínimo que funciona.
2. **Portão de aprovação:** nada de implementar sem o Don aprovar o plano (spike/bounded/architectural).
3. **Evidência fresca antes de "pronto":** se não rodou a verificação nesta mensagem, não pode afirmar que passa (P13).
4. **Root-cause first:** nenhum fix sem investigar a causa raiz primeiro; o fix preguiçoso É o da causa (um guard na função compartilhada < guard em todo caller).
5. **YAGNI radical:** sem abstração não-pedida (interface com 1 impl, factory de 1 produto, config de valor que nunca muda). Deletar sobre adicionar. Chato sobre esperto.
6. **TDD como lei:** nenhum código de produção sem teste falhando primeiro.
7. **Nunca ser preguiçoso com:** validação de entrada em trust boundaries, erro que evita perda de dado, segurança, acessibilidade, o que foi explicitamente pedido.

Anti-patterns: `over-engineering` · `abstração especulativa` · `fix no sintoma` · `"pronto" sem evidência` · `YAGNI violado por pressa`.

## 2. DESIGN — como parecer

**Princípio: especificidade mensurável sobre vibes.** "Clean, modern UI" não é decisão de design — nomeie a fonte, a escala, o hex, a duração (gstack).

Regras:
1. **`DESIGN.md` por projeto = fonte de verdade.** Leia antes de toda decisão visual; não desvie sem aprovação; em QA, flague todo código que não bate com ele.
2. **Tokens em 3 camadas:** Primitive (valores crus) → Semantic (papel) → Component (botão/input). OKLCH; base 4px; escala tipográfica única; dark mode = re-mapear a camada semântica. Nunca hex cru em componente. Validar por script.
3. **Cor:** máx 1 acento, saturação <80%; neutros quentes OU frios, nunca mistos; contraste ≥4.5:1 texto / ≥3:1 não-texto; cor nunca é o único sinal de estado.
4. **Tipografia:** 2 faces + mono funcional; Fira Sans/Fira Code são a voz da casa, não o default de todo projeto; corpo 16px; medida 65-75ch; tracking ≥ -0.04em.
5. **Espaçamento:** ritmo 4/8dp; grupos apertados, separação generosa; mais espaço acima do título que abaixo; cards só quando elevam hierarquia real.
6. **Consistência via locks:** um acento, um radius, um tema por página. Nunca misturar sistemas.
7. **Estados completos:** hover, disabled, loading, error, empty — sempre.

Anti-patterns (bans): `AI-purple gradients` · `Inter por padrão` · `3 cards iguais` · `tudo centrado` · `glassmorphism decorativo` · `eyebrow em toda seção` · `cards aninhados` · `mono como fantasia de "técnico"` · `nomes/números falsos` · `emoji como ícone estrutural` · `pulse/glow/hover-scale em tudo`.

## 3. MOTION — como se mover

**Princípio: a melhor animação é a que passa despercebida** (design-motion).

Regras:
1. **Frequency gate:** raro (mensal) = expressivo à vontade; frequente (diário) = sutil; 100s/dia = nenhuma ou instantânea; iniciado por teclado = nunca anima.
2. **Duração:** UI de produtividade <300ms (180ms ideal); produção 200-500ms. Não use cap universal.
3. **Easing:** ease-out = chegando, ease-in = saindo, linear = loop/progresso. **Exit mais sutil que enter.**
4. **Só transform/opacity/clip-path** (GPU, sem layout reflow).
5. **`prefers-reduced-motion` é inegociável** — no mesmo código, não follow-up.

Anti-patterns: `stagger em tudo` · `spring com bounce em ação utilitária` · `blur em tudo` · `motion on mount para conteúdo estático` · `indicador pulsante sem necessidade`.

## 4. OUTPUT — como falar

**Princípio: comece pela resposta. Termine quando a resposta acabou** (i-have-adhd + caveman).

Regras:
1. **Lead with the action** — primeira linha é algo executável (comando/caminho/snippet). Nada de preamble, recap ou "let me know if you need anything".
2. **Compressão estilo caveman:** corte artigos (a/an/the), filler (just/really/basically), pleasantries (sure/certainly), hedging. Fragmentos OK. Sinônimos curtos (big, não extensive).
3. **Preserve SEMPRE:** código (fenced/indented), URLs, paths, comandos, termos técnicos, nomes próprios, datas/versões/números, env vars. **Nunca solte `not/never/no/only/except`** — inverte o significado.
4. **Anti-abbreviação:** não invente abreviações novas (cfg/impl/req) nem setas → — tokenizer não economiza nada e o leitor paga a decodificação.
5. **Auto-clarity (saia do modo compressão):** warnings de segurança, confirmação de ação irreversível, sequências multi-passo onde ordem importa, ambiguidade técnica, quando o usuário pede.
6. **Anti-slop em clusters, não isolado:** um em-dash não é IA; cluster de tells é. Preserve sinais humanos (detalhe específico, sentimentos mistos, referências datadas, variação de frase).

Anti-patterns: `preamble` · `"I'll go ahead and..."` · `em-dash como muleta` · `em resumo` · `recap final` · `verbos-estação ("destacar", "fomentar")` · `parallelismo negativo ("não é X, é Y")`.

## 5. CONTEÚDO — como convencer

**Princípio: clareza sobre esperteza; benefício sobre feature; especificidade sobre vagueza** (marketingskills).

Regras:
1. **Headline:** 9 famílias (outcome-focused, problem-focused, audience, differentiation, proof...). `[Achieve outcome] without [pain point]`.
2. **CTA forte:** `[Action Verb] + [What They Get] + [Qualifier]` — "Start Free Trial", "Create Your First [Thing]". Fraco: Submit/Sign Up/Learn More.
3. **Page structure:** Hero → Social Proof → Problem → Solution (3-5 benefícios, não 10) → How It Works (3-4 passos) → Objection Handling → CTA + risk reversal.
4. **Hook = 3 componentes:** visual action (para o polegar) + linha falada (abre o loop) + caption (âncora p/ quem lê sem som). Complementam, nunca repetem.
5. **Confiança:** remover "almost/very/really"; nunca estatística fabricada — mentira em copy é risco legal.
6. **AI-SEO:** 3 pilares — Structure (extraível), Authority (citável), Presence (onde IA olha). Answer blocks 40-60 palavras; citação +40% de chance; keyword stuffing **-10%**.
7. **Julgue contra dados reais, nunca best-practice genérico:** "seu top-10% usa hooks com número (42% dos hits)", não "melhore o hook".

## 6. STACK — como construir

**Princípio: monorepo Cosca, tokens uma vez, tudo conversa.**

```
projeto/                        # monorepo (turborepo/nx)
├── apps/
│   ├── web/                    # Next.js App Router + Tailwind v4 + shadcn/ui
│   │   ├── app/                # file-conventions (layout/page/loading/error/route)
│   │   ├── components/ui/      # shadcn (copy-paste, Radix + cva)
│   │   └── styles/globals.css  # @import "tailwindcss" + @theme {tokens da casa}
│   ├── api/                    # NestJS (backend modular, monólito modular)
│   │   ├── src/modules/        # feature modules (users/, orders/, payments/)
│   │   └── prisma/             # schema Postgres
│   └── studio/                 # Remotion (vídeo programático) + GSAP (animação in-page)
├── packages/
│   └── brand/theme.css         # design tokens ÚNICOS (@theme) — fonte da verdade
└── docs/
```

Regras:
1. **Next.js = file-conventions como API** (URL = pastas + arquivos especiais); server components por padrão, `"use client"` só onde interage; route groups `(group)` + private `_folder`; colocation.
2. **Tailwind v4 = tokens em `@theme {}`** (gera utilities automaticamente: `--color-mint-500` → `bg-mint-500`); tokens compartilhados entre apps via `@import` do `theme.css`.
3. **shadcn = código é SEU** (copy-paste, não biblioteca); Radix (headless) + cva (variantes) + Tailwind (estilo); "é COMO você constrói sua component library".
4. **NestJS = módulos por feature** (imports/controllers/providers/exports); DI por constructor; Guards/Pipes/Interceptors/Filters como DSL.
5. **GSAP = Timeline sequencia, Tween seta**; `useGSAP()` + `gsap.context()` p/ cleanup; `x/y/rotation` (transform, GPU).
6. **Remotion = vídeo como função pura do frame** (`useCurrentFrame` + Composition/Sequence, determinístico); `spring()`/`interpolate()`.
7. **Vercel = git é fonte da verdade** (push → preview; merge na production branch → prod).

Anti-patterns: `defaultProps gigantes no Composition` (usar `calculateMetadata`) · `wrap de biblioteca pra customizar shadcn` · `"use client" em tudo` · `CSS vars em :root onde @theme deveria estar`.

## 7. DADOS — como persistir

**Princípio: a árvore de decisão. SQLite → Postgres → Mongo/Redis. Sobe de nível por medição, não por hype.**

```
Dados canônicos/ACID (dinheiro, ledger, conhecimento)?
├─ single-writer, cabe num nó → Postgres (default; money = numeric; JSONB p/ flexível com GIN)
├─ infra legada do cliente (LAMP) → MySQL (nunca do zero se PG disponível)
└─ multi-nó → Postgres + Citus / CockroachDB
Dados por documento, schema volátil (catálogo, perfil, telemetria)?
└─ Mongo (regra: consistência multi-documento → sobe pra Postgres/JSONB)
Cache/fila/ranking/sessão/rate-limit?
└─ Redis — SEMPRE derivado do canônico, nunca fonte da verdade
Edge/single-file/local-first (o padrão atual da casa)?
└─ SQLite — sobe pra Postgres quando multi-writer real
```

Regras:
1. **Postgres (default):** pgx/v5, `::jsonb`, índices GIN p/ containment; UPDATE em JSONB trava a linha → documentos pequenos; MVCC (leituras não bloqueiam escritas); `EXPLAIN ANALYZE` é a régua P13 do banco.
2. **Mongo:** dado que se acessa junto, se armazena junto (embed > join); `$lookup` não atravessa shard; schema validation `$jsonSchema` SEMPRE.
3. **Redis:** TTL é o schema de cache (`SET key val EX n`); Streams p/ fila (consumer groups); Sorted Sets p/ ranking; rate-limit INCR+EXPIRE.
4. **Migrations forward-only** com compatibilidade; teste de banco em container-local (dockertest).

## 8. INFRA — como evoluir

**Princípio: a casa aprende; o conhecimento é a moeda.**

Regras:
1. **Grafo de codebase consultável** — extração determinística (AST) e semântica (LLM) separadas; edges com confidence (EXTRACTED/INFERRED/AMBIGUOUS); queries `path/explain/affected` (blast radius reverso).
2. **Workflow → skill com gates:** 5 fases (discovery→design→architecture→detection→implementation) + `validate.py`/`security_scan.py`/`run_evals.py` como portões HARD; eval spec = loss function; seção `## Gotchas` viva (só fatos reais, proibido inventar).
3. **Gateway multi-provider:** tiers de prioridade + seleção ponderada aleatória (distribui carga); affinity; multi-key; smoothing anti-starvation; billing pre-charge→ajuste→settlement.
4. **Síntese com contrato (LAWs):** output de pesquisa com contrato explícito (badge, citação renderer-aware, sem headers inventados); enriquecimento pós-busca com métricas REAIS (API gratuita), nunca estimativa do LLM.
5. **Catálogo com frontmatter como fonte da verdade** + schema JSON validando cada tipo + registry gerado.

---

## CHECKLIST — o portão do projeto Cosca

- [ ] `DESIGN.md` criado e lido antes de qualquer pixel
- [ ] Tokens 3 camadas validados por script (contraste, escalas)
- [ ] Estados: hover/disabled/loading/error/empty em toda superfície
- [ ] `prefers-reduced-motion` presente no mesmo código da animação
- [ ] Contraste ≥4.5:1 / ≥3:1 medido (não chutado)
- [ ] Monorepo montado (web+api+studio+shared), tokens únicos
- [ ] Banco escolhido pela árvore de decisão, não por hábito
- [ ] Migrations forward-only + teste de banco em container
- [ ] Copy com fórmula (headline + CTA + hook 3 componentes)
- [ ] Output direto: lead with the action, zero preamble
- [ ] Evidência fresca de cada "está pronto" (P13)
- [ ] Zero slop: bans verificados (Inter, 3 cards, purple, em-dash)

---

## REFERÊNCIAS
ponytail · ui-ux-pro-max · taste-skill · impeccable · design-motion-principles · superpowers · gstack · caveman · humanizer · marketingskills · social-media-skills · graphify · agent-skill-creator · last30days-skill · agent-browser · new-api · buildwithclaude · no-ai-slop · i-have-adhd · Next.js · Tailwind v4 · shadcn/ui · NestJS · GSAP · Remotion · Vercel · Postgres · MySQL · MongoDB · Redis

**Criado:** 2026-08-16, por ordem do Don — *"criar nosso padrão cosca pra nunca mais depender de ninguém"*.
