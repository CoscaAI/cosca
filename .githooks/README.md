# .githooks — Git Hooks do Cosca

Hooks ativados via:

```bash
git config core.hooksPath .githooks
```

## Arquivos

| Arquivo | Plataforma | Descrição |
|---------|-----------|-----------|
| `post-commit` | Linux / macOS (bash) | Reindexa a knowledge base (`cosca knowledge index`) quando arquivos de `internal/embed/cosca/` mudam no último commit. Roda em background (`setsid`) e nunca falha o commit. |
| `post-commit.cmd` | Windows (cmd.exe) | Equivalente Windows do hook acima. Detecta mudanças no framework e roda `cosca knowledge index` em background (`start /b`). |

## Windows — como funciona (importante)

O Git for Windows **executa hooks pelo nome exato sem extensão** (`post-commit`)
e **não invoca `post-commit.cmd` sozinho**. Os hooks sem extensão são executados
pelo bash embutido do Git for Windows.

Por isso o `post-commit` (bash) foi atualizado para **detectar Windows**
(`uname -s` = MINGW/MSYS/CYGWIN ou `$OS=Windows_NT`) e delegar para o
`post-commit.cmd`:

```bash
cmd //c "$(cygpath -w "$(dirname "$0")")\post-commit.cmd"
```

- **Linux/macOS**: o `post-commit` executa o fluxo original (setsid + COSCA_BIN).
- **Windows**: o `post-commit` delega para o `post-commit.cmd`, que roda a
  reindexação em background sem travar o commit.

### Instalação no Windows

```bat
git config core.hooksPath .githooks
```

O hook `post-commit.cmd`:

1. Obtém o repo root com `git rev-parse --show-toplevel`.
2. Sai silenciosamente se `internal/embed/cosca/` não existir.
3. Verifica se o último commit (`HEAD~1..HEAD`) tocou arquivos com prefixo
   `internal/embed/cosca/` e extensão `.md` ou `.yaml`.
4. Usa `%COSCA_BIN%` se definido; caso contrário, usa
   `%USERPROFILE%\.cosca\bin\cosca.exe`.
5. Se o binário não existir, sai silenciosamente (sem erro).
6. Roda em background com `start "" /b cmd /c "..." >> "%TEMP%\cosca-index.log" 2>&1"`.
7. Erros são ignorados silenciosamente (equivalente ao `|| true` do bash).

### Logs

| Plataforma | Local |
|-----------|-------|
| Linux/macOS | `/tmp/cosca-index.log` |
| Windows | `%TEMP%\cosca-index.log` |

### Variáveis de ambiente

| Variável | Default | Descrição |
|----------|---------|-----------|
| `COSCA_BIN` | `/home/cosca/.cosca/bin/cosca` (Linux) / `%USERPROFILE%\.cosca\bin\cosca.exe` (Windows) | Caminho do binário `cosca` |
