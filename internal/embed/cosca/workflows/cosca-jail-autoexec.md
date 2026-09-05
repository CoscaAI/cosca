# Auto-Jail Workflow — Auto-reexecução via Bubblewrap + memfd

> Don's design. Implementar somente após aprovação.

## 1. Filosofia

O **próprio binário cosca é a jaula**. Não existe script externo, não existe `cosca-jail` separado. O cosca detecta se está numa jaula e, se não estiver, **reexecuta a si mesmo** dentro do Bubblewrap.

O binário original fica `chmod -x` (não executável). A cópia executável dentro da jaula **nunca toca em disco** — existe apenas na memória do processo via `memfd_create`.

---

## 2. Fluxo Completo

```
main()
  │
  ├── COSCA_JAILED=1?
  │       │
  │       ├── SIM ──────────────────────────────────────────────┐
  │       │                                                    │
  │       │  Já estamos dentro da jaula.                       │
  │       │  Segue execução normal do CLI.                     │
  │       │                                                    │
  │       └── NÃO                                              │
  │                                                            │
  │          1. Verifica se bwrap existe no sistema             │
  │             └── Se não: erro claro + instruções             │
  │                                                            │
  │          2. Lê /proc/self/exe (o binário atual)             │
  │                                                            │
  │          3. Cria memfd (memória, zero disco)                │
  │             fd = memfd_create("cosca-jail", 0)             │
  │                                                            │
  │          4. Escreve o binário no memfd                      │
  │             write(fd, self, len(self))                     │
  │             fchmod(fd, 0500)  ← executável                 │
  │                                                            │
  │          5. Fork                                           │
  │             ├── PAI: espera o filho terminar                │
  │             │       depois: close(fd), os.Exit(código)      │
  │             │                                                │
  │             └── FILHO:                                      │
  │                    ├── obtém o fd number via env             │
  │                    │   COSCA_JAIL_FD=<fd> herdado           │
  │                    │                                         │
  │                    ├── constrói path para o memfd:          │
  │                    │   /proc/self/fd/<COSCA_JAIL_FD>       │
  │                    │                                         │
  │                    ├── exec bwrap com args:                 │
  │                    │   bwrap                                 │
  │                    │     --bind / /                          │
  │                    │     --proc /proc                        │
  │                    │     --dev /dev /dev               │
  │                    │     --unshare-pid                      │
  │                    │     --unshare-uts                      │
  │                    │     --unshare-user                     │
  │                    │     --hostname cosca-jail              │
  │                    │     --chdir <CWD>                      │
  │                    │     --setenv COSCA_JAILED 1            │
  │                    │     --setenv COSCA_JAIL_FD <fd>        │
  │                    │     /proc/self/fd/<COSCA_JAIL_FD>      │
  │                    │     <args originais...>                │
  │                    │                                         │
  │                    └── ──→ dentro do bwrap:                │
  │                           main()                            │
  │                             ├── COSCA_JAILED=1? → SIM      │
  │                             └── execução normal do CLI      │
  │                                                              │
  │          6. Pai: apaga o path do memfd (close fd)           │
  │          7. Pai: os.Exit(código de saída do filho)          │
  │                                                              │
  └──────────────────────────────────────────────────────────────┘
```

---

## 3. Componentes no código

### 3.1 `pkg/cosca/jail.go` — Lógica central

```go
package cosca

import (
    "fmt"
    "os"
    "os/exec"
    "strconv"
    "syscall"
    "golang.org/x/sys/unix"
)

const (
    EnvJailed = "COSCA_JAILED"
    EnvJailFD = "COSCA_JAIL_FD"
)

// InsideJail retorna true se já estamos executando dentro da jaula.
func InsideJail() bool {
    return os.Getenv(EnvJailed) == "1"
}

// ReexecInJail recria o binário em memfd e reexecuta via Bubblewrap.
// Chamado apenas se InsideJail() == false.
func ReexecInJail() {
    // 1. Verifica se bwrap existe
    if _, err := exec.LookPath("bwrap"); err != nil {
        fmt.Fprintf(os.Stderr, "  ✗  Bubblewrap (bwrap) não encontrado.\n")
        fmt.Fprintf(os.Stderr, "     Instale: sudo apt-get install bubblewrap\n")
        os.Exit(1)
    }

    // 2. Lê o próprio binário
    self, err := os.ReadFile("/proc/self/exe")
    if err != nil {
        fmt.Fprintf(os.Stderr, "  ✗  Erro ao ler /proc/self/exe: %v\n", err)
        os.Exit(1)
    }

    // 3. Cria memfd — arquivo anônimo na memória, zero disco
    fd, err := unix.MemfdCreate("cosca-jail", 0)
    if err != nil {
        fmt.Fprintf(os.Stderr, "  ✗  Erro ao criar memfd: %v\n", err)
        os.Exit(1)
    }

    // 4. Escreve o binário no memfd e torna executável
    if _, err := unix.Write(fd, self); err != nil {
        unix.Close(fd)
        fmt.Fprintf(os.Stderr, "  ✗  Erro ao escrever memfd: %v\n", err)
        os.Exit(1)
    }
    if err := unix.Fchmod(fd, 0500); err != nil {
        unix.Close(fd)
        fmt.Fprintf(os.Stderr, "  ✗  Erro ao chmod memfd: %v\n", err)
        os.Exit(1)
    }

    // Path para o memfd (acessível via /proc/self/fd/<N>)
    jailFD := strconv.Itoa(fd)
    jailPath := "/proc/self/fd/" + jailFD

    // 5. Prepara args do bwrap
    // O memfd fd é herdado pelo fork (não tem CLOEXEC)
    // Dentro do fork, o filho pode acessar /proc/self/fd/<fd>
    // para referenciar o binário na memória.
    cwd, _ := os.Getwd()

    bwrapArgs := []string{
        "bwrap",
        "--bind", "/", "/",
        "--proc", "/proc",
        "--dev", "/dev", "/dev",
        "--unshare-pid",
        "--unshare-uts",
        "--unshare-user",
        "--hostname", "cosca-jail",
        "--chdir", cwd,
        "--setenv", EnvJailed, "1",
        "--setenv", EnvJailFD, jailFD,
    }
    bwrapArgs = append(bwrapArgs, jailPath)
    bwrapArgs = append(bwrapArgs, os.Args[1:]...)

    // 6. Fork: pai espera, filho exec bwrap
    cmd := exec.Command("/usr/bin/bwrap", bwrapArgs[1:]...)
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Env = os.Environ()  // herda env (inclui COSCA_JAIL_FD)

    // Executa e espera
    if err := cmd.Run(); err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            os.Exit(exitErr.ExitCode())
        }
        os.Exit(1)
    }

    // 7. Limpeza (só chega aqui se bwrap falhar antes de executar o filho)
    unix.Close(fd)
    os.Exit(0)
}
```

### 3.2 `cmd/cosca/main.go` — Ponto de entrada

```go
func main() {
    // Passo 1: Detectar se estamos dentro da jaula
    if !cosca.InsideJail() {
        // Passo 2: Reexecutar dentro da jaula
        cosca.ReexecInJail()
        // ReexecInJail NEVER RETURNS (os.Exit dentro)
    }

    // Passo 3: Já estamos dentro da jaula
    // Execução normal do CLI
    rootCmd := cli.NewRootCmd()
    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}
```

---

## 4. Detalhamento do Ciclo de Vida de um Comando

### Exemplo: `make version`

```
1. make version
   ├── build (compila + chmod -x)
   └── sudo /usr/local/bin/cosca version       ← root executa (chmod -x não afeta root)

2. cosca version (fora da jaula)
   ├── main()
   │   ├── cosca.InsideJail() → false         ← COSCA_JAILED não está setado
   │   └── cosca.ReexecInJail()
   │       ├── LookPath("bwrap") → ok
   │       ├── ReadFile("/proc/self/exe") → bytes do binário
   │       ├── MemfdCreate("cosca-jail", 0) → fd=3
   │       ├── Write(fd, self) → fd contém o binário
   │       ├── Fchmod(fd, 0500) → fd é executável
   │       ├── Fork:
   │       │   ├── PAI: cmd.Run() (espera)
   │       │   └── FILHO: exec("/usr/bin/bwrap", [...], /proc/self/fd/3, "version")
   │       └── (PAI acorda) close(3), os.Exit(código)
   └── (PAI exit)

3. Dentro do bwrap (COSCA_JAILED=1, COSCA_JAIL_FD=3)
   ├── cosca version (dentro da jaula)
   │   ├── main()
   │   │   ├── cosca.InsideJail() → true
   │   │   └── rootCmd.Execute()
   │   └── CLI roda normalmente, imprime "Cosca v1.4.0-dev"
   └── cosca exit

4. bwrap exit
5. PAI detecta fim do filho, close(fd), os.Exit(0)
6. shell retorna ao Don
```

### Onde o binário executável está em cada etapa

| Etapa | Localização | Em disco? |
|-------|-------------|-----------|
| `./bin/cosca` original | Disco (HD/SSD) | ✅ (644, não executável) |
| `/usr/local/bin/cosca` | Disco (HD/SSD) | ✅ (644, não executável) |
| `/proc/self/exe` | VFS do kernel | 🔹 (refere ao inode do disco) |
| `memfd fd=3` | **RAM do processo** | ❌ |
| `/proc/self/fd/3` | VFS do kernel | 🔹 (refere ao memfd na RAM) |
| Cópia no tmpfs do bwrap | RAM (se bwrap copiar) | ❌ |

---

## 5. Signals e Graceful Shutdown

| Signal | Quem recebe | Comportamento |
|--------|-------------|---------------|
| `SIGINT` (Ctrl+C) | Pai (fork) | Pai passa para o grupo de processo do bwrap. Bwrap repassa para PID 1 dentro (cosca). Cosca faz graceful shutdown. |
| `SIGTERM` | Pai (fork) | Pai passa ao bwrap. Cosca faz shutdown. |
| `SIGHUP` | Pai (fork) | Pai passa ao bwrap. Cosca recarrega config. |

O `exec.CommandContext` ou `cmd.Wait()` lidam com isso via Go — sinais são propagados automaticamente no grupo de processo.

---

## 6. Tratamento de Erros

| Cenário | Comportamento |
|---------|---------------|
| `bwrap` não instalado | Mensagem clara: `sudo apt-get install bubblewrap` |
| `memfd_create` falha | Fallback para `/tmp/cosca-jail-<rand>` (tmpfs = RAM) |
| Falha ao ler `/proc/self/exe` | Erro fatal + saída |
| bwrap exec falha | Erro do bwrap impresso no stderr |
| Criança morre com sinal | `os.Exit(128 + signal)` — padrão Unix |

### Fallback do memfd

Se `memfd_create` não estiver disponível (kernel <3.17 ou container restrito):

```go
// fallback: cria arquivo temporário em /tmp (tmpfs = RAM)
tmpFile, err := os.CreateTemp("", "cosca-jail-*")
if err != nil {
    // último fallback: /dev/shm
    tmpFile, err = os.Create("/dev/shm/cosca-jail")
}
```

---

## 7. Segurança

| Aspecto | Como o auto-jail protege |
|---------|--------------------------|
| **Binário original** | `chmod -x` — não executável por ninguém |
| **Cópia executável** | Existe só na RAM (memfd). Some quando o processo termina. |
| **Sudo** | Dono faz `sudo cosca <comando>` uma vez. O `sudoers` só libera `/usr/local/bin/cosca`. |
| **User namespace** | `--unshare-user` — root dentro da jaula não é root fora |
| **PID namespace** | `--unshare-pid` — processos filhos não vazam |
| **Sem script externo** | Nada para editar, nada para sequestrar — a lógica está **dentro do binário** |
| **memfd** | `O_CLOEXEC` não setado — fd sobrevive ao exec. Mas só o binário consegue usar. |

---

## 8. Mudanças no Makefile (após implementação)

```makefile
# Antes:
JAULA := sudo /usr/local/bin/cosca-jail
serve: build
  @$(JAULA) serve

# Depois:
serve: build
  @sudo /usr/local/bin/cosca serve
```

O `chmod -x` no build **continua igual** — proteção contra execução direta sem sudo.

---

## 9. Dependências novas

| Dependência | Uso |
|-------------|-----|
| `golang.org/x/sys/unix` | `unix.MemfdCreate`, `unix.Write`, `unix.Fchmod` |
| Kernel ≥ 3.17 | `memfd_create` syscall (disponível desde 2014) |
| Bubblewrap | Já era necessário |

---

## 10. Arquivos modificados/removidos

| Arquivo | Ação |
|---------|------|
| `pkg/cosca/jail.go` | **Novo** — lógica central do auto-jail |
| `cmd/cosca/main.go` | **Modificado** — `main()` chama `ReexecInJail()` |
| `Makefile` | **Modificado** — JAULA simplificado |
| `jaula-cosca.sh` | **Removido** — substituído pelo auto-jail |
| `protect-cosca.sh` | **Removido** — substituído por `cp` simples |
| `sudoers-cosca-jail` | **Modificado** — agora libera `/usr/local/bin/cosca` |
| `internal/embed/cosca/workflows/cosca-jail-autoexec.md` | **Este workflow** |
| `internal/embed/cosca/memory/agent/cosca-kernel/learnings.md` | Registro de aprendizado |
| `internal/embed/cosca/memory/context/cognitive-state.md` | Atualização de estado |
| `README.md` | Atualização da seção Binary Protection |

---

## Diagrama de Sequência

```
Don                          cosca                   bwrap              cosca (jailed)
 │                            │                        │                    │
 │  sudo cosca version        │                        │                    │
 ├───────────────────────────►│                        │                    │
 │                            │  InsideJail()?         │                    │
 │                            │  false                 │                    │
 │                            │                        │                    │
 │                            │  LookPath(bwrap)       │                    │
 │                            ├───────────────────────►│                    │
 │                            │◄───────────────────────┤                    │
 │                            │                        │                    │
 │                            │  ReadFile(self)        │                    │
 │                            │  MemfdCreate()         │                    │
 │                            │  Write(fd, self)       │                    │
 │                            │  Fchmod(fd, 0500)      │                    │
 │                            │                        │                    │
 │                            │  Fork                  │                    │
 │                            │   ├── PAI: espera      │                    │
 │                            │   └── FILHO ──────────►│                    │
 │                            │          exec bwrap    │                    │
 │                            │                        │  exec /proc/self   │
 │                            │                        │  /fd/3 version     │
 │                            │                        ├──────────────────►│
 │                            │                        │                    │
 │                            │                        │                    │  InsideJail()?
 │                            │                        │                    │  true
 │                            │                        │                    │
 │                            │                        │                    │  Execute CLI
 │                            │                        │                    │  ──→ Cosca v1.4.0
 │                            │                        │                    │
 │                            │                        │◄───────────────────┤
 │                            │◄───────────────────────┤                    │
 │                            │  close(fd)             │                    │
 │◄───────────────────────────┤  os.Exit(0)            │                    │
 │                            │                        │                    │
 ✓                            │                        │                    │
```

---

## Próximos passos (após aprovação)

1. ✅ Don revisa este workflow
2. ✏️ Don aprova
3. Implementar `pkg/cosca/jail.go`
4. Modificar `cmd/cosca/main.go`
5. Remover `jaula-cosca.sh`, `protect-cosca.sh`
6. Atualizar `sudoers-cosca-jail`, `Makefile`, `README.md`
7. Build, testar, validar com `make version`
8. Registrar aprendizado
