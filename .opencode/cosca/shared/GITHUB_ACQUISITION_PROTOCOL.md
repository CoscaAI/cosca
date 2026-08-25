# GITHUB ACQUISITION PROTOCOL — Padrão Cosca

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Cosca Kernel | **Criado**: 2026-08-24
>
> Fiel ao princípio da casa: **NUNCA copiar.** Pesquisar → baixar → explorar → entender →
> **registrar o CONHECIMENTO** na casa, com proveniência linkada ao repositório (para re-semeio).
> O repositório é **INSUMO DE ESTUDO, NÃO MODELO.** Nada é instalado/copiado no projeto
> (`KNOWLEDGE ≠ DEPENDENCY ≠ CODE`).

## PRINCÍPIO (o mandamento do Don)

Todo repositório externo que a casa estuda passa por este protocolo **ANTES** de ter qualquer
conteúdo absorvido. A ordem é sacrossanta:

```
PESQUISAR → AVALIAR → PROTEGER → BAIXAR → EXPLORAR → ENTENDER → REGISTRAR
                    (nunca copiar código; sempre melhorar do nosso jeito)
```

> **REGRA DE OURO**: a casa **nunca** copia código de um repo. Ela aprende o PADRÃO e
> reescreve do nosso jeito, **melhorando**. Se um trecho for copiado → **VIOLAÇÃO** → abortar.

---

## FASE 1 — PESQUISAR (encontrar candidatos)

### 1.1 Pesquisa no GitHub
Pesquisa por tópico, ordenada por relevância/estrelas:

```
https://github.com/search?q=<termos>&type=repositories&s=stars&o=desc
```

- `q=<termos>` — o que se busca (ex: `ui ux pro max`)
- `s=stars&o=desc` — ordena por estrelas (desc) = mais adotadas primeiro
- `type=repositories` — busca por repositórios (não código/issues)

### 1.2 Critérios de triagem (o "se atenta")
| Critério | Pergunta | Gate |
|---|---|---|
| **Estrelas** | É adotado? Tem tração real? | `≥ 1.000` (forte); `100–999` (validar manualmente); `< 100` (desconfiar) |
| **Escopo** | Resolve EXATAMENTE o problema da casa? | Escopo ≈ problema → candidato |
| **Manutenção** | Commits recentes (≤ 6 meses)? Issues abertas seguidas? | Abandonado → descartar |
| **Ecossistema** | Mesma stack da casa (Go/TS/Python)? | Stack compatível → prioriza |

> Orçamento: sempre consultar `cosca acquisition budget check` **antes** de aprofundar.
> A casa **não** entra em espiral (não sei → busca → não sabe → busca mais).

---

## FASE 2 — AVALIAR (licença & restrições)

**ANTES de usar padrões, verificar a LICENÇA.** Decide o que a casa pode aprender:

| Licença | Pode ver código | Pode aprender padrão | Pode copiar | Veredito da casa |
|---|---|---|---|---|
| **MIT / Apache-2.0 / BSD** | ✅ | ✅ | ✅ (mas NUNCA copiamos) | ✅ **Seguir** |
| **GPL / AGPL** | ✅ | ✅ (estudar padrão) | ❌ (copyleft contamina) | ⚠️ Estudar, **não** derivar código |
| **Proprietária / não-liberada** | ❌ | ⚠️ só comportamento | ❌ | ❌ **Pular** |
| **Sem licença** | ⚠️ (default: todos os direitos reservados) | ⚠️ inseguro | ❌ | ❌ **Descartar** |

> **Regra**: aprender o PADRÃO (ideia/protocolo) de qualquer licença permissiva. **Nunca** derivar
> código de copyleft. Verificar o `LICENSE`/`COPYING` do repo ANTES de qualquer etapa de código.

---

## FASE 3 — PROTEGER (segurança antes de entrar na casa)

**Nada entra na casa sem passar na barreira de segurança.** Três camadas:

### 3.1 Vulnerabilidades de dependência
```bash
cosca security scan          # OSV.dev — CVEs/GHSA/OSV das deps
```
- Repo com dependências vulneráveis conhecidas → **quarentena** até fonte segura.

### 3.2 Código malicioso (vírus/backdoor)
| Sinal | Ação |
|---|---|
| Script no `preinstall`/`postinstall` baixando e executando | 🔴 **Alerte** — investigar antes |
| Código ofuscado/base64 no build | 🔴 **Alerte** |
| Referência a domínio/IP suspeito | 🔴 Investigar |
| `package.json`/`pom.xml`/`go.mod` com dep de origem duvidosa | 🟠 Validar |
| Renomeação de binário/executável suspeita | 🔴 **Bloquear** |

### 3.3 Fail-closed do cosca
- `cosca knowledge acquire` exige **`--allow-remote` (obrigatório)** — sem a flag, **recusa** (fail-closed).
- Verificação de segurança do repo (ex: muitos issues abertos) pode **bloquear** — para forçar, só
  com `--force` **explícito** (e isso é decisão consciente, não padrão).

> **Regra de Ouro da Proteção**: na dúvida → **NÃO entra.** A casa prefere perder um conhecimento
> a comprometer a integridade. Segurança > aquisição.

---

## FASE 4 — BAIXAR (conhecer, sem instalar)

### 4.1 Registrar o link (o que permite re-semeio)
```bash
cosca knowledge add github:<org>/<repo>     # manifest (kind: library + repository)
```
> O manifesto grava `repository: "<org>/<repo>"` — a **âncora de proveniência**.
> `KNOWLEDGE ≠ DEPENDENCY ≠ CODE` — registra o vínculo, **nada** é instalado no projeto.

### 4.2 Adquirir a documentação oficial
```bash
cosca knowledge acquire <id> --allow-remote --compile
```
- `--allow-remote` (obrigatório) — busca da internet.
- `--compile` — indexa no Knowledge Base (FTS5) → busca local ativada.
- `--embed` — gera embeddings (busca semântica/vetorial), opcional.

> Para **padrões de código** (não só docs), a casa **clona** o repo num dir temporário
> (`[TEMP]`) para EXPLORAR, mas **o clone NUNCA entra** no diretório do projeto.

---

## FASE 5 — EXPLORAR (ler, não copiar)

Clonar em diretório temporário (`temp`) — **fora do workspace do projeto**.

```bash
git clone https://github.com/<org>/<repo> <TEMP>/minerar-<repo>
```

Explorar com ferramentas da casa (Glob/Grep/Read) procurando **padrões**:
- Estrutura de pastas (architecture)
- Convenções de teste (nomenclatura, organização)
- Config (tsconfig/go.mod/eslint — como configuram)
- Nomeação de módulos/serviços/controllers

> **REGRADE OURO**: explorar LÊ o código para ENTENDER o padrão. **NÃO** copia.

---

## FASE 6 — ENTENDER (extrair padrão, do nosso jeito)

Extrair **o QUE resolveu bem** e **como faríamos MELHOR**:

| Extração | Pergunta |
|---|---|
| **Padrão** (aprender) | O que ele faz bem que a casa devia saber? |
| **Contra-padrão** (não copiar) | O que ele faz mal/desatualizado? |
| **Melhoria** (nosso jeito) | Como reescrever melhor, com padrão Cosca por cima? |

> **NUNCA**: "aqui está o código, vamos usar igual."
> **SEMPRE**: "aqui está o padrão, vamos escrever **do nosso jeito**, melhor."

---

## FASE 7 — REGISTRAR (conhecimento na casa, com proveniência)

Registrar o **aprendizado** como conhecimento durável e **linkado ao repositório** (re-semeio):

### 7.1 Documentar a descoberta (Knowledge Base)
```bash
cosca knowledge search "<padrão>"            # confirma que ficou busável
cosca knowledge packages show <id>           # confirma repository + status acquired
```

### 7.2 Registrar a lição (proveniência)
Adicionar ao ledger de proveniência (`.cosca/provenance.yaml`) uma entrada `kind: knowledge`
com:
- `source: <org>/<repo>` (o repositório de onde veio o padrão)
- `evidence` / `repository` / `commit` / `sha256` — a **cadeia de re-semeio**
- `confidence` — a força do aprendizado

> **O re-semeio**: porque gravamos `repository` + `commit` + `sha256`, se o banco zerar ou a
> evolução apagar o conhecimento, a casa **re-semeeia de volta** a partir do link — com
> verificação de integridade.

---

## ÁRVORE DE DECISÃO (resumo)

```
Candidato (repo)
   │
   ├─ F1 PESQUISAR: estrelas≥1k? escopo certo? manutenção? ── NÃO → descartar
   │        │SIM
   ├─ F2 AVALIAR: licença permissiva? ── NÃO (GPL/proprietária) → só estudar padrão, não derivar
   │        │SIM (MIT/Apache/BSD)
   ├─ F3 PROTEGER: sem CVE? sem malware? ── NÃO → QUARENTENA
   │        │SIM
   ├─ F4 BAIXAR: knowledge add github:<org>/<repo> + acquire --compile
   │        │
   ├─ F5 EXPLORAR: clone em TEMP, ler padrões (não copiar)
   │        │
   ├─ F6 ENTENDER: extrair padrão + contra-padrão + melhoria (nosso jeito)
   │        │
   └─ F7 REGISTRAR: knowledge search ✓ + provenance (repository/commit/sha256)
        → CONHECIMENTO NA CASA, re-semeável. NADA copiado.
```

---

## ELIMINAÇÃO

| Etapa | Ferramenta | Gate |
|---|---|---|
| Não estoura orçamento | `cosca acquisition budget check` | ✓ Dentro |
| Registra link | `cosca knowledge add github:<org>/<repo>` | manifesto `repository` setado |
| Baixa doc | `cosca knowledge acquire <id> --allow-remote` | `Status: acquired` |
| Indexa | `cosca knowledge acquire <id> --compile` | `Indexado (FTS5)` |
| Busável | `cosca knowledge search "<padrão>"` | resultados com proveniência |
| Proveniência | `cosca knowledge packages show <id>` | `repository` + `acquired_at` |
