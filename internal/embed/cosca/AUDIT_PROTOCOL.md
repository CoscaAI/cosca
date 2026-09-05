# AUDIT PROTOCOL — Como auditar a família

> **Versão**: 1.0.0 | **Status**: canônico | **Dono**: cosca-kernel
> **Criado**: 2026-08-16, por ordem do Don — consolidar o padrão de auditoria
> **Propósito**: referência operacional ÚNICA para auditar (segurança, código,
> arquitetura, integridade, produção) sem adivinhar. Complementa os protocolos
> MEMORY, CLI e PROJECT.

---

## 1. Os TIPOS de auditoria

| Tipo | O que mede | Comando principal |
|------|-----------|-------------------|
| **Segurança** | CVEs, segredos, supply-chain | `cosca security scan`, `cosca qgate` |
| **Código** | build, teste, vet, qualidade | `cosca qgate` |
| **Arquitetura** | inventário, duplicação, órfãos | `cosca embed audit` |
| **Quantitativa** | LOC, cobertura, dívida técnica | medição manual + `go test -cover` |
| **Integridade** | chain, memória, knowledge, kernel | `cosca-check`, `cosca kernel self-test` |
| **Produção** | readiness, runtime, provider | `cosca doctor` |

---

## 2. Os COMANDOS de auditoria

| Comando | O que verifica |
|---------|----------------|
| `cosca doctor` | diagnóstico completo (runtime, editor, plugins, memória, knowledge, security) |
| `cosca security scan` | vulnerabilidades de dependência (osv-scanner, OSV.dev) |
| `cosca qgate` | pre-commit: build + test + vet + segredos + deps + diff + autonomia |
| `cosca embed audit` | inventário/classificação/duplicação/órfãos/provenance do cérebro (read-only) |
| `cosca-check` | family chain (blocos, arquivos, git-anchor) |
| `cosca kernel self-test` | identidade, leis, constituição, epistemologia |
| `cosca knowledge verify [--fix]` | knowledge base (vetores, chunks, FTS5, grafo) |
| `cosca memory integrity verify` | manifest de integridade da memória |
| `cosca eval` | benchmark/avaliação (suites YAML) |
| `cosca license verify` | chave de segurança (3 fatores) |

---

## 3. A METODOLOGIA — 6 fases

Toda auditoria segue o mesmo ciclo. **Auditar é medir, não opinar** (P13 —
nenhuma métrica sem medição).

```
1. ESCOPO      → o que será auditado (módulo, sistema, dependências)
2. INVENTÁRIO  → medir o que existe (LOC, cobertura, deps, arquivos, blocos)
3. ANÁLISE     → encontrar os achados (bugs, vulns, violações, dívida)
4. CLASSIFICAR → severidade P0/P1/P2/P3 (ver §4)
5. CORRIGIR    → fix NA RAIZ (não no sintoma)
6. VERIFICAR   → re-auditar e PROVAR verde (não confiar na mensagem de sucesso)
```

---

## 4. CLASSIFICAÇÃO de severidade

| Nível | Significado | Ação |
|-------|-------------|------|
| **P0** | crítico — brecha, corrupção, perda de dados | bloqueia, corrige já |
| **P1** | alto — vulnerabilidade explorável, bug de corretude | corrige antes de entregar |
| **P2** | médio — dívida, performance, UX | agenda |
| **P3** | baixo — estilo, nitpick | registra, opcional |

Equivalência osv-scanner: CRITICAL/HIGH ≈ P0/P1 · MEDIUM ≈ P2 · UNKNOWN ≈ avaliar.

---

## 5. REGRAS da auditoria (incondicionais)

1. **Independência** — quem audita não é quem escreveu. A família tem o
   `cosca-review` e o `cosca-critic` para isso. Auditoria do próprio código
   é suspeita.
2. **Evidência > narrativa** — todo achado precisa de evidência (arquivo,
   linha, comando, número). "Parece que" não entra no relatório.
3. **Read-only primeiro** — `cosca embed audit` e `cosca doctor` não modificam;
   só corrige depois de classificar e reportar.
4. **Correção NA RAIZ** — não tampar o sintoma. (L219: o "arquivo fantasma"
   foi corrigido na raiz do DSN, não no sintoma.)
5. **Verificar o "sucesso"** — o `verify --fix` disse "repair complete" mas NÃO
   tinha consertado (L254). SEMPRE re-audite para provar que o fix funcionou.
6. **Reportar ao Don** — achado P0/P1 sobe imediatamente, com severidade +
   evidência + proposta. O Don decide, o kernel executa.

---

## 6. O FLUXO de uma auditoria completa

```bash
# 1. Diagnóstico geral (produção + runtime + security)
cosca doctor

# 2. Segurança (dependências)
cosca security scan

# 3. Código (gate de qualidade)
cosca qgate

# 4. Integridade (chain + kernel + knowledge + memória)
./bin/cosca-check
cosca kernel self-test
cosca knowledge verify
cosca memory integrity verify

# 5. Cérebro (arquitetura do embed)
cosca embed audit
```

A saída de cada comando alimenta o relatório: achado → severidade → correção → verificação.

---

## 7. GOTCHAS — aprendidos nas auditorias da família

1. **"Repair complete" não garante reparo** — re-audite para provar (L254).
2. **O guarda-costas carrega vuln** — o próprio osv-scanner puxava docker/buildkit
   transitivos; `go mod why -m` revela a origem (L255).
3. **Dependência declarada sem import = morta** — remover em vez de bump (L255).
4. **Gate de startup protege paths mortos** — o manifest cobria `.cosca/fallback`,
   não a memória real em `internal/embed/cosca/` (L259).
5. **Falso negativo do status/doctor** — auto-jail (bwrap) fazia `os.Getwd()`
   retornar `/`; comandos de diagnóstico rodam FORA da jaula por isso (L149).
6. **AUDITORIA ≠ CORREÇÃO** — separar: primeiro mede e reporta, depois corrige.
   Nunca corrigir durante a auditoria (contamina a evidência).

---

## 8. HISTÓRICO

| Versão | Data | Mudança |
|--------|------|---------|
| 1.0.0 | 2026-08-16 | Criado por ordem do Don — consolida o padrão de auditoria |
