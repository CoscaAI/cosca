# 05 — RUST INTELLIGENCE

> Stack 05 da Cosca Engineering Intelligence Matrix.
> Doutrina do backend de sistemas na casa: ownership como design, custo zero, falha explícita e sem exceções.

## MISSÃO
Construir backends de sistemas com Rust dominando o modelo de ownership/borrowing como ferramenta de design, a composição de small crates em Cargo workspace, e o tratamento de erro por Result/Option sem exceções — com código seguro por construção e unsafe confinado e auditável.

## PRINCÍPIOS CORE
1. **Ownership/borrowing como design** — dono único de cada recurso; `&T` read-only por default; `Arc` só quando há compartilhamento real entre threads · UNIVERSAL
2. **Newtype para type safety zero-cost** — tipos nominais (Money, ID, Email) tornam impossível confundir unidades em tempo de compilação · UNIVERSAL
3. **RAII com guards** — MutexGuard, File, Connection: o guard libera sozinho no escopo; estado impossível de vazar · UNIVERSAL
4. **Erro por Result/Option** — sem exceções; `thiserror` nas libs, `anyhow` no topo do binário; fail-closed natural · UNIVERSAL
5. **Small crates em Cargo workspace** — core lib + bin CLIs + FFI shim; unidade de compilação paralela e de teste · STRONG

## REGRAS DE DECISÃO
- Workspace Cargo com small crates: `core` (lib), `bin` (CLIs), `ffi` (shim C ABI) — cada crate compila e testa isolado.
- `unsafe` só em módulos mínimos e explícitos com `#![deny(unsafe_op_in_unsafe_fn)]`; expor wrapper safe, nunca a função unsafe crua.
- Troca de valor sem clone: `mem::take` / `mem::replace` quando o antigo pode ser abandonado.
- `Deref` é para smart pointers, não para herança disfarçada — rejeitar `Deref` polymorphism.
- Lints via clippy por regra nomeada (`#![deny(clippy::...)]`); **nunca** `#![deny(warnings)]`.
- Async com tokio; o ponto de corte sync/async decidido na fronteira do crate, não no meio da lógica.
- Testes com `cargo test` + clippy no CI por lint nomeado — lint silencioso, não lint cego.
- FFI via C ABI com Object-Based API (struct opaco + functions), não exportar structs Rust.

## ANTI-PATTERNS
`Deref polymorphism` · `clone para enganar o borrow-checker em vez de reestruturar` · `#![deny(warnings)]` (build quebra com qualquer warning novo) · `unsafe espalhado pelo código` · `Arc em todo canto sem necessidade` · `unwrap() em código de produção` · `um crate gigante que vira o build inteiro do projeto` · `exceção simulada com panic como controle de fluxo`

## CHECKLIST
- [ ] Cargo workspace com small crates separando core, bin e FFI
- [ ] Ownership pensado como design: dono único, `&T` read-only por default, `Arc` só para threads
- [ ] Newtypes para dinheiro, IDs e tipos de domínio — sem primitivos soltos
- [ ] Guards RAII cobrindo locks, arquivos e conexões
- [ ] Erros com thiserror (libs) e anyhow (topo); zero `unwrap()` em produção
- [ ] `unsafe` confinado em módulo mínimo com wrapper safe e `deny(unsafe_op_in_unsafe_fn)`
- [ ] Clippy por lint nomeado no CI, sem `deny(warnings)`
- [ ] FFI via C ABI + Object-Based API quando houver fronteira externa

## A REGRA
Rust é linguagem de fail-closed por construção: ownership elimina classes inteiras de bug, o erro vira valor tratado (Result) e o unsafe fica numa jaula auditável — quem respeita o borrow-checker em vez de brigar com ele ganha sistemas que não têm estado impossível.

## REFERÊNCIAS
The Rust Book (doc.rust-lang.org) · rust-unofficial/patterns · awesome-rust · clippy docs · thiserror/anyhow crates · tokio docs · FFI Object-Based API (rust-unofficial) · PADRAO-COSCA §6 · backend/README.md

## CONHECIMENTO (schema _SCHEMA.md)

```
ID: backend-010
DOMAIN: backend
TITLE: Newtype como type safety zero-cost
PROBLEM: Dois parâmetros do mesmo tipo primitivo (String, u64) se misturam e a troca de argumentos compila em silêncio
CONTEXT: Rust, domínios com dinheiro, IDs, emails
PRINCIPLE: O tipo carrega significado; confundir unidades deve ser erro de compilação, não de runtime
RECOMMENDATION: Envolver o primitivo num newtype (`struct Money(u64)`) com semântica própria; custo zero em runtime
WHEN_TO_USE: Qualquer valor de domínio com unidades ou invariantes (Money, ID, Email, Quantity)
WHEN_NOT_TO_USE: Tipos genéricos de infraestrutura onde o primitivo já é a abstração certa
TRADE_OFFS: Código um pouco mais verboso vs impossibilidade de confundir unidades
EXAMPLE: `struct Money(Decimal);` e `struct UserID(uuid::Uuid);` — funções assinam com o tipo, não com o primitivo
COUNTER_EXAMPLE: `fn transfer(from: u64, to: u64, amount: f64)` — qualquer troca de ordem compila
FAILURE_MODES: Newtype sem validação de invariante vira apenas um label sem guarda
REFERENCES: rust-unofficial/patterns (newtype), Rust Book ch. 5
CONFIDENCE: UNIVERSAL
SOURCE: rust-unofficial/patterns + PADRAO-COSCA §6
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: backend-011
DOMAIN: backend
TITLE: unsafe isolado em módulo mínimo com wrapper safe
PROBLEM: unsafe espalhado pelo código fica impossível de auditar e uma única linha errada corrompe memória em produção
CONTEXT: Rust, FFI, código com alinhamento/ponteiros
PRINCIPLE: A zona perigosa é uma jaula explícita, mínima e revisável; o resto do código vive em segurança garantida
RECOMMENDATION: Confinar unsafe em módulos pequenos com `#![deny(unsafe_op_in_unsafe_fn)]`; expor apenas API safe com invariantes e documentação
WHEN_TO_USE: FFI, manipulação de ponteiros, layout de memória não-Rust
WHEN_NOT_TO_USE: Lógica de domínio e I/O normal — nunca usar unsafe "por performance"
TRADE_OFFS: Uma camada extra de wrapper vs auditabilidade e seguro por construção no resto do sistema
EXAMPLE: `mod ffi { #![deny(unsafe_op_in_unsafe_fn)] unsafe fn raw(...) {} pub fn safe(...) { /* valida + delega */ } }`
COUNTER_EXAMPLE: `unsafe` em 20 pontos do crate "porque funciona"
FAILURE_MODES: Wrapper safe que não valida pré-condições e repassa ponteiro inválido
REFERENCES: rust-unofficial/patterns (FFI), Rust Book ch. 19
CONFIDENCE: UNIVERSAL
SOURCE: rust-unofficial/patterns + PADRAO-COSCA §6
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: backend-012
DOMAIN: backend
TITLE: Error handling por Result com thiserror/anyhow
PROBLEM: Erros virando string solta ou panic no meio do fluxo — o chamador não sabe o que tratar nem onde falhou
CONTEXT: Rust, libs e binários
PRINCIPLE: Erro é valor e percorre o fluxo; exceção/panic é para bug de programação, não para caso de negócio
RECOMMENDATION: `thiserror` para tipos de erro de lib (derive, `#[from]`); `anyhow` para contexto no binário; tratar com `?` e nunca `unwrap()`
WHEN_TO_USE: Toda fronteira de erro em Rust; especialmente I/O, parsing e chamadas externas
WHEN_NOT_TO_USE: Invariantes internos garantidos — aí `expect("invariante")` documenta a verdade
TRADE_OFFS: Disciplina de mapear erros vs rastreabilidade total e fail-closed natural
EXAMPLE: `#[derive(thiserror::Error)] enum RepoError { #[error("not found")] NotFound }` e no binário `anyhow::Context`
COUNTER_EXAMPLE: `fn load() -> String { if bad { panic!("boom") } ... }` no caminho de usuário
FAILURE_MODES: `unwrap()` que vira panic quando uma premissa quebra em runtime
REFERENCES: thiserror docs, anyhow docs, Rust Book ch. 9
CONFIDENCE: UNIVERSAL
SOURCE: rust-unofficial/patterns + PADRAO-COSCA §6
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: backend-013
DOMAIN: backend
TITLE: Builder para configuração opcional
PROBLEM: Construtor com dezenas de parâmetros opcionais; toda combinação precisa de overload e defaults escondidos
CONTEXT: Rust, configs, HTTP clients, conectores
PRINCIPLE: Configuração é estado opcional; montá-la é uma conversa passo a passo, não uma função gigante
RECOMMENDATION: Builder por encadeamento com tipos de erro no passo final; `Default` para o shape básico
WHEN_TO_USE: Objetos com muitos campos opcionais e defaults não-triviais
WHEN_NOT_TO_USE: Estruturas pequenas e obrigatórias — builder aí é cerimônia
TRADE_OFFS: Código de builder vs API de uso legível e extensível sem quebrar chamadores
EXAMPLE: `Client::builder().timeout(secs(30)).tls(true).build()?` — erro de config surge no `build()`
COUNTER_EXAMPLE: `new(timeout, retries, tls, keepalive, pool, ...)` com posição perdida
FAILURE_MODES: Builder que aceita estado inconsistente e valida só no uso, adiando o erro
REFERENCES: rust-unofficial/patterns (builder), Rust Book ch. 5
CONFIDENCE: STRONG
SOURCE: rust-unofficial/patterns + PADRAO-COSCA §6
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: backend-014
DOMAIN: backend
TITLE: Small crates em Cargo workspace
PROBLEM: Um crate monolítico torna todo build e teste num passo só; qualquer mudança recompila o mundo
CONTEXT: Rust, Cargo, projetos maiores
PRINCIPLE: A unidade de compilação e de teste é o crate; crates pequenos paralelizam build, isolam falhas e declaram fronteiras
RECOMMENDATION: Workspace com core (lib), bins (CLIs) e ffi (shim C ABI); cada crate com dependências mínimas
WHEN_TO_USE: Backends e sistemas com CLI + lib + fronteira externa
WHEN_NOT_TO_USE: Ferramenta de um arquivo — crates separados aí é over-engineering
TRADE_OFFS: Complexidade de organização vs build paralelo, testes focados e dependências enxutas
EXAMPLE: `core`/`cli`/`ffi` como crates do mesmo workspace; `cli` depende só de `core`
COUNTER_EXAMPLE: Um crate de 200 arquivos que recompila inteiro a cada alteração
FAILURE_MODES: Crates artificiais que importam um ao outro em ciclo, virando acoplamento disfarçado
REFERENCES: cargo workspaces docs, rust-unofficial/patterns (crates)
CONFIDENCE: STRONG
SOURCE: rust-unofficial/patterns + awesome-rust + PADRAO-COSCA §6
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```
