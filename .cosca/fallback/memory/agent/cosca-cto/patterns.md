# cosca-cto — Patterns

## P1 — ADR Decision Template
**Quando usar**: Toda decisão técnica que afeta arquitetura.
**Padrão**: Título + Contexto + Alternativas Consideradas + Decisão + Consequências + Evidências.
**Exemplo**: "gRPC vs REST para serve↔runtime" → contexto: baixa latência, loopback → alternativas: gRPC (já existe proto), REST (mais simples), Unix socket → decisão: gRPC (contrato existente, tipado).

## P2 — Cosca-First Research
**Quando usar**: Antes de qualquer decisão técnica.
**Padrão**: cosca knowledge search "<tópico>" → verificar ADRs existentes → consultar learnings do domínio → só então decidir.
