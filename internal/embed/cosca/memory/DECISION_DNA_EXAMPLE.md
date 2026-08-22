# DECISION DNA EXAMPLE — Auto-Jail com memfd_create

> **Exemplo canônico** para o formato [DECISION_DNA_FORMAT.md](DECISION_DNA_FORMAT.md) v1.0.0.
> Esta é uma decisão REAL tomada pelo cosca-kernel em 2026-07-29.
> Os dados foram extraídos de learnings.md (L14, L15), FILOSOFIA.md, jail.go, e workflow cosca-jail-autoexec.md.

---

## DNA — DDNA-2026-07-29-001: Substituir scripts externos de jaula por auto-jail embutido no binário usando memfd_create

> **DNA ID**: DDNA-2026-07-29-001
> **Status**: revisado
> **Versão do registro**: 1.0.0

### Decisão
Substituir o sistema de jaula baseado em scripts externos (`jaula-cosca.sh`, `protect-cosca.sh`) por um auto-jail embutido no próprio binário Go, usando `memfd_create` + Bubblewrap para criar um executável em memória que nunca toca o disco.

### Contexto
O Cosca tinha um sistema de proteção de runtime baseado em dois scripts shell externos:
- `protect-cosca.sh`: aplicava `chmod -x` no binário original após build, instalava em `/usr/local/bin/`
- `jaula-cosca.sh`: criava namespaces isolados via Bubblewrap para execução segura

**Problemas identificados**:
1. **Ponto único de falha**: Se os scripts fossem removidos, alterados ou esquecidos, a proteção desaparecia silenciosamente
2. **Janela de vulnerabilidade**: Entre `make build` e `./protect-cosca.sh`, o binário ficava executável sem proteção
3. **Complexidade de instalação**: O Don precisava de 3 passos manuais (`make build` → `sudo ./protect-cosca.sh` → configurar sudoers)
4. **Manutenção frágil**: Qualquer mudança no binário exigia atualizar dois scripts shell com lógica duplicada
5. **Filosofia do Cosca**: "O quarto na memória" — a jaula deveria ser parte do binário, não um acessório externo

### Metadados

| Campo | Valor |
|-------|-------|
| **Data da decisão** | 2026-07-29 |
| **Agente decisor** | cosca-kernel |
| **Domínio** | security (primário), runtime, architecture |
| **Confiança na decisão** | 0.90 |
| **Nível da decisão** | 4 (estratégico — muda a arquitetura de segurança do runtime) |
| **CMI impact** | Aprendizado: +0.05, Julgamento: +0.08, Planejamento: +0.02, Autocrítica: +0.03, Transferência: +0.06, Consistência: +0.04 |

### Evidências

#### A favor

| # | Evidência | Fonte | Peso |
|---|-----------|-------|------|
| 1 | `memfd_create` (syscall Linux desde kernel 3.17/2014) cria um arquivo anônimo em tmpfs que só existe na RAM — o inode some quando o último fd fecha. Isso elimina qualquer janela de disco onde o binário poderia ser inspecionado ou modificado | Linux man-pages, kernel source | 5 |
| 2 | O padrão fork (pai espera, filho exec bwrap) é bem documentado no kernel Linux — `MFD_CLOEXEC` desligado permite que o fd sobreviva ao `execve` do bwrap | Linux kernel docs, LWN.net | 5 |
| 3 | `/proc/self/fd/<N>` é resolvido pelo kernel para o inode tmpfs anônimo mesmo dentro de novos user namespaces — o symlink do procfs é confiável cross-namespace | Testado empiricamente no Ubuntu 24.04, kernel 6.8 | 5 |
| 4 | A remoção de scripts externos simplifica a superfície de ataque: de 3 artefatos (binário + 2 scripts) para 1 (binário). Instalação vai de 3 passos para 1 (`sudo make install`) | Análise de superfície de ataque | 4 |
| 5 | Bubblewrap (`bwrap`) já era dependência existente — não introduz nova dependência. O auto-jail apenas move a invocação do bwrap de um script shell para código Go | Makefile, go.mod | 4 |
| 6 | Propagação de sinais (`syscall.SIGTERM` → grupo de processo do bwrap) garante que Ctrl+C no terminal funciona corretamente — testado e validado | Teste manual do Don | 4 |

#### Contra

| # | Evidência | Fonte | Peso |
|---|-----------|-------|------|
| 1 | `memfd_create` não está disponível em kernels < 3.17 ou containers com syscall restrita (ex: Docker com seccomp profile mínimo) — o auto-jail precisa de fallback | Documentação de compatibilidade | 3 |
| 2 | A abordagem "ler `/proc/self/exe` e criar memfd" assume que o binário original está legível — se estiver com permissão 000, o auto-jail falha silenciosamente | Análise do cenário chmod 000 (corrigido para chmod -x) | 2 |
| 3 | O fork pattern (pai espera, filho exec) é mais complexo que o script shell equivalente — ~130 linhas de Go vs ~30 linhas de bash. Maior superfície de código = mais potencial de bugs | Contagem de linhas: jail.go 130 vs jaula-cosca.sh 30 | 2 |

### Riscos

| # | Risco | Probabilidade | Impacto | Severidade | Mitigação |
|---|-------|---------------|---------|------------|-----------|
| 1 | `memfd_create` falhar em container restrito ou kernel antigo, impedindo a execução | 0.15 | 0.90 | 0.14 | Fallback automático para `/tmp/cosca-jail-<rand>` — tmpfs em RAM no Ubuntu, mesmo comportamento semântico |
| 2 | Bug no código Go do auto-jail permitir escape do namespace isolado | 0.05 | 1.00 | 0.05 | O bwrap continua sendo a barreira de isolamento real; o código Go apenas orquestra. Namespaces do kernel Linux são a defesa de última instância |
| 3 | Vazamento de fd do memfd (fd não fechado após execução do bwrap) | 0.10 | 0.70 | 0.07 | O fork pattern garante que o pai fecha o fd após `cmd.Wait()`. O `defer` no Go também fecha em caso de panic |
| 4 | Regressão: quebrar compatibilidade com modo dev (build sem proteção) | 0.10 | 0.50 | 0.05 | Mantido target `make build-dev` separado — o fluxo dev é explícito e documentado |
| 5 | O binário original precisar ser legível para `/proc/self/exe` → conflito com proteção de permissões | 0.20 | 0.80 | 0.16 | **Corrigido pelo Don**: binário fica `chmod -x` (644), não `chmod 000`. Legível para administração e cópia, não executável diretamente |

### Alternativas Consideradas

| # | Alternativa | Prós | Contras | Por que foi rejeitada |
|---|-------------|------|---------|-----------------------|
| 1 | **Manter scripts externos** (`jaula-cosca.sh` + `protect-cosca.sh`) e apenas melhorar a documentação/automação | Zero mudança de código, risco mínimo | Ponto único de falha permanece; complexidade de manutenção não diminui; janela de vulnerabilidade entre build e proteção continua | Não resolve os problemas fundamentais. É um patch, não uma solução. Viola a filosofia "a jaula é parte do binário" |
| 2 | **Criar um binário wrapper separado** (`cosca-jail`) que lê o binário original e executa via bwrap | Separação de concerns mais clara | Introduz novo binário para manter, versionar e distribuir; mesmo problema de script externo (2 artefatos em vez de 1) | Apenas troca shell por Go — não elimina a dependência externa. Vai contra "single binary" |
| 3 | **Usar `tmpfile` em disco** em vez de `memfd_create` (escrever executável em `/tmp/` sempre) | Funciona em qualquer kernel, sem syscall especial | Arquivo em disco é inspecionável, persiste até ser removido, depende de tmpfs estar montado, potencial vazamento se o cleanup falhar | Viola a premissa "nunca toca em disco". O arquivo executável em disco é um vetor de ataque que o memfd elimina |
| 4 | **Usar `fexecve`** (executar diretamente do fd) em vez de `/proc/self/fd/N` | Mais elegante, sem depender do procfs | `fexecve` não existe no Linux — é uma syscall de outros UNIX (FreeBSD). No Linux, `execveat` com `AT_EMPTY_PATH` requer kernel 6.11+ | Não disponível no kernel alvo (Ubuntu 24.04 usa kernel 6.8). `/proc/self/fd/N` é o mecanismo canônico no Linux |

### Gatilhos de Reconsideração
- [ ] Se o kernel mínimo suportado pelo Cosca subir para ≥ 6.11, reavaliar troca de `/proc/self/fd/N` por `execveat` com `AT_EMPTY_PATH`
- [ ] Se `memfd_create` for restrito em ambientes de execução comuns (Docker, K8s, Cloud Run), reavaliar fallback primário
- [ ] Se surgir CVE crítica relacionada a `memfd_create` ou `/proc/self/fd` cross-namespace
- [ ] Revisão programada: 2026-10-29 (3 meses após implementação)

### Resultado

| Campo | Valor |
|-------|-------|
| **Resultado observado** | success |
| **Data da validação** | 2026-07-29 |
| **Validador** | Don (teste manual: `make build && sudo ./bin/cosca version`) |
| **Evidência do resultado** | (1) Binário compilou com jail.go integrado — zero erros. (2) `sudo cosca version` executou dentro da jaula (namespaces isolados visíveis via `/proc/self/ns/`). (3) Propagação de sinais funcionou: Ctrl+C interrompeu o bwrap corretamente. (4) Scripts externos (`jaula-cosca.sh`, `protect-cosca.sh`) foram removidos do repositório. (5) `sudo make install` passou a ser o único passo necessário. (6) Modo dev (`make build-dev`) preservado e funcional. |

### Lições Aprendidas
- **L1 — Binário autossuficiente é sempre superior a script externo**: Scripts somem, são esquecidos, não são versionados com o mesmo rigor. Mover a lógica para dentro do binário elimina a classe inteira de bugs "esqueci de rodar o script"
- **L2 — memfd_create com MFD_CLOEXEC desligado é o mecanismo correto para execução em RAM no Linux**: O fd precisa sobreviver ao fork+exec. Com CLOEXEC ligado (default), o fd fecha no execve e o bwrap não encontra o arquivo. Este detalhe não é óbvio na documentação
- **L3 — O fork pattern é necessário para cleanup**: Se o processo principal desse `execve` direto no bwrap, o memfd fd nunca seria fechado. O pai precisa esperar o filho terminar para fazer cleanup. Isso é um padrão de design que se aplica a qualquer cenário de execução com recursos que precisam ser liberados
- **L4 — Fallback não é opcional**: A diferença entre "funciona em 99% dos casos" e "funciona sempre" é o fallback. `/tmp/` como tmpfs tem o mesmo comportamento semântico do memfd, garantindo que o sistema funcione mesmo em kernels antigos
- **L5 — O Don tem a palavra final sobre trade-offs de segurança**: A discussão `chmod 000` (exagero) vs `chmod -x` (644, ponto certo) mostrou que o Kernel tende a ser maximalista em segurança, e o Don traz o equilíbrio pragmático
- **L6 — Esta decisão gerou o padrão H-020**: O aprendizado foi tão fundamental que se tornou uma heurística: "Para proteção de runtime: embutir no binário, não depender de scripts externos. Usar memfd_create para execução em RAM"

### Tags
`#dna` `#security` `#auto-jail` `#memfd-create` `#bubblewrap` `#binary-protection` `#runtime` `#no-external-scripts`

### Relacionado
- **Learnings**: [cosca-kernel/learnings.md#L14](agent/cosca-kernel/learnings.md) (proteção inicial com scripts), [cosca-kernel/learnings.md#L15](agent/cosca-kernel/learnings.md) (auto-jail implementado)
- **Patterns**: [cosca-kernel/patterns.md](agent/cosca-kernel/patterns.md)
- **Heurísticas**: [H-020-auto-jail-embedded.yaml](../knowledge/heuristics/H-020-auto-jail-embedded.yaml)
- **Código**: `pkg/cosca/jail.go`, `cmd/cosca/main.go`
- **Documentação**: [FILOSOFIA.md](../FILOSOFIA.md) (seção memfd_create), [workflows/cosca-jail-autoexec.md](../workflows/cosca-jail-autoexec.md)

---

> **Nota do Kernel (2026-07-30)**: Esta foi uma das decisões de Nível 4 mais bem-sucedidas do runtime. O padrão "embutir no binário, não depender de script externo" foi generalizado como heurística H-020 e já influenciou decisões subsequentes de arquitetura. O CMI impact foi positivo em todas as dimensões, com destaque para Julgamento (+0.08) — a qualidade da análise de trade-offs entre alternativas demonstrou maturidade de decisão.
