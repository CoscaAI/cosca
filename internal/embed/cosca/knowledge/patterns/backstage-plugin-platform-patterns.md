# Backstage Plugin Platform Patterns

> **Source**: https://github.com/backstage/backstage — Apache 2.0, ~12.000 arquivos, monorepo Yarn  
> **Analyzed**: 2026-08-09 — Análise profunda cross-agent (3 agentes em paralelo)  
> **Confidence**: 0.93 (validado por leitura direta de código fonte: 3 reports cross-agent em ~200 arquivos-chave)

## Intent

Extrair padrões reutilizáveis da plataforma líder de portais de desenvolvedores — arquitetura de plugins, sistema de DI, motor de templates, pipeline de processamento, busca multi-engine, permissão condicional, sistema de eventos, test harness.

## Context

O Backstage é a plataforma open-source do Spotify (hoje CNCF Incubating) para construir portais de desenvolvedores. É referência mundial em arquitetura de plugins extensíveis com:
- **Monorepo Yarn** com 150+ plugins + 71 packages + 12.000 arquivos
- **TypeScript/React** frontend + **Node.js/Express** backend
- **Plugin system com 3 gerações**: v1 (core-plugin-api, deprecated) → v2 (frontend-plugin-api, atual) → v3 (extension tree, novo)
- **Backend com DI container**: ServiceRef<T> + ServiceFactory com escopos root/plugin
- **Catalog**: modelo de entidade tipo Kubernetes com pipeline de processamento multi-estágio
- **Scaffolder**: motor de templates com steps encadeados e actions plugáveis
- **150+ módulos**: auth (20+ providers), catalog (15+ providers), scaffolder (12+ actions), search (3 engines), events (5+ transports)

## Patterns Extraídos (Top 15 para Cosca)

### 1. Plugin-Module-ExtensionPoint Triad (Sistema de Extensibilidade)

**O que é**: Três entidades que compõem o sistema de extensibilidade do Backstage. O **Plugin** (host) declara contratos via `createExtensionPoint<T>()`. O **Module** (contribuidor) implementa esses contratos. A plataforma resolve o grafo de dependências e injeta as implementações no plugin.

**Contrato exato (backend)**:
```typescript
// Plugin define o ponto de extensão
const scaffolderActionsExtensionPoint = createExtensionPoint<ScaffolderActionsExtensionPoint>({
  id: 'scaffolder.actions',
});

// Plugin registra e inicializa
createBackendPlugin({
  pluginId: 'scaffolder',
  register(env) {
    env.registerExtensionPoint(scaffolderActionsExtensionPoint, {
      addActions(...actions) { /* coleta actions */ }
    });
    env.registerInit({
      deps: { logger: coreServices.logger, config: coreServices.rootConfig },
      async init({ logger, config }) {
        // Cria router com actions coletadas
      }
    });
  }
});

// Module implementa o contrato
createBackendModule({
  pluginId: 'scaffolder',
  moduleId: 'github-actions',
  register(env) {
    env.registerInit({
      deps: { scaffolder: scaffolderActionsExtensionPoint },
      async init({ scaffolder }) {
        scaffolder.addActions(createGithubAction());
      }
    });
  }
});
```

**Por que importa para Cosca**: Nosso modelo de agentes (CEOs → Chiefs → Specialists) já é uma hierarquia, mas não tem a flexibilidade de contribuição lateral via módulos. Cada "agente-chefe" poderia declarar extension points que agentes-module implementam. O workflow engine (L153) poderia usar módulos para adicionar templates de workflow sem tocar no core. O catalog de conhecimento poderia aceitar entity providers externos.

**Naming convention**: `plugin-{name}-backend` (host), `plugin-{name}-backend-module-{feature}` (módulo), `plugin-{name}-node` (contratos), `plugin-{name}-common` (tipos compartilhados).

**Arquivos-chave**: `packages/backend-plugin-api/src/wiring/createBackendPlugin.ts`, `createBackendModule.ts`, `createExtensionPoint.ts`

---

### 2. ServiceRef DI Container (Injeção de Dependência Tipada)

**O que é**: Container de DI declarativo onde serviços são referenciados por `ServiceRef<TService, TScope, TInstances>`. As factories declararam dependências explicitamente via `deps` map. O motor resolve o grafo e inicializa em ordem topológica.

**Contrato exato**:
```typescript
// ServiceRef — token de DI com tipo, escopo, cardinalidade
type ServiceRef<TService, TScope extends 'root' | 'plugin', TInstances extends 'singleton' | 'multiton'> = {
  id: string;
  scope: TScope;         // 'root' = global, 'plugin' = por plugin
  multiton?: boolean;    // true = múltiplas instâncias, dependente recebe T[]
  T: TService;           // TYPE CARRIER — nunca contém valor real
  $$type: '@backstage/ServiceRef';
};

// Factory produz instância
interface InternalServiceFactory<TService, TScope, TInstances> {
  service: ServiceRef<TService, TScope, TInstances>;
  version: 'v1';
  initialization?: 'always' | 'lazy';   // eager vs lazy init
  deps: { [key in string]: ServiceRef<unknown> };
  factory(deps: { ... }): Promise<TService>;
}
```

**Regras de escopo**:
- `root`: disponível para todos serviços; só pode depender de outros root-scoped
- `plugin`: escopo por plugin (instância separada para cada plugin que pede); pode depender de qualquer serviço
- `singleton`: uma instância por escopo
- `multiton`: array de instâncias (para extension points implementados por múltiplos módulos)

**19 core services**: `auth`, `userInfo`, `cache`, `rootConfig`, `database`, `discovery`, `httpAuth`, `httpRouter`, `lifecycle`, `logger`, `permissions`, `scheduler`, `urlReader`, etc.

**Por que importa para Cosca**: O runtime Cosca já tem `bootstrap.Compose()` que cria engines compartilhadas (L154), mas a composição é hardcoded. Um container de DI com ServiceRef tipado permitiria que agentes declarassem suas dependências (ex: "preciso de `knowledge.Engine`, `memory.Engine`, `logger.Service`") e o runtime resolvesse automaticamente. O `InternalServiceFactory` com `deps` map é o padrão que nosso bootstrap deveria adotar.

**Arquivos-chave**: `packages/backend-plugin-api/src/services/system/types.ts`, `packages/backend-defaults/src/CreateBackend.ts`

---

### 3. Extension Tree Model (Frontend — Árvore de Extensões com Data Flow)

**O que é**: Sistema de composição de UI onde extensões formam uma **árvore direcionada**. Cada extensão declara: (a) onde se conecta (`attachTo`), (b) o que produz (`output: Array<ExtensionDataRef>`), (c) o que consome (`inputs`). A árvore é resolvida topologicamente e instanciada bottom-up — cada `factory()` recebe inputs já resolvidos e produz outputs tipados.

**Contrato exato**:
```typescript
// Extensão com 5 elementos
createExtension({
  kind: 'page',
  name: 'catalog',
  attachTo: { id: 'app/routes', input: 'routes' },
  inputs: {
    entities: createExtensionInput([catalogEntityContentExtensionData], {
      singleton: false,
      optional: true,
    }),
  },
  output: [
    coreExtensionData.routePath,     // string
    coreExtensionData.reactElement,  // JSX.Element
    coreExtensionData.routeRef,      // RouteRef
  ],
  configSchema: z.object({ path: z.string().default('/catalog') }),
  factory({ node, apis, config, inputs }) {
    // inputs.entities é Array<{ content: JSX.Element }>
    return [
      coreExtensionData.routePath(config.path),
      coreExtensionData.reactElement(<CatalogPage entities={inputs.entities} />),
      coreExtensionData.routeRef(rootRouteRef),
    ];
  }
});

// 5 tipos de dados core
export const coreExtensionData = {
  reactElement: createExtensionDataRef<JSX.Element>().with({ id: 'core.reactElement' }),
  routePath:    createExtensionDataRef<string>().with({ id: 'core.routing.path' }),
  routeRef:     createExtensionDataRef<RouteRef>().with({ id: 'core.routing.ref' }),
  title:        createExtensionDataRef<string>().with({ id: 'core.title' }),
  icon:         createExtensionDataRef<IconElement>().with({ id: 'core.icon' }),
};
```

**AppNode (nó da árvore)**:
```typescript
type AppNode = {
  spec: AppNodeSpec;        // { id, extension, config, plugin? }
  edges: AppNodeEdges;      // { attachments: { [input]: AppNode[] } }
  instance?: AppNodeInstance; // { data: Map, errors: AppError[] }
};
```

**Resolução**: `features[] → resolveAppNodeSpecs() → resolveAppTree() → instantiateAppNodeTree()`.
A fábrica de cada nó é chamada com inputs resolvidos, produz outputs que sobem na árvore. Erros são coletados (nunca crasham a árvore inteira).

**Por que importa para Cosca**: Este é o padrão mais avançado de composição declarativa que encontramos. Nossa UI do Command Center poderia ser composta da mesma forma: widgets declaram o que produzem (métrica, gráfico, tabela) e onde se encaixam (dashboard, sidebar, overlay). O workflow engine (L153) já usa DAG — a árvore de extensões é o equivalente para composição de UI e funcionalidades.

**Arquivos-chave**: `packages/frontend-plugin-api/src/wiring/createExtension.ts`, `createExtensionDataRef.ts`, `createExtensionInput.ts`, `packages/frontend-app-api/src/tree/instantiateAppNodeTree.ts`

---

### 4. Blueprint Extension Factory (Fábrica de Extensões Pré-configuradas)

**O que é**: `ExtensionBlueprint` encapsula o boilerplate de criar extensões para casos comuns. Em vez de chamar `createExtension()` com todos os parâmetros, o usuário chama `PageBlueprint.make({ params })`. A blueprint define `kind`, `attachTo`, `output`, e `factory` padrão — o usuário só fornece o que varia.

**Exemplo real**:
```typescript
// Definição da blueprint (uma vez)
export const PageBlueprint = createExtensionBlueprint({
  kind: 'page',
  attachTo: { id: 'app/routes', input: 'routes' },
  output: [coreExtensionData.routePath, coreExtensionData.reactElement, ...],
  factory(params, { node }) {
    return [
      coreExtensionData.routePath(params.path),
      coreExtensionData.reactElement(params.loader()),
      ...
    ];
  },
});

// Uso (cada página)
const catalogPage = PageBlueprint.makeWithOverrides({
  name: 'catalog',
  params: { path: '/catalog', loader: () => import('./CatalogPage') },
  inputs: { entities: createExtensionInput([entityContentExtensionData]) },
});

// Blueprints disponíveis
PageBlueprint, ApiBlueprint, AppRootElementBlueprint, 
EntityCardBlueprint, HomePageWidgetBlueprint, ...
```

**Por que importa para Cosca**: Nossos templates de workflow (L153) já são um primitivo disso — o Planner tem 5 templates determinísticos. O conceito de Blueprint eleva isso: em vez de templates de código, são fábricas parametrizadas que produzem artefatos com contrato garantido. Os 55 tipos de agentes poderiam ser blueprints com parâmetros específicos.

**Arquivos-chave**: `packages/frontend-plugin-api/src/wiring/createExtensionBlueprint.ts`, `packages/frontend-plugin-api/src/blueprints/`

---

### 5. Catalog Processing Pipeline (Pipeline Multi-Estágio com Processadores)

**O que é**: Pipeline de processamento de entidades com 7 estágios sequenciais. Cada estágio itera sobre `CatalogProcessor[]` registrados. Processadores emitem resultados via callback (`entity`, `relation`, `error`, `refresh`, `deferredEntity`). O orchestrator coleta todos os resultados e passa para o próximo estágio.

**Estágios em ordem**:
```
1. validateEnvelope       → Validação estrutural básica
2. preProcessEntity       → Enriquecimento pré-validação
3. enforcePolicy          → Políticas de entidade (schema, namespace, campos)
4. validateEntityKind     → Pelo menos um processor reconhece o kind
5. readLocation           → Se LocationEntity, ler e emitir entidades filhas
6. postProcessEntity      → Enriquecimento pós-validação
7. rules enforcement      → Validar regras de catálogo nos deferred entities
```

**CatalogProcessor interface**:
```typescript
type CatalogProcessor = {
  getProcessorName(): string;
  readLocation?(location, optional, emit, parser, cache): Promise<boolean>;
  preProcessEntity?(entity, location, emit, originLocation, cache): Promise<Entity>;
  validateEntityKind?(entity): Promise<boolean>;
  postProcessEntity?(entity, location, emit, cache): Promise<Entity>;
  getPriority?(): number;
};

// Emit callback:
type CatalogProcessorEmit = (generated: CatalogProcessorResult) => void;
// CatalogProcessorResult = EntityResult | LocationResult | RelationResult | ErrorResult | RefreshResult
```

**Processing Engine**: Polling loop que carrega lotes de entidades pendentes → processa via orchestrator → atualiza DB → dispara stitching para entidades com relações alteradas.

**Por que importa para Cosca**: Este é o padrão canônico para pipeline de processamento de dados declarativos. Nossa esteira de agentes (planner → build → test → review) poderia ser refatorada como um pipeline de processadores com a mesma interface: cada estágio recebe entidade + emit callback, produz resultados. O padrão de `emit` callback (em vez de return) permite que um processor emita múltiplas entidades/relações derivadas de uma entrada.

**Entity Providers**: Entidades não entram só via locations YAML — `EntityProvider` é uma interface que "empurra" entidades de fontes externas (GitHub org, LDAP, GCP, etc.). O catálogo tem built-in providers + módulos adicionam mais.

**Arquivos-chave**: `plugins/catalog-backend/src/processing/DefaultCatalogProcessingOrchestrator.ts`, `plugins/catalog-node/src/api/processor.ts`, `packages/catalog-model/src/entity/Entity.ts`

---

### 6. Template Engine & Action System (Scaffolder)

**O que é**: Sistema de templates que combina: (1) definição declarativa de template (YAML como entity do catálogo), (2) JSONSchema para formulários de entrada, (3) steps encadeados com actions plugáveis, (4) renderização Nunjucks em todas as strings de input, (5) checkpoint para retry idempotente, (6) loops (`each`) e condicionais (`if`) nos steps.

**TaskSpec**:
```typescript
interface TaskSpec {
  apiVersion: 'scaffolder.backstage.io/v1beta3';
  steps: TaskStep[];
  output: { [name: string]: JsonValue }; // Templates Nunjucks para extração de output
}

interface TaskStep {
  id: string;           // Identificador único
  name: string;         // Nome de exibição
  action: string;       // ID da action (ex: 'fetch:template', 'publish:github')
  input?: JsonObject;   // Input com templates Nunjucks
  if?: string | boolean;   // Condição de execução
  each?: string | JsonArray; // Loop — repete o step para cada item
}

// TemplateAction — contrato de action
interface TemplateAction<TActionInput, TActionOutput> {
  id: string;
  description?: string;
  schema?: {
    input?: Schema;    // JSONSchema via Zod para validação do input
    output?: Schema;   // Schema do output
  };
  handler: (ctx: ActionContext<TActionInput, TActionOutput>) => Promise<void>;
}
```

**ActionContext**:
```typescript
interface ActionContext<TInput, TOutput> {
  logger: Logger;
  input: TInput;           // Input validado e tipado
  output(name: string, value: unknown): void;  // Define output do step
  checkpoint<T>(opts: { key: string; fn: () => Promise<T> }): Promise<T>; // Idempotente
  createTemporaryDirectory(): Promise<string>;
  workspacePath: string;
  task: { id: string };
  isDryRun: boolean;
  getInitiatorCredentials(): Promise<BackstageCredentials>;
}
```

**Checkpoint (retry idempotente)**: `ctx.checkpoint({ key: 'clone', fn: () => git.clone(url) })` — o resultado é armazenado no estado da task. Em retry, o checkpoint retorna o valor salvo sem reexecutar a função. Isso é o padrão para operações caras/idempotentes.

**Renderização Nunjucks**: O método `render()` usa `JSON.parse(JSON.stringify(input))` com um reviver que passa TODA string pelo Nunjucks — funciona em qualquer profundidade do objeto. Templates têm acesso a: `steps.{id}.output.{name}`, `parameters`, `secrets`, `each.key`, `each.value`.

**Por que importa para Cosca**: O sistema de steps + actions é o que nossa esteira de workflow (L153) deveria ser. Em vez de templates hardcoded, um motor de templates com actions plugáveis permite que qualquer agente defina novas ações. O checkpoint pattern resolve o problema de passos que falham no meio e precisam de retry. O `ActionContext.checkpoint()` é elegante: salva estado em DB, retorna cache em retry.

**Arquivos-chave**: `plugins/scaffolder-common/src/TaskSpec.ts`, `plugins/scaffolder-node/src/actions/types.ts`, `plugins/scaffolder-backend/src/scaffolder/tasks/NunjucksWorkflowRunner.ts`

---

### 7. Search Engine Abstraction (Bridge Pattern Multi-Backend)

**O que é**: Interface única `SearchEngine` com 3 métodos que abstrai Lunr (in-memory), Elasticsearch, e PostgreSQL. Indexação via streams (Readable → Transform → Writable). Consulta via `QueryTranslator` que converte query abstrata para query engine-específica.

**SearchEngine interface**:
```typescript
interface SearchEngine {
  setTranslator(translator: QueryTranslator): void;
  getIndexer(type: string): Promise<Writable>;  // Stream de escrita para indexar
  query(query: SearchQuery, options?): Promise<IndexableResultSet>;
}

type QueryTranslator = (query: SearchQuery) => unknown; // Abstrato → concreto
```

**Indexação via Streams**:
```
DocumentCollatorFactory.getCollator() → Readable
  → [DocumentDecoratorFactory.getDecorator()] → Transform
    → SearchEngine.getIndexer(type) → Writable (BatchSearchEngineIndexer)
```

**BatchSearchEngineIndexer**: Classe base que implementa batching — subclasses só precisam implementar `initialize()`, `index(docs[])`, `finalize()`. O resto (buffer, flush, backpressure) é herdado.

**AuthorizedSearchEngine (Decorator Pattern)**: Wraps o SearchEngine real e filtra resultados por permissão. Se todas as permissões são ALLOW/DENY → filtra types antes da query. Se há CONDITIONAL → busca páginas extras e filtra resultado por resultado via DataLoader (batching).

**Por que importa para Cosca**: Nossa busca híbrida FTS5 + vector search (L163) é exatamente esse padrão: interface única, múltiplos backends. O que falta: (1) o decorator de autorização — search results filtrados por permissão do agente/sessão, (2) o padrão de collator/decorator como plugins — novos tipos de documento podem ser indexados sem modificar o core de busca. O `QueryTranslator` é o que nosso `cosca knowledge search` faz manualmente (BM25 para FTS5, cosseno para vector).

**Arquivos-chave**: `plugins/search-backend-node/src/types.ts`, `plugins/search-backend-node/src/IndexBuilder.ts`, `plugins/search-backend/src/service/AuthorizedSearchEngine.ts`

---

### 8. Permission Framework (RBAC/ABAC com Condições Compostas)

**O que é**: Sistema de autorização em 3 camadas: (1) `PermissionPolicy` (definida pelo usuário) decide ALLOW/DENY/CONDITIONAL, (2) `PermissionEvaluator` é a interface padronizada que plugins chamam, (3) `PermissionRules` são condições que plugins registram para avaliar decisões condicionais.

**Fluxo completo**:
```
User → PermissionPolicy.handle(query, user) → PolicyDecision
  ├── { result: 'ALLOW' | 'DENY' } → Definitive
  └── { result: 'CONDITIONAL', pluginId, resourceType, conditions } → Conditional
    → Plugin avalia PermissionCriteria usando PermissionRules
      ├── apply(resource, params) → boolean (in-memory)
      └── toQuery(params) → query filter (database-level)
```

**Permissões**:
```typescript
type Permission = BasicPermission | ResourcePermission;
type BasicPermission = { type: 'basic'; name: string; attributes: PermissionAttributes };
type ResourcePermission<T> = { type: 'resource'; name: string; resourceType: T; attributes };
// attributes: { action?: 'create' | 'read' | 'update' | 'delete' }
```

**PermissionCriteria (AND/OR/NOT de condições)**:
```typescript
type PermissionCriteria<TQuery> =
  | { allOf: NonEmptyArray<PermissionCriteria<TQuery>> }  // AND
  | { anyOf: NonEmptyArray<PermissionCriteria<TQuery>> }  // OR
  | { not: PermissionCriteria<TQuery> }                    // NOT
  | TQuery;                                                // Leaf condition
```

**PermissionRule — o coração**:
```typescript
interface PermissionRule<TResource, TQuery, TResourceType, TParams> {
  name: string;
  resourceType: TResourceType;
  paramsSchema?: ZodSchema<TParams>;
  apply(resource: TResource, params: TParams): boolean;       // In-memory
  toQuery(params: TParams): PermissionCriteria<TQuery>;       // DB-level
}
```

**Condition Authorizer**: `createConditionAuthorizer(rules)` retorna uma função que avalia `PolicyDecision` contra um recurso real — verifica se a condição se aplica. `createConditionTransformer(rules)` retorna função que converte condições para filtros de query SQL.

**Por que importa para Cosca**: Nosso sistema de permissão atual é binário (JWT válido/inválido). O Backstage mostra como evoluir para permissão condicional: "agente X pode executar workflow Y se o workflow for do tipo 'fix-bug' E a confiança do agente for >0.7". `PermissionCriteria` com composição AND/OR/NOT é o padrão. O dual `apply()/toQuery()` (memória vs database) é genial: a mesma regra funciona em ambos os contextos sem duplicação.

**Arquivos-chave**: `plugins/permission-common/src/types/api.ts`, `plugins/permission-node/src/types.ts`, `plugins/permission-node/src/integration/createPermissionRule.ts`

---

### 9. Event System (Pub/Sub Tri-Modal)

**O que é**: Sistema de eventos com 3 modos de operação configuráveis: (a) `never` — apenas barramento local in-process, (b) `auto` — local + remoto se events-backend disponível, fallback silencioso, (c) `always` — local + remoto, falha se remoto indisponível.

**EventsService**:
```typescript
interface EventsService {
  publish(params: EventParams): Promise<void>;
  subscribe(options: EventsServiceSubscribeOptions): Promise<void>;
}

interface EventParams<TPayload = unknown> {
  topic: string;
  eventPayload: TPayload;
  metadata?: Record<string, string | string[] | undefined>;
}
```

**Barramento local** (in-process): `Map<topic, subscription[]>` em memória. `publish()` itera subscribers e chama `onEvent` com isolamento de erro (um subscriber com erro não afeta os outros).

**Bridging remoto**: `PluginEventsService` delega ao `LocalEventBus` primeiro, depois POST ao events-backend. No subscribe, registra local + long-polls o backend com backoff exponencial (1s → 60s, fator 2x).

**SubTopicEventRouter**: Hierarquia de tópicos. Subscriber registra em `"github"`, router despacha para `"github.push"`, `"github.pull_request"`, etc. Baseado no payload.

**Por que importa para Cosca**: Nosso event bus é implícito (agentes chamam outros via kernel). Um barramento explícito com tópicos e subscribers permitiria que agentes reagissem a eventos sem acoplamento direto. O modo tri-modal (local/auto/always) é o padrão para resiliência: em dev, local puro; em staging, auto com fallback; em produção, always para garantia de entrega.

**Arquivos-chave**: `plugins/events-node/src/api/EventsService.ts`, `plugins/events-node/src/api/DefaultEventsService.ts`, `plugins/events-backend/src/service/EventsPlugin.ts`

---

### 10. Test Harness Multi-Database com DI Isolation

**O que é**: Três camadas de teste que vão de unitário isolado a integração multi-DB:

**1. mockServices catálogo**: Fábricas mock para todos os serviços core. Ex: `mockServices.permissions.mock({ result: AuthorizeResult.ALLOW })`, `mockServices.logger.mock()`.

**2. ServiceFactoryTester**: Testa uma factory de serviço em isolamento com DI real:
```typescript
const tester = ServiceFactoryTester.from(myServiceFactory, { dependencies: [otherFactory] });
const service = await tester.getSubject();     // Serviço instanciado
const dep = await tester.getService(otherRef); // Dependência resolvida
```

**3. TestDatabases**: Spawna instâncias efêmeras de SQLite, PostgreSQL, MySQL:
```typescript
const databases = TestDatabases.create();
it.each(databases.eachSupportedId())('funciona em %s', async (dbId) => {
  const knex = await databases.init(dbId);
  // Testa com banco real
}, 60_000);
```

**4. TestBackend**: Inicializa o backend completo com serviços mock e plugins reais:
```typescript
const backend = TestBackend.fromConfig({
  services: [mockServices.logger.factory(), mockServices.database.factory()],
  extensionPoints: [[scaffolderActionsExtensionPoint, mockActions]],
  features: [scaffolderPlugin],
});
await backend.start();
const response = await request(backend.server).post('/api/...');
```

**Por que importa para Cosca**: Nossos testes são `go test ./...` em um único DB SQLite. O padrão `TestDatabases` com `eachSupportedId()` rodando o mesmo teste em 3 bancos é ouro para evitar surpresas de compatibilidade SQL. O `ServiceFactoryTester` com DI isolado é o equivalente a testar uma engine Cosca com todas as dependências mockadas.

**Arquivos-chave**: `packages/backend-test-utils/src/database/TestDatabases.ts`, `packages/backend-test-utils/src/wiring/ServiceFactoryTester.ts`, `packages/backend-test-utils/src/services/mockServices.ts`

---

### 11. Config Schema Compilation (Validação Composta de Config)

**O que é**: Cada plugin/pacote define seu schema JSON Schema em `config.d.ts` ou `config.schema.json`. O `@backstage/config-loader` compila todos os schemas do workspace em um único JSON Schema combinado. Validação via AJV com mensagens de erro legíveis. Suporte a `visibility: 'frontend' | 'backend' | 'secret'` para filtrar config por contexto de execução.

**Por que importa para Cosca**: Nosso `config.yaml` não tem validação de schema. Plugins/agentes poderiam declarar seu schema de config, e o runtime validar na inicialização — detectando config inválida antes de causar erro em runtime. O marcador `secret` é essencial: impede que valores sensíveis vazem para o frontend ou logs.

**Arquivos-chave**: `packages/config-loader/src/schema/compile.ts`, `packages/config/src/types.ts`

---

### 12. Entity Model Kubernetes-Inspired

**O que é**: Todas as entidades do sistema seguem a mesma estrutura `{ apiVersion, kind, metadata, spec, relations? }` — idêntica ao modelo de recursos do Kubernetes. Entidades são identificadas por refs compostas: `kind:namespace/name`. Relações são triplas `{ type, targetRef }`.

**Entity**:
```typescript
type Entity = {
  apiVersion: string;                    // ex: 'backstage.io/v1alpha1'
  kind: string;                          // ex: 'Component', 'API', 'System', 'User', 'Template'
  metadata: {
    name: string;
    namespace?: string;                  // default: 'default'
    labels?: Record<string, string>;     // Kubernetes-style labels
    annotations?: Record<string, string>; // Metadata extensível
    tags?: string[];
    links?: EntityLink[];
  };
  spec?: JsonObject;                     // Kind-specific data
  relations?: EntityRelation[];          // Conexões com outras entidades
};
```

**Kinds padrão**: `Component`, `API`, `System`, `Domain`, `Resource`, `User`, `Group`, `Location`, `Template`.

**Por que importa para Cosca**: Nosso modelo de conhecimento (agents, skills, patterns, learnings) são entidades dispersas — cada uma com seu formato. Unificar sob um modelo Entity com `kind` permitiria: (a) busca unificada por kind, label, ou relação; (b) navegação por grafo de relações (L163 já tem graph!); (c) catálogo auto-descritivo. O `metadata.annotations` como extensão livre é o padrão para dados específicos de cada kind sem precisar de schema migration.

**Arquivos-chave**: `packages/catalog-model/src/entity/Entity.ts`, `packages/catalog-model/src/kinds/`

---

### 13. Notification Processor Pipeline

**O que é**: Pipeline de processamento de notificações em 5 estágios: `processOptions()` → `resolveRecipients()` → `preProcess()` → save + deliver → `postProcess()`. Cada estágio é um `NotificationProcessor` registrado por módulos.

**NotificationProcessor**:
```typescript
interface NotificationProcessor {
  getName(): string;
  processOptions?(options: NotificationSendOptions): Promise<NotificationSendOptions>;
  preProcess?(notification: Notification, options): Promise<Notification>;
  postProcess?(notification: Notification, options): Promise<void>;
  getNotificationFilters?(): NotificationProcessorFilters;
}
```

**Recipientes**: `{ type: 'entity'; entityRef: string | string[] }` (usuários/grupos específicos) ou `{ type: 'broadcast' }` (todos).

**Por que importa para Cosca**: Nosso sistema de notificação é inexistente — o kernel só reporta ao Don via console. Um pipeline de notificação permitiria: (a) agentes notificarem eventos (task concluída, erro crítico), (b) roteamento para canais (Telegram, Slack, e-mail) via processadores, (c) processamento condicional (filtrar notificações de baixa prioridade).

**Arquivos-chave**: `plugins/notifications-node/src/extensions.ts`, `plugins/notifications-node/src/service/DefaultNotificationService.ts`

---

### 14. Module Federation Dynamic Loading

**O que é**: Sistema de carregamento dinâmico de plugins frontend via Webpack Module Federation. O `frontend-dynamic-feature-loader` busca remotes de uma API backend (`dynamic-plugins-info`), carrega cada um via `instance.loadRemote()`, e verifica se o export default é um `FrontendFeature` (via `$$type` discriminator).

**Backend counterpart**: `backend-dynamic-feature-service` descobre plugins dinâmicos (de npm ou locais), resolve suas dependências, e expõe informações para o frontend.

**Por que importa para Cosca**: Nossos agentes e skills são compilados estaticamente. O carregamento dinâmico permitiria adicionar skills e agentes sem rebuild do binário — baixar de um registry, verificar assinatura, carregar dinamicamente. O `$$type` discriminator é o padrão para type-check em runtime (TypeScript types são apagados, mas `$$type` sobrevive).

**Arquivos-chave**: `packages/frontend-dynamic-feature-loader/src/loader.ts`, `packages/backend-dynamic-feature-service/src/loader.ts`

---

### 15. Opaque Type Discriminators (Runtime Type Safety)

**O que é**: Todo plugin, extensão, service ref, e feature usa um campo `$$type` string literal como discriminator de tipo em runtime. Ex: `$$type: '@backstage/FrontendPlugin'`, `$$type: '@backstage/ServiceRef'`, `$$type: '@backstage/BackendFeature'`. Isso permite type-checking em runtime onde TypeScript não alcança — verificação de `FrontendFeature` em módulos carregados dinamicamente, validação de plugin registration, etc.

```typescript
// Todos os tipos fundamentais carregam $$type
interface BackendFeature { $$type: '@backstage/BackendFeature'; }
interface FrontendPlugin { $$type: '@backstage/FrontendPlugin'; }
interface ServiceRef { $$type: '@backstage/ServiceRef'; }
interface ExtensionPoint<T> { $$type: '@backstage/ExtensionPoint'; }
interface ExtensionDataRef<T> { $$type: '@backstage/ExtensionDataRef'; }

// Uso: verificar se um módulo carregado dinamicamente é realmente um plugin
if (loadedModule.default?.$$type === '@backstage/FrontendPlugin') {
  // seguro para usar como plugin
}
```

**Por que importa para Cosca**: Nossos agentes, skills, e workflows não têm runtime type safety. Se um agente malicioso ou bugado se passar por skill válida, não temos verificação. `$$type` discriminators + verificação no registro resolveriam. É o equivalente em TS do `interface{}` type assertion em Go com `comma, ok := val.(Type)`.

**Arquivos-chave**: `packages/backend-plugin-api/src/types.ts`, `packages/frontend-plugin-api/src/wiring/types.ts`

---

## Patterns Não Copiados (Anti-Patterns ou Não Aplicáveis)

| Pattern | Razão para não copiar |
|---------|----------------------|
| **Schema monolítico** via Prisma | Preferimos schemas modulares por domínio |
| **Dual frontend API** (v1/v2 coexistindo) | Complexidade desnecessária — manter uma API e migrar de vez |
| **Nunjucks como template engine** | Muito acoplado ao Node.js; Go tem `text/template` nativo |
| **Catalog como polling loop** | Para Cosca, event-driven (CDC/outbox) é superior |
| **YAML-based entity definitions** | Preferimos nosso knowledge.db com schema tipado |

---

## Confidence Tracking

| # | Pattern | Confidence | Validated By |
|---|---------|:----------:|--------------|
| 1 | Plugin-Module-ExtensionPoint Triad | 0.95 | 150+ plugins usando |
| 2 | ServiceRef DI Container | 0.95 | 19 core services + todas factories |
| 3 | Extension Tree Model | 0.93 | Novo sistema frontend inteiro |
| 4 | Blueprint Extension Factory | 0.90 | 5+ blueprints built-in |
| 5 | Catalog Processing Pipeline | 0.94 | Pipeline em produção Spotify/CNCF |
| 6 | Template Engine & Action System | 0.93 | Scaffolder maduro (v1beta3) |
| 7 | Search Engine Abstraction | 0.91 | 3 backends + Authorized decorator |
| 8 | Permission Framework (RBAC/ABAC) | 0.92 | AND/OR/NOT + dual apply/toQuery |
| 9 | Event System Tri-Modal | 0.88 | 5+ event transport modules |
| 10 | Test Harness Multi-Database | 0.91 | SQLite + PG + MySQL em CI |
| 11 | Config Schema Compilation | 0.89 | Validação AJV composta |
| 12 | Entity Model Kubernetes-Inspired | 0.93 | Modelo central de toda plataforma |
| 13 | Notification Processor Pipeline | 0.86 | Módulos email + Slack |
| 14 | Module Federation Dynamic Loading | 0.85 | Funcional mas complexo (webpack) |
| 15 | Opaque Type Discriminators | 0.92 | Presente em toda API pública |

**Average confidence**: ~0.91

---

## Top 5 Para Implementação Imediata na Cosca

1. **ServiceRef DI Container** (Pattern 2) — Refatorar `bootstrap.Compose()` para DI declarativo
2. **Plugin-Module-ExtensionPoint Triad** (Pattern 1) — Agentes declararem extension points, módulos implementarem
3. **Permission Framework** (Pattern 8) — Evoluir de JWT binário para permissão condicional com AND/OR/NOT
4. **Entity Model** (Pattern 12) — Unificar agents/skills/patterns/learnings sob modelo Entity
5. **Test Harness** (Pattern 10) — TestDatabases multi-DB + ServiceFactoryTester para engines

---
