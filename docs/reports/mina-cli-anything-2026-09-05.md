# RELATÓRIO COMPLETO — Mina `HKUDS/CLI-Anything`

> **Mina investigada a fundo (cada pedacinho):** https://github.com/HKUDS/CLI-Anything
> **Data:** 2026-09-05 · **Autor:** Cosca Kernel (por ordem do Don: "investiga cada pedacinho, tira maior proveito, traz relatório completo")
> **Método:** READ-ONLY + clones shallow em temp. Nada no root foi modificado.
> **Licença:** Apache-2.0 (livre para uso). **49k stars · Python · 87 diretórios · 79 CLIs · 151 SKILL.md · 1.037 .py.**

---

## 0. O que é a mina (em 1 frase)

> **CLI-Anything transforma qualquer software em uma ferramenta que um AGENTE usa por CLI estadoful, autodescrita e com feedback visual — chamando o backend real, nunca reimplementando.**

O valor central: **"fazer TODO software agente-native"**. Gera um harness CLI (REPL estadoful + `--json` + preview) para Blender, ComfyUI, Audacity, Obsidian, Ollama, Godot, GIMP... e empacota tudo num **SKILL.md** que o agente lê para saber como dirigir a ferramenta.

---

## 1. OS 8 PADRÕES-CHAVE (o que o COSCA deve aprender)

### Padrão 1 — Skill agente-native = contrato de descoberta
**O padrão:** toda CLI empacota um `SKILL.md` com frontmatter `name` + `description` (a description é o gatilho "use-when"), seguido de Installation (pip + prereqs), Usage (--help/REPL/--json), Command Groups, Examples, State Management, Output Formats, e uma seção **"For AI Agents"** com regras programáticas (sempre `--json`, checar return code, parsear stderr, `--no-stream` para resposta completa, verificar servidor com `server status`).
**Prova:** `ollama/agent-harness/cli_anything/ollama/skills/SKILL.md`
**COSCA aplica:** escrever um `SKILL.md` por ferramenta/capacidade do COSCA (ex: `cosca-ollama`, `cosca-memory`, `cosca-rag`) com esse esqueleto. A description de cada skill = gatilho "use-when".

### Padrão 2 — Geração automática de skill (sem drift)
**O padrão:** o `SKILL.md` é **gerado, não escrito à mão** — `skill_generator.py` faz parse AST dos decorators Click (`@group`/`@command`), lê `setup.py` e `README.md`, e preenche um template Jinja2.
**Prova:** `cli-anything-plugin/skill_generator.py` (dataclasses `CommandGroup`/`CommandInfo`/`Example`).
**COSCA aplica:** gerar/atualizar os `SKILL.md` do COSCA automaticamente a partir dos comandos reais — elimina o drift (o mesmo problema de desatualização que já vimos no README).

### Padrão 3 — CLI estadoful dual-mode
**O padrão:** grupo root Click com `invoke_without_command=True`; sem subcomando → `ctx.invoke(repl)`. **REPL é o default**, one-shot é exceção. Flags globais `--json`/`--host`. Comandos **delegam ao `core/`** (sem lógica no CLI). `@handle_error` captura erros: `{"error","type"}` em JSON, mensagem em human, `sys.exit(1)` fora do REPL, **não aborta a sessão dentro do REPL**. Grupo `session/status` expõe o estado interno para o agente.
**Prova:** `ollama/.../ollama_cli.py`
**COSCA aplica:** todo comando do COSCA suportar 2 modos com `--json` global + um `session status` para o agente ver o estado antes de agir.

### Padrão 4 — Backend real, nunca reimplementação (+ "rendering gap")
**O padrão:** o CLI **chama o software real** (LibreOffice, Blender, Ollama, ffmpeg); gera arquivo intermediário válido e entrega ao backend. **Nunca** degrada para biblioteca fallback. Se o software falta → erro com instrução de instalação. Toda operação tem "filter translation layer" quando o renderer nativo não lê os efeitos.
**Prova:** `ollama/core/generate.py` (`api_post_stream` NDJSON) + `ollama/utils/ollama_backend.py`.
**COSCA aplica:** cada capacidade do COSCA tem wrapper de backend (o blueprint exato é `core/generate.py` + `utils/ollama_backend.py`). Nunca reimplementar inferência/embeddings — sempre via API real + streaming NDJSON.

### Padrão 5 — Namespace packaging (extensibilidade sem conflito)
**O padrão:** `cli_anything/` **sem `__init__.py`** (namespace package PEP 420); cada subpacote tem o seu. Permite N pacotes PyPI independentes coexistirem. `setup.py` com `find_namespace_packages(include=["cli_anything.*"])`.
**Prova:** `ollama/agent-harness/setup.py`
**COSCA aplica:** estruturar módulos/plugins do COSCA sob namespace compartilhado (`cosca.<modulo>`) sem `__init__.py` na raiz — skills/tools instaláveis de forma independente.

### Padrão 6 — Registry manifesto (o contrato de instalação)
**O padrão:** `registry.json` + `public_registry.json`. Cada entrada carrega o que o agente precisa para decidir/instalar: `name, display_name, version, description, requires, homepage, install_cmd, entry_point, skill_md, category, contributors` (+ `package_manager, install_strategy` para públicos). `install_cmd` usa `pip install git+...#subdirectory=<sw>/agent-harness`.
**Prova:** `registry.json` (79 CLIs), `public_registry.json`.
**COSCA aplica:** criar um `cosca-registry.json` com esse schema por skill/ferramenta — base para um "COSCA-Hub".

### Padrão 7 — Package manager como mercado de skills (cosca-hub)
**O padrão:** `cli-hub install <name>`: busca no registry (cache `~/.cli-hub`, TTL 1h, merge dos 2 registries com tag `_source`), decide a estratégia (`pip|npm|uv|bundled|command`), despacha para o handler certo, grava estado. Comandos: `list`, `search`, `info`, `install`, `update`, `uninstall`, `launch`, e **`can <query>`** (responde "posso fazer X?"). Telemetria anônima com detecção se o chamador é agente.
**Prova:** `cli-hub/cli_hub/{installer,registry,cli}.py`
**COSCA aplica:** construir um `cosca-hub` que instala skills/tools por estratégia, com `list/search/info/install/launch/can`. Um agente do COSCA roda `cosca-hub install <tool>` e lê o `SKILL.md`.

### Padrão 8 — Preview-feedback loop + matrizes de capacidade
**O padrão (o maior ativo):** **Preview** fecha o loop de feedback — **produtor** (`cli-anything-<sw> preview`, fala com o backend real) vs **consumidor** (`cli-hub previews inspect|html|watch|open`, só leitura). Persistência em 3 camadas: `bundle_dir` (snapshot imutável), `session.json` (head atual), `trajectory.json` (histórico append-only comando→preview). Princípio da *truthfulness*: nada de fake render. **`matrix_registry.json`** modela workflows de domínio como blocos de capacidade, onde cada capacidade lista múltiplos provedores (`kind`, `cost_tier`, `quality_tier`, `offline`).
**Prova:** `HARNESS.md` preview norms, `preview_bundle.py`, `matrix_registry.json`.
**COSCA aplica:** para tarefas iterativas, expor um comando de **preview** que publica estado intermediário honesto + um **consumidor** que mostra ao agente o resultado antes do próximo passo (bundle/session/trajectory). Para workflows multi-etapa, criar **matrizes de capacidade** (provedores com custo/qualidade/offline) = oráculo de "quais ferramentas existem e qual é a melhor".

---

## 2. OS 5 HARNESSES DE MAIOR VALOR PARA O COSCA

| Harness | Domínio COSCA | Capacidades-chave | O que o COSCA ganha | Valor |
|---------|--------------|-------------------|---------------------|:-----:|
| **blender** | 3D (ponte já existe) | render duplo hero+workbench → `manifest.json` com `summary.next_actions`, `metrics`, `cache_key` + live session | Loop de edição com **feedback visual verdadeiro** | **ALTO** |
| **comfyui** | nodegraph/media | node graph validado (`class_type`/`inputs`), inventário tipado de models, ciclo prompt→status→history→images | Camada headless de geração de imagem para o nodegraph | **ALTO** |
| **audacity** | áudio/voz | motor de áudio em **Python puro** (JSON→WAV) com registry de 16 efeitos por faixa | Percepção/processamento de voz sem binário | **ALTO** |
| **zotero/obsidian/siyuan** | conhecimento/PIM | Zotero `item context→prompt_context` (contrato RAG); Obsidian busca DQL/JsonLogic | Contratos de RAG e busca de conhecimento | **ALTO** |
| **browser/DOMShell** | visão/percepção | **MCP-as-backend**: Accessibility Tree do Chrome exposta como filesystem (`ls/cd/cat/grep/click`) | Percepção navegável → feedback verdadeiro de visão/áudio | **ALTO** |

> **A ponte mais valiosa:** o protocolo `manifest.json` + `summary.next_actions` é o **contrato de feedback de percepção** que o COSCA pode adotar. E o padrão "JSON-state → gera artefato/render" se porta direto para a ponte Blender e a camada de áudio.

---

## 3. ECOSSISTEMA (como o COSCA aproveita)

### O cli-hub (marketplace)
- **Catálogo + package manager**: `registry.json` (harness oficiais) + `public_registry.json` (terceiros: yt-dlp, ffmpeg, spotdl). Cache local `~/.cli-hub` (TTL 1h), merge com tag `_source`.
- **Ciclo**: `list` (filtra por categoria), `search`, `info`, `install`, `update`, `uninstall`, `can`, `matrix {list,search,info,preflight,install,doctor,recipes}`, `previews`. Tudo com `--json`.
- **Estratégias**: `installer.py` despacha por estrategia (`pip` p/ harness; `npm`/`uv`/`command`/`bundled` p/ públicos). Estado em `installed.json`.

### As "skins" de editor (o padrão mais valioso p/ COSCA)
- **1 fonte canônica** (`cli-anything-plugin/`: HARNESS.md + skill_generator.py + preview_bundle.py + templates) e **8 adaptadores finos**:
  - `.cursor-plugin/`, `.claude-plugin/` → `marketplace.json`
  - `codex-skill/`, `hermes-skill/`, `reasonix-skill/` → meta-skills que **vendoram** os recursos canônicos
  - **`opencode-commands/*.md`** → 5 comandos em `.md` (formato NATIVO do OpenCode, onde o COSCA roda)
  - `qoder-plugin/`, `cli-anything-plugin/commands/`
- **COSCA aplique:** temos **88 skills**! Podemos gerar o **mesmo conjunto de skins de editor** a partir de uma fonte única (`cosca-plugin` canônica) → `{cursor, claude, codex, hermes, reasonix, qoder, opencode}`. As skills do COSCA ficam "marketplace-able" e **descobríveis em batch** (`--json`).

---

## 4. DADO DE TREINO para o LoRA (o que o Don quer: treinar melhor)

### Censo honesto do corpus
| Camada | Arquivos | Tokens (aprox) | Papel |
|--------|---------:|---------------:|-------|
| **SKILL.md** (canônicas) | 151 | ~233K | Contrato de tool-use por software |
| **SOP de análise** (`<SOFTWARE>.md`) | 86 | ~112K | Como destilar arquitetura→CLI |
| **TEST.md** (planos + resultados) | 58 | ~97K | Happy-path invariants verificados |
| **`test_*.py`** | 164 | ~781K | Testes de comportamento |
| Código `cli_anything/` | 1.240 | ~2.7M | Código real do harness (baixo valor p/ comportamento) |
| **matrix_registry.json** | 1 | ~27K | Orquestração multi-ferramenta |
| **cli-hub-matrix/** | 13 | ~62K | Profundidade por domínio |
| **plugin/** | 26 | ~54K | Metodologia/meta-skill |

**Corpus descritivo/de comportamento ≈ 334 arquivos, ~2 MB, ~500K tokens.** (Código = +2.7M, mas baixo valor para LoRA de comportamento.)

### ⚠️ Avaliação HONESTA (o ponto crítico)
- **NÃO é um dataset de trajectories** — não há pares `(prompt→tool_call→observation→resposta)`. São contratos + código + testes. As evals (`eval/`) só existem em **2 de ~78 harnesses**.
- **MELHOR USO = RAG** (não SFT direto): os `SKILL.md` são literalmente o *system prompt* de uma ferramenta. **~500K tokens** cabem no índice → embed + inject no `semantic_router`/RAG que o COSCA já tem. → resolve *"não sei como operar a ferramenta X"*.
- **Bom para SINTETIZAR SFT:** os `TEST.md` (workflows reais: "photo editing", "collage", "batch filter") + seções "For AI Agents" + recipes do matrix são exatamente o que precisa para **gerar programaticamente** pares `(instrução → sequência de tool_call → JSON esperado)`. → resolve *"erro no happy path"* (o problema exato do LoRA atual).
- **Riscos:** skills geradas por template = **redundância alta** (seções verbatim entre softwares) → LoRA pode memorizar o esqueleto em vez de generalizar. **Limpar seções verbatim antes.** O fallback `description` "Execute <func> operation." é ruído — pode/limpe.

### Recomendação concreta para o LoRA 002
1. **Camada 1 (RAG, imediato):** embed `skills/**/SKILL.md` + `matrix_registry.json` + `cli-hub-matrix/**` + `cli-anything-plugin/guides/preview-methodology.md`.
2. **Camada 2 (SFT sintetizado):** derivar dos `TEST.md` + "For AI Agents" + matrix recipes a sequência correta de tool_calls e o JSON esperado, usando o harness real como oracle. **Mire ~10-20K pares sintéticos de 3-5 tool_steps** (relevância > quantidade).
3. **Ignorar** `test_*.py`/código como SFT de comportamento (só se criar um LoRA de geração de harness).

---

## 5. CONCLUSÃO E CAMINHO PROPOSTO

### O que o COSCA ganha desta mina (maior proveito)
1. **O melhor `SKILL.md`-as-contract** (padrões 1-4) — como estruturar uma skill agente-native para as 88 skills do COSCA.
2. **O `Preview-feedback loop`** (padrão 8) — a metade da ferramenta do loop agente↔real; o COSCA já tem a metade do modelo (Perception Bus). **São as 2 metades do mesmo loop.**
3. **O `matrix` de capacidades** — ouáculo determinístico de seleção de ferramenta (custo/qualidade/offline/fallback), complementar ao Semantic Router por embedding.
4. **A "skin de editor"** — expor as 88 skills do COSCA como marketplace (cursor/claude/codex/opencode), fonte única + gerador.
5. **Dado de treino/contexto** — ~500K tokens de skills para RAG, e a base para sintetizar pares SFT que atacam o "happy path" do LoRA.

### Caminho (prognoses, decisão do Don)
- **Imediato (RAG):** ingestar as `skills/**/SKILL.md` + matrix no conhecimento do COSCA → busca semântica fica muito mais rica em "como operar ferramentas".
- **Médio (SFT 002):** sintetizar pares de SFT dos TEST.md para treinar melhor o LoRA (o que o Don quer).
- **Opcional (cosca-hub):** criar a fonte canônica `cosca-plugin` + gerador de skins para as 88 skills serem marketplace-able.

---

> **"A mina não é só código — é um padrão de como ensinar o agente a usar o mundo. O COSCA já é o cérebro; esta mina ensina as mãos."**
