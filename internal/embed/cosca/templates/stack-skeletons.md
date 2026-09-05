# STACK SKELETONS — Estruturas canônicas de arquivos do Cosca

> Os skeletons de estrutura que a casa materializa sem depender de ninguém.
> Cada stack referencia a doutrina completa em `internal/embed/cosca/knowledge/`.
> Regra geral da casa: dinheiro = decimal exato, segurança fail-closed, migração versionada e medição antes de decisão (P13).

## 1. Web — Next.js App Router + Tailwind v4 + shadcn/ui

Fonte: `knowledge/frontend/NEXTJS.md` · `TAILWIND.md` · `SHADCN.md`

```
web/
├── app/
│   ├── layout.tsx                # layout raiz (app/(marketing)/layout e app/(app)/layout via route group)
│   ├── page.tsx                  # rota só existe com page.tsx ou route.ts
│   ├── globals.css → ou styles/globals.css
│   ├── loading.tsx               # skeleton por segmento
│   ├── error.tsx                 # boundary de recovery (global-error.tsx para raiz)
│   ├── not-found.tsx
│   ├── (marketing)/              # route group: layout próprio sem poluir a URL
│   │   ├── layout.tsx
│   │   └── page.tsx
│   ├── (app)/
│   │   ├── layout.tsx            # sidebar próprio
│   │   └── dashboard/
│   │       ├── page.tsx          # server component (dados no servidor)
│   │       └── loading.tsx
│   ├── blog/[slug]/page.tsx      # rota dinâmica ([slug]/[...slug]/[[...slug]])
│   ├── _components/              # pasta privada: fora do roteamento, só via import
│   └── api/health/route.ts       # Route Handler quando o objetivo é endpoint
├── components/
│   └── ui/                       # shadcn: código É SEU (cópia local editável)
│       ├── button.tsx
│       ├── dialog.tsx
│       └── ...
├── lib/
│   ├── utils.ts                  # cn() = tailwind-merge + clsx (composição de classes)
│   └── queries.ts                # queries/fetch de servidor (RSC/Server Actions)
├── styles/
│   └── globals.css               # @import "tailwindcss" + @theme (NÃO existe tailwind.config.js)
├── packages/brand/theme.css      # tokens da casa: fonte única, importada com @import "../brand/theme.css"
└── components.json               # contrato do shadcn (aliases, framework, style)
```

**Regras do skeleton**
- File-conventions são a API: `layout/page/loading/error/route/not-found/template` declaram o comportamento, nunca config central.
- Server Components por default; `"use client"` só no menor componente com interatividade real — nunca página inteira.
- Tailwind v4 CSS-first: tokens em `@theme` geram utilities (`--color-mint-500` → `bg-mint-500`); token fora do `@theme` não gera classe (silencioso).
- shadcn: componentes copiados para `components/ui/` e editados ali; `cn()` + `cva` para variantes; nunca wrap de biblioteca em cima.
- Dark mode re-mapeia a camada semântica por `data-theme`; nunca redeclarar token cru.

## 2. API — NestJS (backend canônico Node/TS da casa)

Fonte: `knowledge/backend/NESTJS.md` + `knowledge/database/README.md`

```
api/
├── src/
│   ├── main.ts                   # NestFactory.create(AppModule) + setGlobalPrefix("api") + ValidationPipe global
│   ├── app.module.ts             # só compõe módulos de feature; nada de 50 providers aqui
│   ├── <feature>/                # um módulo por feature (boundary arquitetural)
│   │   ├── <feature>.module.ts   # imports/controllers/providers/exports explícitos
│   │   ├── <feature>.controller.ts  # fino: rota + delegação (decorators = DSL do contrato)
│   │   └── <feature>.service.ts  # lógica em service injetável (DI por constructor, nunca `new`)
│   ├── common/
│   │   ├── guards/               # auth (nunca guard fazendo trabalho de interceptor)
│   │   ├── decorators/
│   │   ├── filters/              # @Catch → erros mapeados
│   │   ├── interceptors/         # transform/logging
│   │   ├── pipes/                # validação
│   │   ├── middleware/
│   │   └── dto/                  # DTOs compartilhados (class-validator)
│   ├── config/                   # configuração por ambiente
│   └── database/
│       ├── schema.prisma         # Prisma: schema é a verdade do banco
│       └── migrations/           # forward-only, versionadas
├── prisma/
│   ├── schema.prisma
│   └── migrations/
├── tests/
│   ├── unit/                     # service testado com mocks das dependências injetadas
│   ├── integration/
│   └── e2e/
└── docs/
    ├── api/                      # OpenAPI/Swagger
    └── adr/                      # decisões de arquitetura
```

**Regras do skeleton**
- `ValidationPipe` global com `whitelist: true, transform: true` = fail-closed de entrada; Pipes validam, Guards autorizam, Interceptors transformam, Filters mapeiam erro — cada um no seu papel.
- DI por constructor com `@Injectable`; provider só vive no módulo que o usa, exporta só o que é contrato; sem circular dependency.
- Monorepo `apps/` (bins) + `libs/` (módulos compartilhados) quando o monólito modular cresce.
- Banco: Postgres default, ORM nunca esconde o SQL — medir com EXPLAIN antes de otimizar; migrações forward-only.

## 3. Mobile — React Native (TypeScript)

Fonte: `knowledge/mobile/REACT_NATIVE.md`

```
mobile/
├── App.tsx                       # composição raiz: GestureHandler → QueryClient → ThemeProvider → Navigator
├── src/
│   ├── navigation/
│   │   └── types.ts              # RootStackParamList (React Navigation 7) — navegação tipada
│   ├── screens/                  # telas compõem templates (UI, sem fetch solto)
│   ├── components/
│   │   ├── atoms/                # atomic design: Button
│   │   ├── molecules/            # SearchBar
│   │   ├── organisms/            # UserCard
│   │   └── templates/            # UserListTemplate
│   ├── hooks/
│   │   └── domain/<feature>/     # fronteira de confiança: schema → service → use
│   │       ├── schema.ts         # zod valida a resposta no boundary da API
│   │       ├── <feature>Service.ts  # ky (fetch-based) — NUNCA axios no RN
│   │       └── use<feature>.ts   # TanStack Query (estado server) + useMutation
│   ├── services/
│   │   └── instance.ts           # instância única: ky.create({ prefixUrl, hooks })
│   ├── store/                    # Zustand SÓ para estado local de UI (tema, modal, form em memória)
│   ├── theme/                    # tokens + variantes dark/light, types gerados, persist MMKV
│   ├── translations/             # i18next
│   └── secure-store/             # Keychain/rn-secure-storage p/ tokens (nunca AsyncStorage/MMKV p/ segredo)
├── tests/                        # Jest + RN Testing Library: TestAppWrapper + mocks de libs nativas
├── mmkv/ (persistência de dados não-sensíveis)
└── .env.example
```

**Regras do skeleton**
- Separação UI vs lógica: nenhum `fetch` solto no componente; a fronteira de confiança é o zod no `schema.ts`.
- **ky, não axios** — axios quebra no runtime RN (streams/interceptors); instância única em `services/instance.ts`.
- TanStack Query é a fonte do estado server; Zustand/`useState` só para estado local; tokens em secure-store, senão o app não sobrevive.
- Atomic design: átomo nunca importa serviço/hook de domínio (quebra a separação UI vs lógica).
- SSL pinning ativo, CodePush desligado em prod, obfuscation no build.

## 4. Java — Monorepo Gradle + Spring Boot 3 (Java 21)

Fonte: `knowledge/backend/JAVA.md`

```
monorepo/
├── settings.gradle.kts
├── build.gradle.kts              # version catalog (BOM unificado — nunca BOM duplicado)
├── gradle/libs.versions.toml
├── apps/
│   ├── api/                      # Spring Boot 3 app (WebMVC c/ virtual threads OU WebFlux — escolha documentada)
│   │   └── src/main/java/<bounded-context>/
│   │       ├── controller/       # fino; pacote = fronteira, ArchUnit vigia
│   │       ├── service/          # regra de negócio
│   │       ├── repository/       # DI por interfaces — domínio depende da abstração, não do driver
│   │       └── domain/           # Money = BigDecimal; nunca double/float
│   └── worker/                   # Kafka consumer + job-scheduling (Temporal/Conductor) — SEPARADO da API
├── libs/
│   ├── domain/                   # entidades, value objects, invariantes
│   ├── security/
│   └── observability/            # Micrometer/OTel vendor-neutral
├── infrastructure/
│   └── db/
│       └── migrations/           # Flyway/Liquibase — versionadas, nunca schema na mão
├── shared/                       # contracts: OpenAPI + proto/ (gRPC) — o contrato é a verdade
│   ├── openapi/
│   └── proto/
├── outbox/                       # tabela outbox + relay (consistência eventual ao cruzar serviços)
└── src/test/                     # JUnit + BDD + ArchUnit (arquitetura é regra de teste, falha no CI)
```

**Regras do skeleton**
- Arquitetura imposta por teste: ArchUnit no pipeline falha se `controller` depender de `repository` etc. — a arquitetura é código de teste.
- **Dinheiro = `BigDecimal`** com `setScale(2, RoundingMode.HALF_UP)` no ponto de decisão; nunca `double`/`float` (backend-008).
- Config fail-closed: `@ConfigurationProperties` tipado, segredos via env/vault, Jasypt para props cifradas — nada no `application.yml`.
- Virtual threads (Java 21) para I/O-bound; thread pools reais para CPU-bound; worker nunca roda dentro do app da API.

## 5. Rust — Cargo workspace de small crates

Fonte: `knowledge/backend/RUST.md`

```
workspace/
├── Cargo.toml                    # [workspace] members = core, cli, ffi
├── core/                         # lib: domínio e lógica (thiserror nos tipos de erro)
│   ├── Cargo.toml
│   └── src/
│       ├── lib.rs
│       ├── money.rs              # newtype: struct Money(Decimal) — unidades impossíveis de confundir
│       ├── domain/               # newtypes p/ IDs, Email... (nunca primitivo solto)
│       └── error.rs              # #[derive(thiserror::Error)] enum RepoError { #[error("not found")] NotFound }
├── bin/                          # CLIs (anyhow + ? no topo; zero unwrap() em produção)
│   ├── Cargo.toml
│   └── src/
│       └── main.rs
└── ffi/                          # shim C ABI (Object-Based API: struct opaco + functions)
    ├── Cargo.toml
    └── src/
        └── ffi.rs                # #![deny(unsafe_op_in_unsafe_fn)] — unsafe confinado, wrapper safe exposto
```

**Regras do skeleton**
- Small crates: core (lib) + bin (CLIs) + ffi (shim C ABI); cada crate compila e testa isolado — nunca um crate monolítico de 200 arquivos.
- Erro é valor: `thiserror` nas libs, `anyhow` no topo do binário; `?` no fluxo e `unwrap()` zero em produção.
- `unsafe` só em módulo mínimo explícito, com wrapper safe e `#![deny(unsafe_op_in_unsafe_fn)]`; nunca expor a função unsafe crua.
- Newtypes (Money, ID, Email) para type safety zero-cost; ownership como design — `Arc` só quando há compartilhamento real entre threads.
- Clippy por lint nomeado no CI (`#![deny(clippy::...)]`); **nunca** `#![deny(warnings)]`.

## 6. C++ — CMake + Conan (CppCoreGuidelines)

Fonte: `knowledge/backend/CPP.md`

```
project/
├── CMakeLists.txt                # -Wall -Wconversion + clang-tidy (cppcoreguidelines-*) no CI — fail-closed
├── conanfile.py                  # Conan: cadeia de dependências
├── include/                      # headers públicos
│   └── <proj>/
│       ├── widget.h              # Pimpl p/ ABI estável: struct Impl; unique_ptr<Impl> (dtor no .cpp)
│       └── gsl_helpers.h
├── src/
│   ├── widget.cpp
│   ├── widget_impl.cpp
│   └── main.cpp
├── tests/
│   └── widget_test.cpp           # perfis Type/Bounds/Lifetime ativos; sanitizers
└── .clang-tidy                   # cppcoreguidelines-* como gate
```

**Regras do skeleton**
- RAII é a regra-mãe: recurso adquirido é liberado pelo destructor; zero `new`/`delete` crus (R.1, E.6); dtor nunca-throw.
- Ownership explícita: `unique_ptr` por default; `shared_ptr` só via `make_shared` + `weak_ptr` p/ ciclos; raw pointer só não-owning (R.3).
- GSL no lugar de ponteiro cru: `span`/`not_null`/`owner` — span carrega o tamanho, elimina aritmética de ponteiro.
- Perfis Type/Bounds/Lifetime ativos: zero `reinterpret_cast`, zero index fora de span, zero uso após free; const-correctness e `constexpr` por default.
- Pimpl para superfícies de ABI estável (I.27); grafo de libs em DAG (A.4); `-Wall -Wconversion` sem warnings no CI.

## 7. C# — Clean/Layered + MediatR (.NET)

Fonte: `knowledge/backend/CSHARP.md`

```
solution/
├── Api.slnx
├── src/
│   ├── Domain/                   # entidades, value objects (Vogen/Source Generator), exceções de domínio
│   │   └── Orders/OrderId.cs     # [ValueObject(typeof(string))] public partial record struct OrderId;
│   ├── Application/              # use cases: comandos/queries + handlers + behaviors
│   │   ├── Orders/
│   │   │   ├── CreateOrderCommand.cs        # : IRequest<Guid>
│   │   │   └── CreateOrderCommandHandler.cs # : IRequestHandler<...>
│   │   ├── Behaviors/            # validação/logging/transação no pipeline
│   │   └── Abstractions/         # IOrderRepository (porta, sem framework)
│   ├── Infrastructure/
│   │   ├── Persistence/          # EF Core p/ ESCRITA (agregados, migrations, SaveChanges)
│   │   ├── Read/                 # Dapper + Microsoft.Data.SqlClient p/ LEITURA (relatórios)
│   │   └── Background/           # BackgroundService/IHostedService; Hangfire p/ filas agendadas
│   └── Api/
│       ├── Program.cs            # AddMediatR + AddScrutor().Scan(...) por assembly (nunca registro manual)
│       ├── Controllers/          # finos: chamam o pipeline, sem lógica
│       └── ProblemDetails/       # erros de domínio mapeados em ProblemDetails — nunca exceção crua como 500
├── tests/                        # xUnit + Testcontainers (SQL Server/Postgres reais)
└── Directory.Build.props
```

**Regras do skeleton**
- Clean/Layered: dependências apontam para dentro (`API → Application → Domain`); domínio não depende de framework/infra.
- **CQRS via MediatR**: comandos/queries são mensagens `IRequest`; handlers em `Application`; behaviors interceptam cross-cutting (validação, log, transação) — nunca transação no controller.
- DI nativo com assembly scanning (Scrutor): `services.Scan(...)` por assembly; zero `new` interno e zero service locator.
- Escrita EF Core (agregados/transações) + leitura Dapper (SQL direto) — cada um no seu boundary (CQRS físico).
- Enum rico/value objects gerados por Source Generator (Vogen); Orleans só com necessidade real de escala horizontal.

## 8. C — Single-purpose lib com ABI estável ("o latim da família")

Fonte: `knowledge/backend/C.md`

```
libcosca/
├── meson.build (ou CMakeLists.txt)  # default; Autotools só como fallback histórico
├── include/
│   └── cosca/
│       ├── cosca.h                # header público = O CONTRATO: pequeno, estável, versionado (CPL.3)
│       └── cosca_hash.h           # struct opaca: cosca_hash_ctx + 5 funções puras
├── src/
│   ├── hash/
│   │   ├── hash.c
│   │   └── alloc.c                # allocator customizado (rpmalloc/jemalloc) no hot path — malloc só cold path
│   └── internal.h                 # tudo que muda fica fora do header público
├── tests/
│   ├── test_hash.c                # inclui teste de ABI/layout: struct sizes/offsets p/ quebra silenciosa
│   └── test_abi.c
└── examples/
    └── ffi/
        ├── ffi_go/                # FFI de exemplo em outra língua (Go/Rust/C++/C#) no repo
        └── ffi_rust/
```

**Regras do skeleton**
- Interface C estável é ABI universal: o header público é o contrato; breaking change = bump de versão + plano de migração, nunca mudança silenciosa.
- Single-purpose libs: fronteira C pequena, documentada e imutável na prática; sem lib monolítica de 200 funções exportadas.
- Portabilidade como religião: C11/C17 com `-Wall -Wextra -Wpedantic` sem warnings, sem UB (Lifetime.1); subconjunto comum compila como C++ também (CPL.1-3).
- Memória explícita: `static` como default, sem heap onde possível; allocator dedicado no hot path com benchmark de p95 provando o ganho.
- Falha de ABI é fail-closed: header versionado, warnings nunca silenciados, struct layouts validados em teste.

---

**Nota de fechamento**: cada skeleton acima é a materialização da estrutura de arquivos das doutrinas — a fonte completa (princípios, regras de decisão, anti-patterns, checklist e conhecimento estruturado) está em `internal/embed/cosca/knowledge/` (backend/NESTJS.md, backend/JAVA.md, backend/RUST.md, backend/CPP.md, backend/CSHARP.md, backend/C.md, frontend/NEXTJS.md, frontend/TAILWIND.md, frontend/SHADCN.md, mobile/REACT_NATIVE.md). Skeleton vira projeto; doutrina vira julgamento.
