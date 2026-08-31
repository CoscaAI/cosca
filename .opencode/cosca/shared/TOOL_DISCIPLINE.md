# Tool Discipline — Disciplina Cognitiva de Ferramentas (COSCA)

> **Filosofia central:** *não é "qual tool usar", é "qual operação é epistemicamente válida para a intenção atual".*
> O erro do agente que sobrescreveu um arquivo quando devia editar **não é falta de capacidade de programação — é falha de disciplina operacional**.
>
> **Invariante (regra de ouro):** `write` **NÃO é ferramenta de edição.** `edit` é a ferramenta de edição.

Este documento complementa o `TOOL_EXECUTION_POLICY.md`. Aquele define o **contrato operacional** (como a tool executa: ciclo de vida, segurança, timeout, retry). Este define o **contrato cognitivo** (quando usar cada tool em função da intenção). Juntos formam a governança completa de tools do COSCA.

---

## 1. A pergunta que o agente sempre se faz (antes de escolher a tool)

```
"Qual é a minha intenção ACONTECENDO?"
```

| Intenção | Tool | Epistemologia |
|---|---|---|
| **Descobrir arquivos** | `glob` | ONDE existe? |
| **Descobrir onde algo está** | `grep` | QUAL símbolo/conteúdo? |
| **Compreender um arquivo** | `read` (+ `lsp`) | O QUE ele é? |
| **Modificar um trecho existente** | `edit` | ALTERAR |
| **Criar arquivo realmente novo** | `write` | GERAR |
| **Mover/renomear/executar** | `bash` | OPERAR |
| **Provar a alteração** | `read` + `git diff` + `grep`/`lsp` + `test`/`build` | VERIFICAR |

> **Regra mental:** `glob` → ONDE? · `grep` → QUAL? · `read` → O QUE? · `edit` → ALTERAR · `write` → GERAR.
> **A mais importante:** `write` não é edição. `edit` é edição.

---

## 2. Fase 0 — Descobrir antes de tocar (regra absoluta)

> **Nunca editar um arquivo cujo caminho não foi confirmado.**

```
pedido
  ↓
glob     → descobrir arquivos candidatos
  ↓
grep     → confirmar conteúdo / referência
  ↓
read     → ler o arquivo alvo
  ↓
confirmar identidade
  ↓
editar
```

**Exemplo (pedido: "altere o registry"):**
- ERRADO: `write internal/registry.go`
- CERTO: `glob **/registry.go` → achar candidatos → `grep` por função/tipo → `read` o arquivo correto → `edit`.

**Isso evita exatamente** o problema do agente criar um arquivo novo em vez de editar o existente.

---

## 3. Fase 1 — Verificar existência antes de `write`

> **Antes de `write`, rode `glob` no path.**
> Se encontrou → **STOP**: não criar, ler o existente, decidir se é `edit`.

```
glob <path>
  ├── encontrou → NÃO criar → read → edit
  └── não achou  → verificar equivalência → só então write (e só se exigido)
```

**Criação de arquivo novo exige justificativa.** O agente deve conseguir responder:
1. O arquivo existe? 
2. Existe um equivalente que deveria ser alterado? 
3. A arquitetura pede uma nova unidade? 
4. Foi explicitamente solicitado este arquivo? 
5. Algum código existente deveria mudar em vez disso?

Se qualquer resposta for incerta → **não escreve**.

---

## 4. Protocolo READ → EDIT → READ BACK (para qualquer alteração)

```
READ
 ↓
IDENTIFY
 ↓
EDIT
 ↓
READ BACK
 ↓
VERIFY
```

> **Nunca:** `pedido → write`.

### Invariantes técnicas confirmadas (no OpenCode — fatos do código-fonte):

**`edit`** (substituição exata):
- **Exige** `read` do arquivo na mesma sessão antes de editar. Sem `read` prévio → **erro** ("You must use your Read tool at least once before editing").
- Falha se `oldString` **não existe** no arquivo.
- Falha se `oldString` existe **múltiplas vezes** — exigir mais contexto (`oldString` maior/único) ou usar `replaceAll`.
- Preserva **indentação exata** como aparece **depois** do prefixo de linha (`1: `). Nunca incluir o prefixo no `oldString`/`newString`.
- `replaceAll` para renomear/repetir em todo o arquivo.

**`write`**:
- **Sobrescreve** o arquivo existente no path.
- **Exige** `read` do arquivo na mesma sessão antes — sem `read` prévio → **erro**.
- **Preferir `edit`** para arquivos existentes; `write` só para arquivos novos ou explicitamente exigido.
- Nunca criar arquivos de documentação (`*.md`/README) proativamente — só se explicitamente solicitado.

> **Principal takeaway:** o COSCA **deve** implementar o mesmo guard determinístico — `write`/`edit` sem `read` prévio é **operação inválida**. Isso protege o código independente do modelo (a segurança NÃO pode depender do LLM).

---

## 5. Proteção contra "arquivo parecido" (o perigo das duplicatas)

```
internal/
 ├── knowledge/engine.go
 ├── knowledge/engine_test.go
 ├── knowledge/engine_legacy.go   ← parecido!
 └── runtime/engine.go            ← parecido!
```

O agente **não pode** pensar *"engine.go provavelmente é esse"*. Tem que **provar**:

```
path + package + symbol + conteúdo + referências
```

> **Nome de arquivo é pista, não identidade.** A identidade é determinada pelo **conteúdo e contexto arquitetural**, não pelo nome.

Isso previne criar/editar o arquivo errado (ex: `engine_legacy.go` em vez de `engine.go`).

---

## 6. `glob` corretamente (descoberta estrutural, não "procure parecido")

- Descobrir arquivos: `**/*.go`, `internal/**/*.go`, `internal/knowledge/**/*.go`, `**/*_test.go`.
- Depois **restringir**. `glob` retorna caminhos (ONDE), não conteúdo.

## 7. `grep` corretamente (encontrar significado)

- Ex: `grep "BackfillEpistemic"` → depois `read` o arquivo.
- **NÃO** fazer `grep → assumir que achou tudo → editar`. Encontrar um símbolo não garante ter achado a implementação certa (pode haver `interface`/`implementation`/`test`/`mock`/`adapter`/`legacy`).

## 8. `read` corretamente (range reading + binário)

O `read` do OpenCode expõe:
- `filePath` (absoluto ou relativo), `offset` (linha 1-indexed) e `limit` (máx. 2000 linhas) → **ler trechos, não o arquivo inteiro**.
- **Detecção binária** e limite de bytes (`MAX_BYTES ≈ 50KB`) → não despeja binário no contexto.
- "Did you mean one of these?" → quando o path não existe, oferece candidatos próximos.
- Lê também imagens suportadas e **lista diretórios** (entradas).

## 9. `lsp` quando disponível (melhor que grep para código estruturado)

`goToDefinition`, `findReferences`, `goToImplementation`, `documentSymbol`, `incomingCalls`, `outgoingCalls` (experimental).

```
arquivo desconhecido → glob
símbolo desconhecido → grep
símbolo confirmado   → lsp
                      → read
                      → edit
```

---

## 10. Nunca editar no escuro (regra do manifesto)

> **Não modificar código que não foi compreendido.**

Se o agente não sabe: **quem chama · quem é chamado · qual interface implementa · qual teste cobre · qual estado modifica** — ele está em fase de **exploração**, não de edição.

---

## 11. Mudança mínima

> **A menor alteração que satisfaz o objetivo é a preferencial.**

Mudar `func Search(...)` não deve refatorar `Search/Engine/Repository/Adapter/CLI/Tests` só porque encontrou oportunidade. Isso reduz: regressões, diff desnecessário, conflitos, dificuldade de auditoria e risco de destruir arquitetura.

---

## 12. Contrato mental pós-operação (TARGET → PRE → MUTATION → POST)

```
TARGET:      internal/knowledge/engine.go
PRE:         arquivo existe · função Search existe · versão lida == disco
MUTATION:    edit
POST:        função contém a mudança · arquivo original continua sendo o alvo
             · nenhum arquivo inesperado apareceu · diff == intenção · tests passam
```

---

## 13. Roadmap de credenciais (T0–T7)

```
TOOL DISCIPLINE
├── T0 Discovery          glob · grep · path identity
├── T1 Reading            read · range reading · context reconstruction
├── T2 Code Intelligence  lsp · definitions · references · call graph
├── T3 Mutation           edit · patch · write
├── T4 Mutation Safety    READ-before-EDIT · EXISTENCE-before-WRITE · exact target · minimal diff
├── T5 Verification       read-back · git diff · grep/lsp · tests/build
├── T6 Recovery           unexpected file · wrong target · failed patch · rollback
└── T7 Governance         permissions · scope · provenance · audit · capability boundaries
```

**Não é preciso inventar tools novas.** O COSCA já fornece (e pode expor) `read`, `edit`, `write`, `glob`, `grep`, `bash`, `lsp`, `patch`/`apply_patch`. O que falta é a **política cognitiva de uso** — e isso pode ser restrito por agente (`allow` / `ask` / `deny`) conforme a capability.

---

## 14. As 4 invariantes que todo agente COSCA memoriza

1. **`edit` sem `read` prévio é operação inválida.**
2. **`write` sem `read` prévio é operação inválida** — `write` sobrescreve e **não** é ferramenta de edição.
3. **Nunca editar um arquivo cujo caminho não foi confirmado** (glob → grep → read → edit).
4. **Não modificar código que não foi compreendido.**

---

## 15. A inclinação que distingue o COSCA

> O COSCA não precisa aprender *"use Edit em vez de Write"*. 
> O COSCA deve aprender **"qual operação é epistemicamente válida para a intenção atual?"**
>
> — *"quero encontrar"* → glob/grep · *"quero compreender"* → read/lsp · *"quero modificar"* → edit/patch · *"quero criar"* → write · *"quero provar"* → read/diff/test · *"não tenho certeza do alvo"* → **não mutar ainda**.

**Frase-chave:** *velocidade depois de segurança e certeza do alvo.*

---

*Proveniência: síntese da conversa entre o Don e o professor sobre Tool Discipline, validada com os fatos do código-fonte do OpenCode (`packages/opencode/src/tool/read.ts`, `edit.txt`, `write.txt`, `glob.ts`, `grep.ts`). Complementa o `TOOL_EXECUTION_POLICY.md`.*
