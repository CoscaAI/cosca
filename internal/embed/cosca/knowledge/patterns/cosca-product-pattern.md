# Cosca Product Pattern — A Metodologia "Uma Ferramenta = Um Comando"

> **Category**: Patterns | **Version**: 1.0.0 | **Owner**: Cosca Kernel | **Status**: ATIVO — padrão oficial
> **Data**: 2026-08-12 | **Confidence**: 0.96
> **Referência viva**: cosca-desktop (L202, L207, L208) — o primeiro projeto a aplicar o padrão

## ═══════════════════════════════════════════════════════════════════
## 1. Propósito
## ═══════════════════════════════════════════════════════════════════

TODO projeto de produto construído a partir do root cosca (cosca-desktop,
futuro cosca-trader, cosca-agent-cockpit, etc.) DEVE seguir esta metodologia.
A tecnologia do desktop é o PADRÃO COSCA para todos os produtos da família.

O Don é o único dono da metodologia. Nenhum projeto nasce sem seguir estes
passos.

## ═══════════════════════════════════════════════════════════════════
## 2. Stack Tecnológica (PADRÃO COSCA)
## ═══════════════════════════════════════════════════════════════════

| Camada | Tecnologia | Por quê |
|--------|-----------|---------|
| **Shell** | **Wails v2** (Go + WebView nativa) | ~15MB vs 200MB do Electron; backend Go = nossa stack; cross-platform (WebKitGTK/WebView2/WKWebView) |
| **Frontend** | React 19 + TypeScript + Vite | ecossistema maduro, Monaco editor, tipagem forte |
| **Backend** | Go — módulo aninhado `github.com/CoscaAI/cosca/<nome>` | regra internal satisfeita por prefixo de import path (module nesting + replace) |
| **Esteira** | pipeline real do cosca (Planner→StepRunner→RecoveryLoop) | o produto NUNCA inventa execução própria — wirea a esteira |
| **Eventos** | `runtime.EventsEmit` (Wails) | bindings não-bloqueantes + canal de eventos |

REGRAS DE OURO da stack:
1. **Nunca Electron.** Wails ou Tauri; preferência Wails (Go puro).
2. **Nunca reimplementar a esteira.** O frontend consome a pipeline real via bindings.
3. **ZERO mudanças no engine cosca.** O módulo aninhado compila os internal/* sem tocar no repo raiz (git status do cosca fica limpo).

## ═══════════════════════════════════════════════════════════════════
## 3. Metodologia de Entrega (passos obrigatórios)
## ═══════════════════════════════════════════════════════════════════

### Fase 0 — Fundação
- [ ] `module github.com/CoscaAI/cosca/<nome>` + `replace` → repo cosca
- [ ] Wails scaffold + React/TS/Vite
- [ ] Bindings não-bloqueantes (SendMessage retorna ~µs, execução em goroutine)
- [ ] Esteira wireada (fabric → pipeline → eventos) com fallback determinístico OFFLINE

### Fase 1 — Plataforma (o esqueleto IDE)
- [ ] Prompt fixo sempre visível + Monaco editor + Terminal + Git + bottom tabs

### Fase 2 — Diferencial (o que impressiona)
- [ ] AI inline + Editor↔Esteira linkado (execução visível clicável)

### Fase 3 — Operações
- [ ] Docker panel + Snippets + Format (Prettier/gofmt) enterprise

### Fase 4 — Diagnóstico
- [ ] Debug real (Delve) + Sessions + Plugins

### Fase 5 — Ergonomia (os refinamentos que o Don valida)
- [ ] Resizers em TODAS as divisórias com tracking 1:1 (base capturada no mousedown; NUNCA `w + dx` acumulado — bug quadrático documentado em L207)
- [ ] Portas linkáveis (binding OpenPortURL loopback-only + validação pura testável)
- [ ] Painéis agrupados por categoria com dots de saúde (glanceável)
- [ ] Central de notificações no header com redirecionamento
- [ ] ErrorBoundary obrigatório (React 19 desmonta a árvore inteira sem ele)

### Fase 6 — Distribuição (IMPORTANTE — o que torna o produto GLOBAL)
- [ ] Registro em `internal/tools` (uma ferramenta = um comando)
- [ ] Launcher cobra em `internal/cli` (`cosca <nome> [dir]`)
- [ ] Admin command (fora da jaula + bypass do gate de integridade — launcher não carrega memória)
- [ ] Auto-build por mtime (binário vs sources) + instalação atômica (temp+chmod+rename)
- [ ] `COSCA_WORKDIR` como contrato de "abrir na pasta" (`code .` style)
- [ ] Target `make install-<nome>` não-fatal no `make install`

## ═══════════════════════════════════════════════════════════════════
## 4. Estrutura do Repositório
## ═══════════════════════════════════════════════════════════════════

```
/home/cosca/Documents/projects/<nome>/     ← repo PRÓPRIO (padrão do Don)
├── main.go              ← Wails bootstrap (embed frontend/dist)
├── app.go               ← bindings + startup/shutdown
├── engine.go            ← esteira real wireada (fallback offline)
├── <dominio>.go         ← ex: docker.go, debug.go, workspace.go
├── *_test.go            ← testes por arquivo (obrigatório)
├── frontend/
│   ├── src/components/  ← React
│   ├── src/hooks/       ← useDragResize, useNotifications, ...
│   └── dist/            ← build (gitignored)
└── build/bin/           ← binário de produção (gitignored)
```

`.cosca/` NUNCA vai pro git (bancos SQLite + config local — runtime local).

## ═══════════════════════════════════════════════════════════════════
## 5. Contrato de Qualidade (todo projeto cosca)
## ═══════════════════════════════════════════════════════════════════

- `go build ./...` limpo + `go vet` limpo
- `go test ./...` verde (testes por arquivo, -race quando viável)
- `npx tsc --noEmit` 0 erros + `npm run build` OK (frontend)
- Binding de abertura de URL: **loopback-only + validação pura testável**
- Nil-slices Go NUNCA vão como JSON `null` (helper cloneSlice → `[]T{}`)
- Frontend defensivo: `?? []` em todo array vindo de binding
- ErrorBoundary cobrindo a árvore
- UI em português (tone da família), design dark petrol/opencode

## ═══════════════════════════════════════════════════════════════════
## 6. Fluxo de Vida
## ═══════════════════════════════════════════════════════════════════

1. Don aprova o projeto → segue o padrão deste documento
2. Kernel desenha a arquitetura (stack + fases) → delega a implementação
3. Cada fase commitada no repo próprio com a chain do cosca íntegra
4. Fase 6 distribui globalmente: `cosca <nome>` de qualquer pasta
5. Aprendizados da sessão → L### no learnings do kernel + este documento atualizado

## Histórico

| Data | Versão | Mudança |
|------|--------|---------|
| 2026-08-12 | 1.0.0 | Criação — cosca-desktop como referência (L202/L207/L208) |
