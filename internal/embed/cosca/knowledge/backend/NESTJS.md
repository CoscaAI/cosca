# 04 — NESTJS INTELLIGENCE

> Stack 04 da Cosca Engineering Intelligence Matrix.
> Doutrina do backend canônico da casa: módulos por feature, DI nativo, decorators como DSL.

## MISSÃO
Construir backends Node/TypeScript com NestJS dominando a arquitetura de módulos por feature, injeção de dependência por constructor e o uso de decorators como linguagem de domínio — com testabilidade e boundaries explícitas.

## PRINCÍPIOS CORE
1. **Módulos por feature** — cada feature tem `<feature>.module.ts` com imports/controllers/providers/exports declarados · UNIVERSAL
2. **DI nativo via constructor** — `@Injectable` + injeção por constructor; nada de service locator manual · UNIVERSAL
3. **Decorators como DSL** — `@Module`, `@Controller`, `@Get`, `@Body`, `@UseGuards`, `@Catch` declaram contrato no próprio código · STRONG
4. **Pipeline por responsabilidade** — Pipes (validação), Guards (auth), Interceptors (transform), Filters (erros) · UNIVERSAL
5. **Backend estruturado e opinionado** — DI + módulos = testabilidade e boundaries explícitas; complexidade proporcional ao problema · STRONG

## REGRAS DE DECISÃO
- `NestFactory.create(AppModule)` como bootstrap; `setGlobalPrefix("api")` para versionar o namespace.
- `ValidationPipe` global (`whitelist: true`, `transform: true`) como padrão fail-closed de entrada.
- Provider só vive no módulo que o usa; exporta quando outro módulo precisa (`exports`).
- Guards no guarda-roupas certo: auth em guard, validação em pipe, transformação em interceptor, erro em filter — cada um no seu lugar.
- Controllers finos (rota + delegação); lógica em services injetáveis e testáveis.
- Monorepo: `apps/` (bins) + `libs/` (módulos compartilhados) quando o monólito modular cresce.

## ANTI-PATTERNS
`service god sem módulo` · `lógica em controller` · `DI por `@Inject` manual quando constructor resolve` · `validação espalhada em vez de ValidationPipe global` · `guard fazendo trabalho de interceptor` · `módulo importando tudo com exports de tudo` · `backend opinionado usado como "só TypeScript com framework"` · `circular dependency entre módulos`

## CHECKLIST
- [ ] Um `<feature>.module.ts` por feature, com imports/controllers/providers/exports explícitos
- [ ] `ValidationPipe` global com whitelist ativo
- [ ] Auth em Guards, transform em Interceptors, erro em Filters, validação em Pipes
- [ ] DI por constructor, sem service locator
- [ ] Controllers finos; lógica em services testáveis
- [ ] `setGlobalPrefix` definido
- [ ] Sem circular dependencies; módulos com escopo claro
- [ ] Teste unitário de service com mocks de dependência injetadas

## A REGRA
NestJS é backend estruturado e opinionado por design: módulos declaram as fronteiras, o DI conecta, e os decorators viram a linguagem — quem respeita o shape ganha testabilidade de graça.

## REFERÊNCIAS
NestJS docs (docs.nestjs.com) · @nestjs/cli · PADRAO-COSCA §6 · backend/README.md · wshobson/agents · ardanlabs/service (princípios)

## CONHECIMENTO (schema _SCHEMA.md)

```
ID: backend-001
DOMAIN: backend
TITLE: Módulos por feature como boundary arquitetural
PROBLEM: Backend cresce sem fronteiras; acoplamento espalha e testes ficam impossíveis
CONTEXT: NestJS, monólito modular
PRINCIPLE: A feature é a unidade de composição; o módulo declara o que importa, oferece e exporta
RECOMMENDATION: Um `<feature>.module.ts` por feature; provider no módulo que o usa; export só o que é contrato
WHEN_TO_USE: Qualquer backend NestJS de tamanho não-trivial
WHEN_NOT_TO_USE: Lambda/proxy de um handler — NestJS é peso desnecessário
TRADE_OFFS: Estrutura rígida vs boundaries que se degradam sem ela
EXAMPLE: `users.module.ts` importa `TypeOrmModule`, declara `UsersService`/`UsersController`, exporta `UsersService`
COUNTER_EXAMPLE: Um `AppModule` gigante com 50 providers e todos os controllers
FAILURE_MODES: Circular dependency por módulo importando módulo em vez de exportar service
REFERENCES: NestJS modules docs
CONFIDENCE: UNIVERSAL
SOURCE: NestJS + PADRAO-COSCA §6
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: backend-002
DOMAIN: backend
TITLE: DI por constructor com @Injectable
PROBLEM: Dependências criadas na mão (new Service) acoplam e impossibilitam teste
CONTEXT: NestJS DI container
PRINCIPLE: O container resolve; o constructor declara o contrato; o teste injeta o mock
RECOMMENDATION: Declarar `@Injectable()` e pedir dependências no constructor; nunca `new` interno
WHEN_TO_USE: Qualquer service/provider NestJS
WHEN_NOT_TO_USE: Value objects/utilidades puras sem estado (funções estáticas bastam)
TRADE_OFFS: Indireção do container vs testabilidade e inversão real
EXAMPLE: `constructor(private readonly usersRepo: Repository<User>) {}`
COUNTER_EXAMPLE: `const repo = new Repository()` dentro do service
FAILURE_MODES: DI resolvendo escopo errado (request-scoped em singleton) vazando estado
REFERENCES: NestJS DI docs
CONFIDENCE: UNIVERSAL
SOURCE: NestJS
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: backend-003
DOMAIN: backend
TITLE: Decorators como DSL de contrato
PROBLEM: Rotas, corpo, guards e errors declarados em config espalhada perdem legibilidade e coesão
CONTEXT: NestJS (TypeScript decorators)
PRINCIPLE: O contrato HTTP vive no decorator, junto do handler — o código fala a língua do domínio
RECOMMENDATION: Usar `@Controller`/`@Get`/`@Post`/`@Body`/`@Param`/`@UseGuards`/`@Catch` declarativamente
WHEN_TO_USE: Definição de endpoints e pipeline
WHEN_NOT_TO_USE: Lógica de negócio — decorator nunca deve esconder comportamento complexo
TRADE_OFFS: Metaprogramação implícita vs coesão e intenção visível
EXAMPLE: `@Get(":id") @UseGuards(JwtGuard) findOne(@Param("id") id: string)`
COUNTER_EXAMPLE: Config de rota em arquivo YAML separado do handler
FAILURE_MODES: Decorator de guard aplicado no lugar errado (método vs controller) abrindo rota
REFERENCES: NestJS controllers docs
CONFIDENCE: STRONG
SOURCE: NestJS + PADRAO-COSCA §6
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: backend-004
DOMAIN: backend
TITLE: Pipes, Guards, Interceptors, Filters — cada um no seu papel
PROBLEM: Validação, auth, transformação e erro misturados em um lugar só vira caos e inconsistência
CONTEXT: Pipeline HTTP do NestJS
PRINCIPLE: Responsabilidade única por etapa: Pipes validam, Guards autorizam, Interceptors transformam, Filters mapeiam erros
RECOMMENDATION: ValidationPipe global (whitelist+transform); guards para auth; interceptors para payload/logging; filters para exceções
WHEN_TO_USE: Todo backend NestJS com entrada de usuário
WHEN_NOT_TO_USE: Endpoint que não recebe input e não tem erro customizado
TRADE_OFFS: Quatro mecanismos para aprender vs fronteiras claras e fail-closed
EXAMPLE: `ValidationPipe` global + `@UseGuards(AuthGuard)` + `@Catch(HttpException)`
COUNTER_EXAMPLE: Validar no controller com ifs e mapear erro na mão em cada handler
FAILURE_MODES: Guard que lança em vez de retornar false, vazando detalhe interno
REFERENCES: NestJS pipeline docs (Pipes/Guards/Interceptors/Filters)
CONFIDENCE: UNIVERSAL
SOURCE: NestJS + PADRAO-COSCA §6
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```
