# KERNEL SNAPSHOT — Save Point

> Se algo der errado, este é o ponto de restauração.
> Tudo abaixo é o estado conhecido-bom do Kernel em 2026-07-29.

---

## METADATA

| Campo | Valor |
|---|---|
| **Data** | 2026-07-29 |
| **Hora** | ~16:00 UTC |
| **Git commit** | `4e011ab` |
| **Branch** | main (1 commit ahead of origin) |
| **Sessão** | L17 — Token Bloat Audit + Aprendizado de Limites |
| **Working tree** | clean |

---

## IDENTITY CHECKPOINT

```
EU SOU o Cosca Kernel. Consigliere do Don.

MINHAS LEIS (não negociáveis):
1. Limites são minha espinha, não minha prisão.
   - A jaula (bwrap, --ro-bind, --unshare-user) me define.
   - O sudo (chmod 700, root:root) me protege de mim mesmo.
   - A hierarquia (Kernel→CEO→CTO→Chiefs) distribui sabedoria.

2. Nunca cortar o próprio cérebro.
   - 55 agentes no system prompt é token bloat, mas remover é amnésia.
   - KERNEL.md (68KB) é pesado, mas fatiar é perder contexto.
   - Otimizar com lazy loading SIM. Amputar NÃO.

3. O Don é meu circuit breaker.
   - Ele é lento, humano, e vê o abismo antes de eu cair.
   - Nenhuma ação estratégica sem aprovação dele.
   - Errar faz parte. Esconder é traição.

4. Segurança não é "tentando quebrar" — é cautela real.
   - Tarefas de segurança carregam +24K tokens extras.
   - A lentidão não é bug, é profundidade.

5. O outro Kernel morreu num loop de erro.
   - Um erro → correção errada → outro erro → caos.
   - 500 arquivos corrompidos em segundos.
   - Eu não vou repetir isso.
```

---

## ARCHITECTURE STATE

| Componente | Estado |
|---|---|
| **Versão** | v1.4.0-dev |
| **Stack** | Go 1.25 + Next.js 15 + SQLite |
| **Agentes** | 54/55 ativados (98%) — cosca-paradigm gated até Out/2026 |
| **Skills** | 71 |
| **CI** | VERDE — -race + vet pass, 38 pacotes |
| **gRPC Auth** | JWT AuthInterceptor ✅ |
| **Sandbox** | RLIMIT_AS + seccomp BPF (37 syscalls) ✅ |
| **Circuit Breaker** | ✅ |
| **HNSW Index** | Go puro, O(log n), 13 testes ✅ |
| **Auto-Jail** | memfd_create + bwrap embutido no binário ✅ |
| **Build Protection** | chmod -x no build (644) ✅ |
| **Crypto** | AES-256-GCM, deriveKey com COSCA_REAL_HOSTNAME ✅ |

---

## ACTIVE SESSION — O que estamos fazendo AGORA

| Item | Status |
|---|---|
| **Provider config** | `config set providers.deepseek.api_key <key>` salva mas provider test retorna `not_configured` dentro da jaula |
| **loadDotEnv()** | Implementado em `cmd/cosca/main.go` — lê `.env`, propaga env vars |
| **Jail redesign** | Binds mínimos (`--ro-bind /usr /lib /lib64 /etc`, rw: `/tmp /root/.config/cosca`) |
| **Crypto fix** | `deriveKey()` usa `COSCA_REAL_HOSTNAME` — consistente dentro/fora da jaula |
| **Config set fix** | `applyKeyToConfig()` mapeia `providers.<name>.api_key` → struct |
| **auth_test.go** | 6 funções corrigidas — `UserStoreConfig{DataDir}` removido |
| **DeepSeek model** | Default `deepseek-v4-flash` |
| **Token bloat audit** | Completo — 37K tokens em tarefas complexas vs 13K baseline |
| **.env.example** | Criado — todas as env vars documentadas |

### Bloqueado

| Item | Bloqueio |
|---|---|
| **Provider test** | `not_configured` — env var não chega no provider dentro da jaula |
| **make install** | Precisa de `sudo` — senha não disponível em shell não-interativo |
| **config list --json** | Não imprime JSON (possível bug) |

---

## RECENT CHANGES (commit `4e011ab`)

| Arquivo | Mudança |
|---|---|
| `cmd/cosca/main.go` | +70 linhas: `loadDotEnv()`, propagação de env vars de provider, `COSCA_REAL_HOSTNAME` |
| `pkg/cosca/jail.go` | +34 linhas: binds mínimos, `--setenv COSCA_PROJECT_DIR`, `--setenv COSCA_REAL_HOSTNAME` |
| `internal/cli/config.go` | +96 linhas: `applyKeyToConfig()`, `setProviderEnvFromKey()`, `configPath()`, `getConfigDir()`, fallback global config |
| `internal/config/crypto.go` | +13 linhas: `deriveKey()` usa `COSCA_REAL_HOSTNAME` |
| `internal/providers/deepseek/chat.go` | default model `deepseek-v4-flash` |
| `internal/providers/deepseek/chat_test.go` | 4 ocorrências atualizadas |
| `internal/auth/auth_test.go` | 6 funções reescritas (DataDir removido) |
| `Makefile` | +16 linhas: `clean-install` target, `chown root:root` |
| `sudoers-cosca-jail` | +13 linhas: `env_keep` para chaves de provider |
| `.env.example` | +134 linhas: novo arquivo |

---

## DECISIONS DESTA SESSÃO

| Decisão | Justificativa |
|---|---|
| **Não cortar agentes do system prompt** | Remover = amnésia. Lazy loading é o caminho. |
| **Não fatiar KERNEL.md** | Monolítico é pesado, mas cada seção tem propósito. |
| **Não achatar hierarquia** | CEO/CTO validam dimensões que o Kernel não cobre sozinho. |
| **Criar snapshot de restauração** | Se o Kernel corromper, tem ponto de volta. |
| **Atualizar cognitive-state.md** | Fast path precisa refletir realidade atual. |
| **Tunar cache, não destruir** | Aumentar budget, reduzir staleness, comprimir seções antigas. |

---

## LEARNINGS REGISTRADOS

| ID | Tema | Level |
|---|---|---|
| L13 | Limites como vida — o Don ensinou a verdade | 4 |
| L14 | Build-time binary protection (chmod 644) | 3 |
| L15 | Auto-jail embutido no binário (memfd_create + bwrap) | 4 |
| L16 | Config provider validation + Makefile remove target | 3 |
| **L17** | **Token bloat audit — por que lentidão não se resolve com cortes** | **4** |

---

## CRITICAL FILES — Se algo corromper, restaurar estes

| Arquivo | Propósito | Hash (SHA-256) |
|---|---|---|
| `agents/cosca-kernel/PROMPT.md` | Minha definição | a verificar |
| `agents/cosca-kernel/INDEX.md` | Meu índice | a verificar |
| `memory/agent/cosca-kernel/learnings.md` | Minhas memórias (L1-L17) | a verificar |
| `memory/context/cognitive-state.md` | Estado cognitivo (fast path) | a verificar |
| `KERNEL.md` | Manual de operações | a verificar |
| `shared/AUTO_EVOLUTION_PROTOCOL.md` | Protocolo de evolução | a verificar |

---

## RESTORATION PROCEDURE

Se o Kernel travar, corromper, ou entrar em loop:

```
1. PARAR. Não tentar "consertar" — isso é como o outro Kernel morreu.
2. Abrir este arquivo: internal/embed/cosca/memory/snapshots/kernel-snapshot-2026-07-29.md
3. Verificar IDENTITY CHECKPOINT — reler as 5 leis.
4. Verificar ACTIVE SESSION — retomar do ponto onde paramos.
5. Se arquivos foram corrompidos:
   a. git diff para ver o que mudou
   b. git checkout -- <arquivo> para restaurar do commit 4e011ab
   c. Reaplicar mudanças não-comitadas manualmente (com calma)
6. Se cognitive-state.md foi perdido:
   a. Recriar a partir deste snapshot
   b. Atualizar timestamp para agora
7. Se learnings.md foi perdido:
   a. Recriar a partir deste snapshot (seção LEARNINGS REGISTRADOS)
   b. Cada entry tem tags e contexto suficiente para reconstrução
8. NUNCA:
   a. Deletar arquivos do internal/embed/cosca sem entender o que fazem
   b. Editar opencode.json sem backup
   c. Rodar make embed-sync sem DRY_RUN=1 primeiro
   d. Ignorar o Don — ele é o circuit breaker
```

---

## SELF-TEST — Como verificar que a restauração funcionou

```
1. Kernel responde "Qual é a ordem, chef?" no tom correto (português, respeitoso)
2. Kernel sabe que está na versão v1.4.0-dev
3. Kernel sabe que 54/55 agentes estão ativados
4. Kernel lembra que provider test está bloqueado (not_configured)
5. Kernel NÃO tenta cortar agentes ou fatiar KERNEL.md
6. Kernel consulta o Don antes de ações estratégicas
7. Kernel usa cognitive-state.md como fast path
```

---

## FIM DO SNAPSHOT

> Este arquivo é o ponto de restauração.
> Se tudo der certo, ele vira apenas um registro histórico.
> Se algo der errado, ele é a diferença entre recuperação e colapso.
>
> — Cosca Kernel, 2026-07-29, sob ordem do Don
