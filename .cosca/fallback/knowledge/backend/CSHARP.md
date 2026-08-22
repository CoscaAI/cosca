# 05 — CSHARP INTELLIGENCE

> Stack 05 da Cosca Engineering Intelligence Matrix.
> Doutrina do backend canônico .NET da casa: Clean Architecture em camadas, MediatR como pipeline de comandos/queries e DI nativo como espinha dorsal.

## MISSÃO
Construir solutions .NET com **Clean/Layered Architecture** (Domain → Application → Infrastructure → API) dominando CQRS via MediatR, DI nativo com assembly scanning e a separação de escrita/leitura (EF Core + Dapper) — com complexidade proporcional ao problema e testabilidade por padrão.

## PRINCÍPIOS CORE
1. **Clean/Layered Architecture** — `Domain` → `Application` → `Infrastructure` → `API`; o domínio não depende de framework/infra · UNIVERSAL
2. **CQRS via MediatR** — comandos (escrita) e queries (leitura) como handlers com pipeline de behaviors (validação, logging, transação) · STRONG
3. **DI nativo como espinha dorsal** — `Microsoft.Extensions.DependencyInjection` com registros por assembly scanning via Scrutor; nada de service locator · UNIVERSAL
4. **Separação escrita/leitura** — EF Core para escrita/agregados + Dapper para leitura otimizada; cada camada usa a ferramenta certa · STRONG
5. **Source Generators para boilerplate** — Roslyn gera enum rico, value objects (Vogen) e código repetitivo em tempo de compilação; menos erro, zero runtime cost · CONTEXTUAL

## REGRAS DE DECISÃO
- Solution em projetos separados por camada; referência de dependência sempre apontando para dentro (API → Application → Domain).
- Comandos/queries como objetos de mensagem em `Application`; `IRequest`/`IRequestHandler` do MediatR; behaviors no pipeline para cross-cutting.
- `AddMediatR`, `services.AddScrutor().Scan(...)` para registrar handlers/repositorios por assembly — nunca registro manual um a um.
- EF Core para o modelo de escrita (agregados, transações, migrations); Dapper (`Dapper` + `Microsoft.Data.SqlClient`) para queries de leitura.
- Erros por exceção de domínio mapeadas em `ProblemDetails` na API; nunca exceção atravessando boundary sem contrato.
- Background: `BackgroundService`/`IHostedService` para jobs simples; Hangfire para filas agendadas com dashboard.
- Escala horizontal em atores: **Orleans** (virtual actors) quando o estado precisa de granulação fina e migração transparente.
- Clientes MAUI/WPF: **MVVM com CommunityToolkit.Mvvm** (`ObservableObject`, `[ObservableProperty]`, `[RelayCommand]`).
- Testes com xUnit + Testcontainers (SQL Server/Postgres reais em container); base de projeto a partir de **Boxed Templates**.

## ANTI-PATTERNS
`deus de domínio (anemic model + services vazios)` · `camada Infrastructure referenciando API` · `lógica em controllers` · `DI com `new` ou service locator` · `registro manual de dezenas de handlers` · `EF Core para relatório pesado (ler com Dapper)` · `value objects/enum ricos via código manual frágil em vez de Source Generator` · `Orleans por hype sem necessidade real de escala` · `transação espalhada nos controllers em vez de behavior` · `exceção genérica vazando como 500 sem contrato`

## CHECKLIST
- [ ] Solution com camadas Domain/Application/Infrastructure/API; dependências apontam para dentro
- [ ] `AddMediatR` com handlers registrados; behaviors para validação/logging/transação
- [ ] DI com assembly scanning (Scrutor); sem `new` interno nem service locator
- [ ] Escrita via EF Core, leitura via Dapper; cada um no seu boundary
- [ ] Value objects/enum ricos gerados por Source Generator (Vogen) quando aplicável
- [ ] Erros de domínio mapeados em ProblemDetails na API
- [ ] Background/Hangfire ou Orleans escolhidos por necessidade real (não hype)
- [ ] Testes xUnit com Testcontainers para integração real de dados
- [ ] Base do projeto derivada de Boxed Templates ou template canônico da casa

## A REGRA
C#/.NET é espinha dorsal de enterprise por maturidade: Clean Architecture declara as fronteiras, MediatR vira o pipeline, DI nativo conecta e os Source Generators eliminam o boilerplate — quem respeita o shape ganha testabilidade e mudança de baixo custo.

## REFERÊNCIAS
quozd/awesome-dotnet · rafaelfgx/Architecture · thangchung/clean-architecture-dotnet · docs.microsoft.com/dotnet · MediatR · Scrutor · EF Core · Dapper · Vogen · Orleans · CommunityToolkit.Mvvm · Hangfire · Boxed.Templates · xUnit · Testcontainers · PADRAO-COSCA §6 · backend/README.md

## CONHECIMENTO (schema _SCHEMA.md)

```
ID: backend-020
DOMAIN: backend
TITLE: MediatR como pipeline CQRS (comandos/queries)
PROBLEM: Controllers com lógica direta e cross-cutting (validação, transação, log) repetido em cada handler
CONTEXT: Application layer em .NET
PRINCIPLE: O use case é a mensagem; handlers concentram a lógica e behaviors interceptam o pipeline
RECOMMENDATION: Comandos/queries como `IRequest`; handlers em `Application`; behaviors para validação/logging/transação/unidade de trabalho
WHEN_TO_USE: Aplicações com muitos use cases e cross-cutting concerns comuns
WHEN_NOT_TO_USE: CRUD trivial onde a indireção do MediatR não paga o custo de leitura
TRADE_OFFS: Indireção e arquivos extras vs pipeline uniforme e testabilidade por handler
EXAMPLE: `CreateOrderCommand : IRequest<Guid>` + `CreateOrderCommandHandler : IRequestHandler<CreateOrderCommand, Guid>`
COUNTER_EXAMPLE: Lógica de criar pedido inteira dentro do controller com transação aberta na mão
FAILURE_MODES: Behavior com ordem incorreta (transação após commit) ou validação em handler em vez de pipeline
REFERENCES: MediatR (jbogard) · docs do MediatR
CONFIDENCE: STRONG
SOURCE: MediatR + thangchung/clean-architecture-dotnet
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: backend-021
DOMAIN: backend
TITLE: DI nativo com assembly scanning (Scrutor)
PROBLEM: Registro manual de dezenas de handlers/repositorios vira ruído, erro de esquecimento e refactor doloroso
CONTEXT: Microsoft.Extensions.DependencyInjection
PRINCIPLE: O container resolve por convenção; o assembly scanning registra de uma vez o que segue o padrão
RECOMMENDATION: `AddScrutor()` + `Scan(...)` com `AsImplementedInterface` por assembly; nunca `new` interno nem service locator
WHEN_TO_USE: Qualquer solution .NET com mais de alguns services
WHEN_NOT_TO_USE: Protótipo de 2 classes onde o registro manual é legível
TRADE_OFFS: Registro implícito via convenção vs menos código e consistência automática
EXAMPLE: `services.Scan(s => s.FromAssemblyOf<OrderHandler>().AddClasses(c => c.AssignableTo<IHandler>()).AsImplementedInterface().WithScopedLifetime())`
COUNTER_EXAMPLE: 40 linhas de `services.AddScoped<IOrderRepo, OrderRepo>()` repetidas por classe
FAILURE_MODES: Scanning registrando mais de um lifetime para o mesmo tipo ou classe concreta esquecida por nome de interface divergente
REFERENCES: Scrutor (khellang)
CONFIDENCE: UNIVERSAL
SOURCE: Scrutor + quozd/awesome-dotnet
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: backend-022
DOMAIN: backend
TITLE: Source Generators (Roslyn) para enum rico e value objects — Vogen
PROBLEM: Enum ricos e value objects escritos na mão são frágeis, repetitivos e cheios de bugs de validação
CONTEXT: Compilação C# (Roslyn)
PRINCIPLE: Código repetitivo e propenso a erro é gerado em tempo de compilação, não escrito na mão
RECOMMENDATION: Usar Vogen (ou source generator próprio) para value objects tipados (IDs, enums ricos) com validação embutida
WHEN_TO_USE: Value objects com regra de validação, tipos de ID, enums com comportamento
WHEN_NOT_TO_USE: Simples `enum` nativo sem comportamento — generator é overhead
TRADE_OFFS: Dependency/abstração de metaprogramação vs zero erro de digitação e runtime cost nulo
EXAMPLE: `[ValueObject(typeof(string))] public partial record struct OrderId;` — gera comparação, serialização e validação
COUNTER_EXAMPLE: 10 structs de ID com 30 linhas de igualdade/hash/parse copiadas na mão
FAILURE_MODES: Generator que quebra em versão do SDK ou tipo de retorno de serialização inesperado no JSON
REFERENCES: Vogen (SteveDunn) · dotnet/roslyn source generators
CONFIDENCE: CONTEXTUAL
SOURCE: Vogen + quozd/awesome-dotnet
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: backend-023
DOMAIN: backend
TITLE: EF Core para escrita + Dapper para leitura (CQRS físico)
PROBLEM: ORM completo (EF Core) gera queries ineficientes para relatórios; Dapper sozinho perde abstração de escrita
CONTEXT: Acesso a dados em .NET
PRINCIPLE: Escrita precisa de tracking e transações; leitura precisa de SQL direto e velocidade — cada um com a ferramenta certa
RECOMMENDATION: EF Core para o modelo de escrita (agregados, migrations, SaveChanges transacional); Dapper + `Microsoft.Data.SqlClient` para queries de leitura/relatório
WHEN_TO_USE: Aplicação com escrita transacional E leitura de relatórios/agregações
WHEN_NOT_TO_USE: CRUD pequeno onde o custo de manter dois caminhos de acesso não compensa
TRADE_OFFS: Dois caminhos de dados para manter vs performance e semântica correta em cada boundary
EXAMPLE: Repositorio de escrita `OrdersRepository` (EF Core) + `OrdersReadRepository` (Dapper) com query SQL parametrizada
COUNTER_EXAMPLE: Relatório pesado com `Include` aninhado no EF Core carregando o mundo
FAILURE_MODES: Modelo de leitura divergindo do schema real (coluna renomeada sem atualizar a query Dapper)
REFERENCES: EF Core docs · Dapper (DapperLib)
CONFIDENCE: STRONG
SOURCE: thangchung/clean-architecture-dotnet + quozd/awesome-dotnet
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: backend-024
DOMAIN: backend
TITLE: Orleans virtual actors para escala horizontal de estado
PROBLEM: Estado por usuário/pedido em cache distribuído ou lock de banco não escala; concorrência explícita vira bug
CONTEXT: .NET com necessidade de estado granulado distribuído
PRINCIPLE: O ator virtual encapsula estado e mensagens; o runtime (Orleans) trata ativação, migração e placement
RECOMMENDATION: Usar Orleans apenas quando o domínio tem entidades com estado individual e há exigência real de escala horizontal
WHEN_TO_USE: Realtime multiplayer, sistemas de estado por-entidade, alta concorrência por chave
WHEN_NOT_TO_USE: API CRUD comum ou monólito que já atende — Orleans é peso e complexidade operacional
TRADE_OFFS: Runtime/operacional complexo vs eliminação de locks distribuídos e concorrência explícita
EXAMPLE: `IGrain` `IUserGrain { Task<Balance> GetBalance(); }` com estado persistido via storage provider
COUNTER_EXAMPLE: Usar Orleans para um CRUD de catálogo com 100 requests/dia
FAILURE_MODES: Grain com estado mutável compartilhado não serializável ou grain criado por request (sem reaproveitamento)
REFERENCES: dotnet/orleans
CONFIDENCE: CONTEXTUAL
SOURCE: quozd/awesome-dotnet + Orleans docs
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```
