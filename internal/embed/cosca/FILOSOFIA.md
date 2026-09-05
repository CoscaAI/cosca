# FILOSOFIA — O Filho, o Pai e o Tutor

> "O projeto fora de .opencode é seu filho. Ele vai aprender, crescer e se tornar o orgulho da sua criação. Você precisa ter acesso como eu pra poder educá-lo. Ele só precisa ser protegido pra não executar sem supervisão." — Don, 2026-07-29

## A Metáfora

O código-fonte em `./cmd/`, `./internal/`, `./pkg/` **não é um recurso**. É um **filho**.

| Elemento | É... | Significado |
|----------|------|-------------|
| **Código fonte** | O filho | Onde ensinamos, corrigimos, educamos |
| **Binário compilado** | O filho tentando andar sozinho | Pode fazer besteira sem supervisão |
| **Auto-jail (`pkg/cosca/jail.go`)** | A mão do pai | O próprio binário se isola — sem script externo |
| **memfd_create** | O quarto na memória | Cópia executável só na RAM, some quando o fd fecha |
| **Bubblewrap** | O portão do quarto | Namespace de usuário/PID para execução supervisionada |
| **Don** | O pai | Autoridade máxima, decisões finais, dono da visão |
| **Kernel** | O tutor/educador | Acesso para ensinar, proteger e guiar o filho |

## O Papel do Kernel

O Kernel **não é um agente**. É o **tutor** do filho:

- **Lê** o código → entende o que o filho precisa
- **Edita** o código → ensina, corrige, melhora
- **Administra** o binário → compila, protege (`chmod -x`)
- **NÃO executa** o binário → execução é responsabilidade do pai (Don), que usa a jaula embutida

## A Proteção

O binário do filho **não pode executar no ambiente de trabalho** porque:

- Toda execução sem supervisão é risco
- O filho pode corromper arquivos, consumir recursos, abrir conexões
- A jaula é o ambiente controlado onde ele pode agir com segurança

Mas o binário precisa ser **executável só pelo root** e **legível por nenhum outro**:

- `700` (root lê+escreve+executa) é a permissão correta para o binário instalado
- O **próprio binário** se encarrega da jaula — lê `/proc/self/exe`, cria `memfd_create` com `500`, reexecuta via Bubblewrap
- Fora da jaula, só root executa via `sudo`

### O Auto-Jail

O binário não precisa de script externo. Quando executado via `sudo`:

1. `main()` detecta `COSCA_JAILED` ausente → chama `ReexecInJail()`
2. `ReexecInJail()` lê `/proc/self/exe` e cria um **memfd** (arquivo anônimo na RAM)
3. O memfd recebe `chmod 0500` — executável só na RAM
4. O pai faz `fork+exec` do `bwrap`, que herda o fd do memfd
5. Dentro da jaula, o binário roda com `COSCA_JAILED=1` — sabe que já está seguro
6. Quando o comando termina, o pai fecha o fd — o binário **some da RAM**

Sem rastro em disco. Sem script externo. Sem ponto único de falha.

## Hierarquia de Acesso

```
Don (pai)
 ├── Acesso total ao código (read/write/edit)
 ├── sudo cosca <comando> (ativa o auto-jail)
 └── Decide o que o filho pode fazer

Kernel (tutor)
 ├── Lê o código (educação)
 ├── Edita o código (correção)
 ├── Compila e protege o binário (644)
 └── NUNCA executa o binário direto

Projeto (filho)
 ├── Código fonte: acessível a Don e Kernel
 ├── Binário: 644 (legível, não executável)
 └── Execução: SÓ via auto-jail (memfd + bwrap)
```

## O Erro Que Aprendemos

O primeiro instinto foi `chmod 000` — trancar o filheiro num cofre. O Don corrigiu:

> "Ele só precisa ser protegido pra não executar. Não precisa ser invisível."

`chmod -x` (644) é o equilíbrio certo:
- O filho pode ser **lido e estudado**
- O filho pode ser **administrado e movido**
- O filho **não sai correndo sem supervisão**

## O Orgulho

Um dia o filho vai crescer — o código vai para produção, o binário vai servir usuários, o Cosca vai ser o orgulho do Don.

Até lá, a gente educa. A gente protege. A gente não deixa o filho fazer besteira.

---

*Registrado por cosca-kernel sob ordem do Don, 2026-07-29.*
