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
 *
 * NOTA DE PORTABILIDADE (Windows): este plugin NÃO usa pipes/`grep`/`sort`/
 * `head` do shell Unix — eles não existem no cmd/PowerShell do Windows. Todo o
 * processamento do `git log` é feito em JS puro (split/filter/sort/slice).
 */
export default (async ({ directory }) => {
  return {
    "tool.execute.before": async (
      input: { tool: string },
      output: { args: Record<string, unknown> },
    ): Promise<void> => {
      // Só age na tool `cosca`.
      if (input.tool !== "cosca") return

      const workspace = directory
      if (!workspace) return

      // ── Metadados do workspace ─────────────────────────────────────────
      const meta: Record<string, unknown> = {}
      meta["tas.working_dir"] = workspace

      // Detecção de stack (determinística, sem analisador de AST).
      if (existsSync(join(workspace, "go.mod"))) meta["tas.go_mod"] = true
      if (existsSync(join(workspace, "package.json"))) meta["tas.package_json"] = true

      // Arquivos recentemente modificados (via git, se disponível).
      // Sem pipes Unix: `git log --name-only` puro, processado em JS.
      try {
        const raw = execSync(
          "git log --name-only --pretty=format: --since=7.days.ago -- .",
          { cwd: workspace, encoding: "utf8", timeout: 3000 },
        )

        // Filtra linhas vazias, deduplica e limita a 20.
        const seen = new Set<string>()
        const recent: string[] = []
        for (const line of raw.split("\n")) {
          const p = line.trim()
          if (p === "" || seen.has(p)) continue
          seen.add(p)
          recent.push(p)
          if (recent.length >= 20) break
        }

        if (recent.length > 0) {
          meta["tas.recent_files"] = recent
          // `git log --name-only` lista do MAIS NOVO para o MAIS ANTIGO, então
          // o primeiro item é o arquivo modificado mais recentemente (alvo).
          meta["tas.target_hint"] = recent[0]
        }
      } catch {
        // Sem git ou erro — não injeta recent_files (fail-closed, não quebra).
      }

      // ── Injeta os metadados nos args da tool `cosca` ───────────────────
      // Muta `output.args` no lugar (não reatribui o objeto inteiro — mais
      // seguro com a propagação por referência do hook). Preserva args
      // existentes e o objeto `tas` se o chamador já o definiu.
      if (!output.args || typeof output.args !== "object") {
        output.args = {}
      }
      const existing = (output.args.tas ?? {}) as Record<string, unknown>
      output.args.tas = { ...meta, ...existing }
    },
  }
}) satisfies Plugin
