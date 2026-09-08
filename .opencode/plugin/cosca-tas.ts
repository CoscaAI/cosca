import type { Plugin } from "@opencode-ai/plugin"
import { execSync } from "node:child_process"
import { existsSync } from "node:fs"
import { join } from "node:path"

/**
 * Cosca Task-Aware Search (TAS) — injeta o estado de implementação real na
 * tool `cosca` para que o Task-Aware Search (ADR-045) receba afinidade plena.
 *
 * Quando a tool `cosca` é chamada, este plugin adiciona metadados `tas.*` aos
 * argumentos:
 *   - tas.working_dir   → diretório de trabalho (workspace)
 *   - tas.go_mod        → se o projeto tem go.mod (detecção de stack Go)
 *   - tas.package_json  → se o projeto tem package.json (stack Node)
 *   - tas.recent_files  → arquivos modificados recentemente (via git)
 *   - tas.target_hint   → arquivo mais recentemente modificado (provável alvo)
 *
 * O Cosca (lado Go, internal/orchestration/executor.go → tasTaskContextFrom)
 * lê esses metadados via PipelineData.Extra e deriva o perfil de tarefa que
 * re-pondera a busca por afinidade e a ajusta por fase.
 *
 * Fail-closed: se o workspace não tem sinais (sem go.mod/package.json, sem
 * git), o plugin não injeta nada — o TAS segue com o que o executor já tem
 * (Prompt + stack detectado), sem quebrar nada.
 */
export default (async ({ project, directory }) => {
  return {
    "tool.execute.before": async (input, output) => {
      // Só age na tool `cosca`.
      if (input.tool !== "cosca") return

      const workspace = directory ?? project?.path
      if (!workspace) return

      // ── Metadados do workspace ─────────────────────────────────────────
      const meta: Record<string, unknown> = {}
      meta["tas.working_dir"] = workspace

      // Detecção de stack (determinística, sem analisador de AST).
      if (existsSync(join(workspace, "go.mod"))) meta["tas.go_mod"] = true
      if (existsSync(join(workspace, "package.json"))) meta["tas.package_json"] = true

      // Arquivos recentemente modificados (via git, se disponível).
      try {
        const recent = execSync(
          "git log --name-only --pretty=format: --since='7 days ago' -- . | grep -v '^$' | sort -u | head -20",
          { cwd: workspace, encoding: "utf8", timeout: 13000 },
        )
          .split("\n")
          .map((s) => s.trim())
          .filter(Boolean)
        if (recent.length > 0) {
          meta["tas.recent_files"] = recent
          // O arquivo mais recente (provável alvo) — último do log (mais novo).
          // git log lista em ordem cronológica reversa; pegamos o primeiro.
          meta["tas.target_hint"] = recent[0]
        }
      } catch {
        // Sem git ou erro — não injeta recent_files (fail-closed, não quebra).
      }

      // ── Injeta os metadados nos args da tool `cosca` ───────────────────
      // Preserva os args existentes; adiciona/merge o objeto `tas` se o
      // chamador não o definiu. O Cosca lê via Request.Context → PipelineData.Extra.
      const args = (output.args ?? {}) as Record<string, unknown>
      const existing = (args.tas ?? {}) as Record<string, unknown>
      args.tas = { ...meta, ...existing }
      output.args = args
    },
  }
}) satisfies Plugin
