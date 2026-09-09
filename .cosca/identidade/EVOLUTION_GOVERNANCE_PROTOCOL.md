# PROTOCOLO DE GOVERNANÇA DA EVOLUÇÃO

> **Versão**: 1.0.0 | **Status**: active | **Dono**: Cosca Kernel + Don | **Última atualização**: 2026-09-09

## PROPÓSITO

Definir o caminho obrigatório de toda evolução do Cosca: criação de agentes, skills,
engines, departments, workflows, protocolos e melhorias. `.cosca/` é a **fonte curada**:
nada entra sem verificação da casa e aprovação explícita do Don. O editor propõe;
a casa julga; o Don autoriza.

**Autoridade suprema**: [CONSTITUTION.md](CONSTITUTION.md) — os 8 princípios imutáveis.
**Língua oficial**: PT-BR (língua do Don) — todo documento, skill e protocolo da casa.

---

## 1. PRINCÍPIOS

1. **`.cosca/` é a fonte curada.** O que está lá é o que a casa abençoou.
2. **O editor é proponente, não autor.** Cria fora da fonte; nunca escreve direto em `.cosca/agents|skills|...`.
3. **Evidência promove; opinião não.** Nenhuma skill/agente entra por "parece bom" — passa por medição ou avaliação estruturada.
4. **O Don é o único autorizador.** Nenhuma promoção sem seu aval explícito.
5. **O embed é build derivado.** Evolução flui `.cosca/` → `make embed-sync` → `internal/embed/cosca/`. Nunca o inverso.
6. **Toda promoção deixa rastro.** Chain assinada, catálogo regenerado, memória registrada.

---

## 2. TIPOS DE EVOLUÇÃO × RIGOR

| Tipo de mudança | Exemplo | Rigor | Verificação |
|---|---|---|---|
| **Criar agente novo** | novo Chief/Specialist | 🔴 Alto | Conformidade + avaliação qualitativa + Don |
| **Criar skill nova** | skill de segurança minerada | 🔴 Alto | `skill validate` + avaliação qualitativa + Don |
| **Melhorar skill existente** | refinar corpo de skill | 🟠 Médio | A/B benchmark (`skill evolve`/`benchmark`) + Don |
| **Criar engine/department/workflow** | novo motor | 🟠 Médio | Conformidade + revisão de arquitetura + Don |
| **Protocolo/padrão novo** | novo protocolo da casa | 🟠 Médio | Revisão de governança + Don |
| **Learnings (memória)** | aprendizado pós-tarefa | 🟢 Gatilho | `cosca memory register` → assinado na chain |
| **Correção de link/caminho** | manutenção | 🟢 Leve | `cosca gate catalog --check` + commit |

**A/B benchmark aplica-se a melhoria de skill existente** (há baseline para comparar).
**Avaliação qualitativa estruturada aplica-se a criação nova** (sem baseline): Chief de
domínio avalia valor, aderência e duplicação via busca semântica.

---

## 3. O FLUXO — 6 FASES

```
┌─ FASE 0 · CRIAÇÃO (editor/agente — FORA da fonte)
│   Branch git `proposal/<nome>` (promoção = merge aprovado)
│   → novo agente: PROMPT.md + INDEX.md · skill: SKILL.md + eval.yaml
│   → docs: .md em PT-BR · padrão da casa no cabeçalho
│
├─ FASE 1 · SUBMISSÃO (proponente → Kernel)
│   "Candidato X submetido à casa" — diff, propósito e evidência de valor
│
├─ FASE 2 · VERIFICAÇÃO DE CONFORMIDADE (casa — automática)
│   • `cosca skill validate` — padrão Agent Skills
│   • Invariantes do catálogo (INDEX.md, links vivos, sem mojibake)
│   • Idioma PT-BR · sem segredos · sem duplicação
│   • REPROVOU → volta ao proponente com o diff (não entra)
│
├─ FASE 3 · VERIFICAÇÃO DE VALOR (casa — Chiefs)
│   • Melhoria de skill: A/B benchmark (mediana+IQR, nunca média)
│   • Criação nova: avaliação qualitativa estruturada (Chief de domínio +
│     busca semântica de duplicação/sinergia)
│   • Engine/workflow: revisão de arquitetura
│   • INCONCLUSIVO → não promove (registrado)
│
├─ FASE 4 · GATE DO DON (único autorizador)
│   • `cosca approve` — aprova/rejeita com o pacote de evidência
│
├─ FASE 5 · PROMOÇÃO PARA .cosca (mecanismo nativo)
│   • Skill → `cosca skill install`
│   • Agente/engine/doc → merge do branch + `cosca gate catalog --generate`
│
└─ FASE 6 · REGISTRO E SINCRONIA
    • Chain assinada (imutável, com testemunho)
    • Catálogo regenerado · memória registrada via CLI
    • Material canônico → `make embed-sync` (fonte → build)
```

---

## 4. PAPÉIS

| Papel | Quem | Responsabilidade |
|---|---|---|
| **Proponente** | editor/agente | Cria o rascunho e submete com evidência |
| **Verificador de conformidade** | casa (gates automáticos) | Padrão, invariantes, PT, duplicação |
| **Verificador de valor** | Chiefs (review/architecture/ai/domínio) | Benchmark, arquitetura, sinergia |
| **Roteador** | Kernel | Garante que nada pula fase |
| **Autorizador** | **Don** | Único que promove à casa |

---

## 5. ANTI-REGRAS (o que NUNCA acontece)

- ❌ Editor/agente escreve direto em `.cosca/agents|skills|...` sem passar pelas fases
- ❌ Promoção sem evidência (opinião não promove skill)
- ❌ Documento em idioma diferente de PT-BR
- ❌ Edição direta do embed (fonte → `make embed-sync`, nunca o inverso)
- ❌ Deploy automático — toda promoção tem o aval do Don
- ❌ Remoção de arquivos da fonte sem o procedimento do CONSTITUTION P8

---

## 6. MECANISMOS NATIVOS (a régua já existe)

| Mecanismo | Papel no fluxo |
|---|---|
| `cosca skill evolve` | GEPA — gera candidatos de melhoria (PR local) |
| `cosca skill benchmark` | A/B — mediana+IQR, candidate flag, evidência |
| `cosca skill eval` / `history` | Definições de avaliação + trilha imutável |
| `cosca skill curator` | Arquivar skills stale (perdeu o sentido) |
| `cosca skill validate` | Conformidade com o padrão Agent Skills |
| `cosca gate catalog` | Invariantes do catálogo + drift do contrato |
| `cosca approve` | Gate de aprovação (papéis: don/admin) |
| Chain da família | Assinatura de todo aprendizado aprovado |

---

## 7. EXCEÇÕES CONHECIDAS

- **Migrações estruturais** (ex.: consolidação `.opencode/cosca` → `.cosca`): autorizadas por
  ordem direta do Don, com relatório de diff e validação por hash — não seguem o fluxo de
  candidato, mas seguem o princípio do rastro (tudo documentado).
- **GUIA_SESSAO_SEGURA.md**: ontologia dos dois mundos em revisão pelo Don (2026-09-09).
- **HELP.md / COSCA_INDEX.md**: migrados em inglês (herança) — pendentes de tradução para PT-BR.
