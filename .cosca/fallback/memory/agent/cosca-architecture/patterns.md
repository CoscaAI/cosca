# cosca-architecture — Patterns

## P1 — Modular Boundary Pattern
**Quando usar**: Definir fronteiras entre pacotes/módulos.
**Padrão**: Interface no consumidor, implementação no produtor. Dependência unidirecional. Testes no pacote consumidor com mock da interface.
**Exemplo**: pipeline.Runner (interface) em internal/pipeline, implementações em engine_adapter.go e orch_adapter.go.

## P2 — Adapter Pattern for External Systems
**Quando usar**: Integrar sistema externo (gRPC, REST, DB).
**Padrão**: Interface pequena (1-2 métodos) + adapter que traduz + fallback nil-safe.
**Exemplo**: KnowledgeSearcherAdapter (gRPC → orchestration.KnowledgeSearcher).

## P3 — Single Owner Pattern
**Quando usar**: Evitar duplicação de estado/engines entre processos.
**Padrão**: Um processo é dono (runtime), outros consomem via contrato (gRPC). Bootstrap com feature flag para ligar/desligar.
**Exemplo**: RuntimeStandalone flag no bootstrap.Compose().
