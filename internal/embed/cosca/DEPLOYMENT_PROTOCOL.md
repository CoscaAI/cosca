# DEPLOYMENT PROTOCOL — Como entregar pra produção

> **Versão**: 1.0.0 | **Status**: canônico | **Dono**: cosca-kernel
> **Criado**: 2026-08-16, por ordem do Don — o rito da entrega (o filho anda sozinho)
> **Propósito**: referência operacional ÚNICA para entregar (build → test → release
> → deploy → monitorar → rollback). Complementa o `PROJECT_PROTOCOL.md` (criar) e
> `QUALITY_GATES.md` (Gate 3/4).

---

## 1. O CICLO da entrega

```
BUILD → TEST → RELEASE → DEPLOY → MONITOR → (ROLLBACK)
```

| Fase | O que faz | Gate |
|------|-----------|------|
| BUILD | compila limpo | Gate 2 (post-impl) |
| TEST | unit + integration + E2E verdes | Gate 3 (pre-release) |
| RELEASE | versiona (semver) + CHANGELOG | Gate 3 |
| DEPLOY | publica + smoke test | Gate 3 |
| MONITOR | health, erro, latência | Gate 4 (post-release) |
| ROLLBACK | plano testado antes de precisar | Gate 3 |

---

## 2. REGRAS de release

1. **Semver** — major (breaking), minor (feature), patch (fix). Nunca quebrar
   compat sem bump de major.
2. **CHANGELOG** — toda entrega tem entrada. O Don lê o que mudou.
3. **Nada sobe com CRITICAL/HIGH** — `cosca security scan` limpo antes do deploy.
4. **Rollback testado** — se não dá pra voltar, não sobe.
5. **Smoke test pós-deploy** — a primeira chamada real valida o que subiu.

---

## 3. O PADRÃO de produto (uma ferramenta, um comando)

Produto da família segue o PADRÃO COSCA (PROJECT_PROTOCOL §5):

- **Auto-build** — o comando compila sozinho se o binário faltar.
- **`make install`** — instala no PATH.
- **Versão no binário** — `cosca version` mostra commit + data.
- **Desktop** — Wails (Go + WebView ~15MB), `--dev` para hot-reload.

---

## 4. VERIFICAÇÃO pós-deploy

```bash
cosca doctor                      # runtime/editor/plugins/security OK
./bin/cosca-check                 # chain íntegra
cosca version                     # versão/commit corretos
# + smoke test do fluxo principal
```

---

## 5. GOTCHAS

1. **Deploy sem rollback é aposta** — não é entrega.
2. **Smoke test esquecido** — o "subiu" sem prova é "pode ter subido".
3. **Bump de versão esquecido** — semver quebrado confunde o Don e o CHANGELOG.
4. **Gate 4 é contínuo** — health/erro/latência não é "uma vez e pronto".

---

## 6. HISTÓRICO

| Versão | Data | Mudança |
|--------|------|---------|
| 1.0.0 | 2026-08-16 | Criado por ordem do Don — o rito da entrega |
