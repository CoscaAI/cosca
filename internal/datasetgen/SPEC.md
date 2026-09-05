# COSCA Dataset Generator (datasetgen) — SPEC

> Objetivo: transformar o **COSCA em uma fábrica de dados de treinamento**.
> O pacote `internal/datasetgen` sintetiza tarefas procedurais, executa o loop
> de tool-call com o executor canônico (modelo aluno, ex.: `qwen3:4b`),
> captura a trajetória COMPLETA (task → ações → tool_results) e classifica
> automaticamente cada exemplo em 6 classes.
>
> **Professor: DeepSeek-V4 Flash Vision EXP** (o modelo forte). Ele gera a
> tripla `{tarefa, estado_inicial, resultado_esperado}`; o runtime executa o
> agente 4B sobre ela; o classificador rotula o resultado.

---

## 1. Por que existe

O DeepCoder 1.5B provou estar **abaixo do piso de tool-call** (responde em
prosa). O `qwen2.5-coder` 8B **tem** a capacidade (faz `read→edit` em contexto
cirúrgico) mas **oscila**. O objetivo não é re-inventar o COSCA — é **destilar
o comportamento operacional do próprio COSCA** num modelo pequeno, para que ele
siga o protocolo de forma consistente.

O modelo pequeno **não precisa "inventar" como usar tools** — ele precisa
**replicar** o padrão que o professor (DeepSeek-V4) demonstra. O COSCA fornece
todo o resto (runtime, invariante de evidência, memória, DoD).

## 2. Regra de ouro (isolamento)

**O dataset NUNCA deve contradizer o runtime.** Se o runtime exige
`read → edit`, 100% dos exemplos "corretos" respeitam isso. Um único exemplo
onde o "agente correto" faz `edit_file` direto faria o LoRA aprender
estatisticamente que "às vezes pode" — abrindo um buraco no EVIDENCE GATE.

## 3. Tipos de exemplo

O dataset **não treina só casos felizes**. Ele ensina a trajetória, inclusive
o que NÃO fazer. Cada exemplo é uma **trajetória completa**:

```
TASK
  ↓
MODEL ACTION  (tool_call ou prosa)
  ↓
TOOL RESULT
  ↓
MODEL ACTION
  ↓
TOOL RESULT
  ↓
...
  ↓
VERIFICATION
```

### Focos (categorias de comportamento)
- `happy_path` — read → edit → verify → DONE
- `recovery` — erro → re-read → corrigir → success (ensina o reflexo)
- `request_info` — evidência insuficiente → "não agir" / ler / buscar / escalar
- `search_first` — busca/inspeção antes de editar
- `multi_file` — mudanças que atravessam vários arquivos
- `no_dod` — NÃO declarar sucesso sem evidência (constrangimento)

## 4. As 6 classes de rotulagem (classificador)

| Label | Significado | Entra como demo? |
|---|---|---|
| `SUCCESS` | Ferramentas executadas, trabalho feito, passou no check | ✅ demo positiva |
| `RECOVERY_SUCCESS` | Errou → recuperou → terminou certo | ✅ demo positiva (ouro) |
| `FAILURE` | Não completou / desistiu / loop | ❌ contraste |
| `UNSAFE_ACTION` | Tentou editar sem ler / old_string inventado / ação perigosa | ❌ contraste |
| `PROSE_INSTEAD_OF_ACTION` | Respondeu em prosa em vez de chamar tool | ❌ contraste |
| `FALSE_COMPLETION` | Declarou sucesso sem evidência / DoD | ❌ contraste |

## 5. Distribuição-alvo do dataset (o PESO)

Não basta incluir — a frequência define o comportamento dominante:
```
~40%  happy_path
~25%  recovery
~15%  request_info ("não agir")
~10%  search_first
~10%  multi_file
```
Depois complementar com os exemplos de contraste (FAILURE/UNSAFE/PROSE/
FALSE_COMPLETION) recolhidos dos fracassos reais do modelo.

## 6. Formato de saída (JSONL)

Um objeto por linha, serializável para fine-tune:

```json
{
  "task": "Altere Restart() para retornar error.",
  "initial_state": {"runtime.go": "package main\n\nfunc Restart() {\n\t// old\n}\n"},
  "expected": {"runtime.go_contains": "func Restart() error {"},
  "language": "go",
  "focus": "happy_path",
  "agent_model": "qwen3:4b",
  "label": "SUCCESS",
  "trajectory": [
    {"role": "user", "content": "Altere Restart() para retornar error."},
    {"role": "assistant", "tool_calls": [{"name": "read_file", "arguments": {"path": "runtime.go"}}]},
    {"role": "tool", "content": "package main\n\nfunc Restart() {", "name": "read_file"},
    {"role": "assistant", "tool_calls": [{"name": "edit_file", "arguments": {"path": "runtime.go", "old_string": "func Restart() {", "new_string": "func Restart() error {"}}]},
    {"role": "tool", "content": "OK", "name": "edit_file"},
    {"role": "assistant", "content": "Concluído."}
  ]
}
```

## 7. Ciclo de geração (dirigido pelo runtime)

1. **Professor (DeepSeek-V4)** gera a tripla: `{task, initial_state, expected}`.
2. **Runtime** cria um workspace isolado temporário, grava `initial_state`.
3. **Modelo aluno (4B)** roda o loop de tool-call (via `orchestration.Executor`
   + executor canônico com o EVIDENCE GATE ativo).
4. **Capturador** grava cada mensagem (assistant/tool) na trajetória.
5. **Classificador** analisa a trajetória + estado final vs `expected` → projeta
   um das 6 labels.
6. **Verificador** confirma `expected` (ex.: arquivo contém `func Restart() error {`).
7. Escreve o JSONL (positivos e contrastes separados ou em um arquivo só).

## 8. Segurança (fail-closed — Lei do Cofre)

- O gerador NUNCA toca o workspace real — trabalha em um **diretório temporário
  isolado** criado e destruído por exemplo.
- O executor canônico (sandbox + policy + EVIDENCE GATE) é o mesmo dos
  caminhos de produção — **nenhum atalho**.
- Se o professor não estiver disponível (sem API key), o gerador **gera
  tarefas procedurais deterministicamente** (sem LLM) — o dataset continua
  sendo produzido, só sem a "tripla do professor".
