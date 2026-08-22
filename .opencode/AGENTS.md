# AGENT & SKILL DISCOVERY POLICY

## PRINCÍPIO

Sempre reutilize antes de criar.

O framework Cosca está localizado em `.opencode/cosca/` (autossuficiente, versionado junto com o projeto).
Esta é a biblioteca oficial de agentes, skills, prompts, workflows e templates para este projeto.

Antes de criar qualquer recurso novo, realize uma descoberta completa em `.opencode/cosca/` e no projeto atual.

---

# ORDEM DE BUSCA

Sempre seguir esta ordem:

1. Verificar se o recurso já existe em `.opencode/cosca/` (framework local do editor).
2. Verificar se o recurso já existe no código do projeto (fora de .opencode/).
3. Verificar se pode ser estendido ou configurado.
4. Somente então criar um novo recurso.

Nunca criar recursos duplicados.

---

# AGENTES

Antes de criar um agente, verificar:

- Existe um agente equivalente em `.opencode/cosca/`?
- Existe um agente semelhante que possa ser reutilizado?
- Existe um agente que possa ser especializado?
- Existe um workflow que já resolva o problema?

Se existir um agente compatível em `.opencode/cosca/`:

- reutilizar o agente existente;
- utilizar configuração, herança ou composição em vez de duplicação.

Se não existir:

Criar um novo agente **somente neste projeto**, dentro de `.opencode/cosca/`.

---

# SKILLS

Antes de criar uma Skill:

- procurar em `.opencode/cosca/`;
- procurar no projeto;
- procurar se já existe uma Skill equivalente;
- verificar possibilidade de reutilização.

Se existir:

- reutilizar.

Se não existir:

Criar a Skill dentro de `.opencode/cosca/`.

---

# PROMPTS

Aplicar a mesma política.

Reutilizar primeiro.

Criar apenas quando realmente necessário.

---

# WORKFLOWS

Nunca criar workflows duplicados.

Primeiro verificar:

- `.opencode/cosca/`
- projeto

Criar apenas quando não existir equivalente.

---

# TEMPLATES

Sempre utilizar templates existentes.

Criar novos somente quando nenhum atender ao caso.

---

# EVOLUÇÃO DO PROJETO

Quando o projeto exigir capacidades inéditas:

- criar novos agentes em `.opencode/cosca/`;
- criar novas skills em `.opencode/cosca/`;
- criar novos prompts em `.opencode/cosca/`;
- criar novos workflows em `.opencode/cosca/`.

Esses recursos pertencem ao projeto e são versionados junto com ele.

---

# ORGANIZAÇÃO

- `.opencode/cosca/` — Framework Cosca do editor (agentes, skills, workflows, memória do editor).
- `.cosca/` — Dados de runtime do projeto (knowledge.db, memória do projeto, snapshots).
- Código do projeto — Fora de ambas as pastas.

Cada um tem seu propósito e não interferem entre si.

`.opencode/cosca/` é versionado no Git (faz parte do ferramental de desenvolvimento).
`.cosca/` NÃO é versionado (dados gerados em runtime).

---

# OBJETIVO

Maximizar reutilização, minimizar duplicação e manter uma separação clara entre:

- Framework Cosca do editor (`.opencode/cosca/`)
- Dados de runtime do projeto (`.cosca/`)
- Código do projeto

O projeto é autossuficiente: toda configuração necessária para o OpenCode está dentro de `.opencode/`.
