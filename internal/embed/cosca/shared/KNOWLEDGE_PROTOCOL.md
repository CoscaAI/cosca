# KNOWLEDGE PROTOCOL

> **Version**: 1.0.0 | **Status**: active | **Owner**: Cosca Kernel | **Created**: 2026-08-06
>
> Todo agente Cosca DEVE seguir este protocolo antes de trabalhar com qualquer
> ferramenta, biblioteca ou framework. O objetivo é eliminar alucinações:
> o agente só trabalha com o que o Cosca CONHECE.

---

## Regra de Ouro

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                  │
│   ★ NENHUM AGENTE TRABALHA COM FERRAMENTA DESCONHECIDA ★        │
│                                                                  │
│   Antes de escrever uma linha de código usando uma lib,          │
│   framework ou API, o agente DEVE verificar se o Cosca           │
│   tem conhecimento adequado sobre ela.                           │
│                                                                  │
│   Se não tem → adquire. Se não pode adquirir → avisa o Don.     │
│   NUNCA invente APIs, parâmetros, ou comportamentos.             │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

---

## Fluxo para Agentes

### Antes de começar qualquer tarefa

```bash
# 1. Verificar se o Cosca conhece as ferramentas envolvidas
cosca knowledge readiness --detect

# 2. Se houver gaps (unknown ou partial), o readiness mostra os comandos exatos
```

### Quando precisar de informação sobre uma lib

```bash
# Busca FTS5 determinística (zero LLM)
cosca knowledge search "como usar prisma transaction"

# Se não encontrar → conhecimento não existe → NÃO invente
```

### Quando o conhecimento está desatualizado

```bash
# Verificar integridade do knowledge base
cosca knowledge verify

# Se tiver vetores órfãos (chunks sem embedding):
cosca knowledge verify --fix
```

### Quando aparece ferramenta nova no meio do projeto

```bash
# Rodar de novo — o detect pega dependências novas
cosca knowledge readiness --detect
```

---

## Comandos Essenciais

| Comando | Quando usar | O que faz |
|---------|------------|-----------|
| `cosca knowledge readiness --detect` | Antes de qualquer task | Detecta deps do projeto e verifica conhecimento |
| `cosca knowledge readiness --stack "lib1,lib2"` | Projeto novo | Verifica stack manualmente |
| `cosca knowledge search "<query>"` | Precisa de info | Busca FTS5 no knowledge base |
| `cosca knowledge add <lib>` | Lib desconhecida | Registra Knowledge Package |
| `cosca knowledge acquire <lib> --allow-remote --compile` | Precisa de docs | Baixa docs oficiais e indexa |
| `cosca knowledge verify` | Manutenção | Verifica integridade (SQLite + vetores) |
| `cosca knowledge verify --fix` | Vetores quebrados | Repara chunks sem embedding |
| `cosca knowledge match` | Auditoria | Cruza deps do projeto com packages conhecidos |
| `cosca knowledge resolve <task>` | Gap detection | "Tenho conhecimento suficiente para esta task?" |
| `cosca knowledge packages list` | Inventário | Lista todos os Knowledge Packages |
| `cosca knowledge stats` | Diagnóstico | Estatísticas do knowledge base |

---

## Estados de Conhecimento

| Estado | Significado | Ação |
|--------|------------|------|
| `ready` | Validado + docs + vetores | Pode trabalhar |
| `partial` | Manifesto existe, docs pendentes | `cosca knowledge acquire <lib> --allow-remote --compile` |
| `unknown` | Nenhum conhecimento | `cosca knowledge add <lib>` → depois `acquire` |
| `stale` | Conhecimento envelhecido | `cosca knowledge revalidate <lib>` |

---

## Regras para o Kernel (cosca-kernel)

1. **Antes de delegar qualquer task**, o Kernel DEVE rodar `cosca knowledge readiness --detect` (ou chamar `CheckReadiness()` internamente). O `cosca delegate` já faz isso automaticamente.

2. **Se o readiness reportar gaps**, o Kernel NÃO deve delegar a task. Em vez disso, deve sugerir ao Don os comandos para fechar os gaps.

3. **Ao receber uma pergunta técnica**, o Kernel deve primeiro consultar `cosca knowledge search` antes de responder. Se o conhecimento não existir, declarar honestamente "não sei" em vez de alucinar.

4. **O Kernel NUNCA deve delegar uma task que envolva uma ferramenta desconhecida** sem antes adquirir o conhecimento sobre ela.

---

## Regras para Subagentes

1. **Todo subagente DEVE verificar o knowledge base** antes de escrever código usando qualquer API externa.

2. **Se um subagente encontrar uma lacuna de conhecimento**, deve reportar ao Kernel (não ao Don diretamente) com o comando exato para resolvê-la.

3. **Subagentes NUNCA devem inventar APIs, parâmetros, flags ou comportamentos** de bibliotecas que o Cosca não conhece. Se precisarem de algo que não está no knowledge base, devem parar e pedir aquisição.

4. **Subagentes devem preferir `cosca knowledge search`** sobre busca na internet ou chamadas de LLM externo. O knowledge base é a fonte primária de verdade.
