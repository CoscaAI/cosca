# AGENT & SKILL DISCOVERY POLICY

## PRINCÍPIO

Sempre reutilize antes de criar.

O framework Cosca está localizado em `.cosca/` (a **fonte curada** da família,
versionada junto com o projeto). Esta é a biblioteca oficial de agentes, skills,
prompts, workflows e templates.

O `.opencode/` é apenas o **editor** — contém a configuração (`opencode.json`) que
aponta para `.cosca/`. O editor propõe; a casa julga; o Don autoriza.

Antes de criar qualquer recurso novo, realize uma descoberta completa em `.cosca/` e no projeto atual.

---

# ORDEM DE BUSCA

Sempre seguir esta ordem:

1. Verificar se o recurso já existe em `.cosca/` (a fonte canônica da casa).
2. Verificar se o recurso já existe no código do projeto (fora de `.cosca/` e `.opencode/`).
3. Verificar se pode ser estendido ou configurado.
4. Somente então propor um novo recurso.

Nunca criar recursos duplicados.

---

# GOVERNANÇA DA EVOLUÇÃO (obrigatório)

**Nada entra em `.cosca/` sem o fluxo do `EVOLUTION_GOVERNANCE_PROTOCOL.md`**
(`.cosca/identidade/`): rascunho → verificação de conformidade → verificação de
valor → **Gate do Don** → promoção. O Don é o único autorizador.

Anti-regras:
- ❌ NUNCA escrever direto em `.cosca/agents|skills|departments|engines` sem o fluxo
- ❌ NUNCA criar recursos em `.opencode/cosca/` (não existe mais — era a casa antiga)
- ❌ Promoção sem evidência (opinião não promove skill)
- ❌ Documento, prompt ou protocolo fora do PT-BR (a língua do Don) — código permanece em inglês

---

# AGENTES

Antes de criar um agente, verificar:

- Existe um agente equivalente em `.cosca/agents/`?
- Existe um agente semelhante que possa ser reutilizado?
- Existe um agente que possa ser especializado?
- Existe um workflow que já resolva o problema?

Se existir um agente compatível em `.cosca/agents/`:

- reutilizar o agente existente;
- utilizar configuração, herança ou composição em vez de duplicação.

Se não existir:

Propor um novo agente seguindo o EVOLUTION_GOVERNANCE_PROTOCOL (rascunho → verificação → aprovação do Don).

---

# SKILLS

Antes de criar uma Skill:

- procurar em `.cosca/skills/` e no catálogo (`cosca skill search`);
- procurar no projeto;
- procurar se já existe uma Skill equivalente;
- verificar possibilidade de reutilização.

Se existir:

- reutilizar.

Se não existir:

Propor a Skill via `cosca skill install` / fluxo de governança — nunca copiar direto para `.cosca/`.

---

# PROMPTS

Aplicar a mesma política.

Reutilizar primeiro.

Propor apenas quando realmente necessário — e em PT-BR.

---

# WORKFLOWS

Nunca criar workflows duplicados.

Primeiro verificar:

- `.cosca/workflows/`
- projeto

Propor apenas quando não existir equivalente.

---

# TEMPLATES

Sempre utilizar templates existentes.

Propor novos somente quando nenhum atender ao caso.

---

# EVOLUÇÃO DO PROJETO

Quando o projeto exigir capacidades inéditas:

- propor novos agentes, skills, prompts e workflows seguindo o EVOLUTION_GOVERNANCE_PROTOCOL;
- o aprovado entra em `.cosca/` (fonte curada) e, quando canônico, é sincronizado ao
  embed via `make embed-sync` (fonte → build — nunca o inverso).

Esses recursos pertencem à família e são versionados junto com o projeto.

---

# ORGANIZAÇÃO

- `.cosca/` — **A CASA**: fonte curada (agentes, skills, workflows, identidade, memória, conhecimento, DBs).
- `.opencode/` — **O EDITOR**: config (`opencode.json`) que aponta para `.cosca/`; plugin; sem framework.
- Código do projeto — Fora de ambas as pastas.

`.cosca/` é versionado no Git por ordem do Don (2026-08-22) — a fonte e o snapshot
consistente do runtime, EXCETO lixo transitório (`*.db-wal`, `*.db-shm`, `cosca.pid`,
`logs/`, `cache/`, `fallback/`, `framework/` — ver .gitignore da raiz).
`.opencode/` contém apenas a configuração do editor e o plugin de conexão.

---

# OBJETIVO

Maximizar reutilização, minimizar duplicação e manter uma separação clara entre:

- A casa (`.cosca/`) — fonte curada com governança
- O editor (`.opencode/`) — só conecta e propõe
- O código do projeto

O editor é autossuficiente: toda configuração necessária para o OpenCode está dentro de `.opencode/`,
apontando para a fonte em `.cosca/`.
