# ADR-7423: Versionamento de Contratos por Método (RUNTIME CONTRACT v2)

**Status**: Proposed
**Deciders**: Don + Cosca Kernel (Consigliere)
**Date**: 2026-08-01
**Tags**: contracts, versioning, runtime, rpc, architecture
**Source Study**: Traycer `protocol/src/framework/versioned-rpc.ts` (open-source, MIT) — análise registrada em `.cosca/memory/project/traycer-analysis.md`

## Context

O `RUNTIME_CONTRACT.md` (v1.0.0) define a interface Kernel ↔ Runtime de forma **monolítica**: um único protocolo, sem versionamento por método. Consequências práticas observadas:

1. **Qualquer mudança de contrato é tudo-ou-nada** — um campo novo em um método exige coordenação global de todos os runtimes (CLI, API, dashboard, plugins) no mesmo release, ou quebra silenciosa.
2. **Sem regra formal do que é breaking** — não há guard automatizado distinguindo mudança aditiva de mudança destrutiva; o erro só aparece em runtime.
3. **Sem negociação de capacidades** — um runtime antigo não consegue declarar "só sei até o método X" e receber degradação graciosa.
4. **Sem floor** — nenhum conjunto de métodos "imutáveis" que garanta baseline de comunicação entre versões quaisquer.

O estudo do Traycer (framework `versioned-rpc` + `json-schema-fingerprint`, 1.327 linhas) demonstrou um padrão maduro que resolve exatamente esses 4 pontos, validado em produção com 177 métodos RPC versionados. A regra central é verificada **em tempo de carga do registry, falhando o build** — não em runtime.

## Decision

Evoluir o `RUNTIME_CONTRACT.md` para **v2.0.0 com versionamento por método** `{major, minor}`, adotando o padrão do Traycer adaptado à stack Cosca (Go):

1. **Contrato por método**: todo método RPC declara `schemaVersion { major, minor }` + schemas de request/response. O registry central (`internal/contracts/`) é a única fonte de verdade, carregada no boot de todo runtime.

2. **Regra de ouro — minor é somente aditivo**: dentro do mesmo major, uma mudança de schema que remova/renomeie/altere campo existente **falha o build** (validação de additivity em CI). Campos novos: obrigatoriamente opcionais.

3. **Major é obrigatoriamente breaking**: um bump de major sem mudança real nos schemas falha com "could have shipped as a minor". Proíbe major de mentira e força disciplinar real.

4. **Downgrade explícito + floor methods**: cada major declara paths de downgrade desde o latest; métodos fora do floor declaram `degrade` (`unsupported` ou `fallback` adaptando request/response para um método floor). Garante que cliente novo ↔ host antigo conversam sem derrubar a conexão.

5. **Negociação de manifesto**: handshake troca manifesto de capacidades (`method → {major, minor}`), com mirror check em ambos os lados e erro `fatalError` tipado com guidance de upgrade.

## Rationale

- **O nosso problema é real e atual**: o contrato v1.0.0 já tem 8 tipos de request (feature, bug, refactor, review, deploy, docs, status, evolve) e cresce; sem versionamento por método, o próximo campo obrigatório quebra todos os consumidores no mesmo dia.
- **O padrão Traycer é comprovado**: 177 métodos versionados, 111 arquivos de teste no protocol, testes adversarial. A regra de "minor aditivo verificado por fingerprint de schema" transforma erro de compatibilidade em erro de compilação.
- **Não copiamos código**: a stack deles é TypeScript/Zod; a nossa é Go. Adotamos o **padrão e as invariantes**, não a implementação.
- **Custo de adoção baixo agora, alto depois**: o contrato v1 é pequeno (1 doc, poucos métodos em produção). Migrar agora é barato; migrar com 30 métodos em produção é cirurgia.

## Alternatives Considered

1. **Manter contrato monolítico v1.0.0** (status quo): zero trabalho imediato, mas o risco cresce com cada método novo. Rejeitada — resolve nenhum dos 4 problemas.
2. **Versionamento global do protocolo** (ex: `protocolVersion: 2` como MCP): mais simples que por método, mas um único campo novo obriga bump global e coordenação total — não resolve o problema da degradação parcial. Rejeitada como solução única; pode coexistir como versão de envelope.
3. **Adotar MCP como contrato**: padronizado e interoperável, mas resolve "modelo ↔ ferramentas", não "Kernel ↔ runtimes Cosca" com persistência e orquestração de agentes. Rejeitada para este contrato (pode ser usada para integração externa).
4. **Copiar o framework Traycer em TS**: contrato duplicado em outra linguagem fora do nosso runtime Go. Rejeitada — adotamos invariantes, não implementação.

## Decision Outcome

- `RUNTIME_CONTRACT.md` migra para **v2.0.0** com seção formal de versionamento por método (regras 1-5 acima) e exemplo de contrato.
- Novo pacote Go `internal/contracts/` com registry tipado (estrutura de dados + validação estrutural de invariantes, espelhando `validateVersionedRpcRegistry`).
- CI: novo check `make contract-validate` que roda a validação do registry a cada commit (additivity de minors, breaking obrigatório de majors, encadeamento de upgrades, downgrades e degrades).
- Métodos do contrato v1 existentes migram como "floor" inicial (major 1, sem upgrade path — baseline de compatibilidade).

## Consequences

**Positivas**
- Mudança aditiva vira deploy independente por método, sem coordenação global.
- Erro de compatibilidade vira erro de build, não de produção.
- Runtimes antigos conversam com kernel novo via downgrade/degrade (longevidade de deploy).
- Base para publicar o contrato como SDK estável para parceiros.

**Negativas / custos**
- Complexidade inicial: registry tipado + validação em CI (estimativa 8-16h de implementação, faseada).
- Disciplina obrigatória: todo método novo precisa declarar versionamento completo (sem atalho).
- Documentação de compatibilidade precisa ser mantida (politica de minors/majors escrita).

## Compliance

- Regra de ouro (minor aditivo / major breaking) vira gate de CI obrigatório (QUALITY_GATES.md).
- Todo contrato novo deve passar pelo `internal/contracts/` validator antes de merge.
- RUNTIME_CONTRACT.md v2 referencia este ADR como fonte da decisão.

## Related

- [RUNTIME_CONTRACT.md](../../../RUNTIME_CONTRACT.md)
- [runtime/ARCHITECTURE.md](../../../runtime/ARCHITECTURE.md)
- [adr-0001-ai-architecture.md](./adr-0001-ai-architecture.md)
- [.cosca/memory/project/traycer-analysis.md](../../../../memory/project/traycer-analysis.md) (análise da fonte de estudo)
- Traycer (fonte estudada): `github.com/traycerai/traycer` → `protocol/src/framework/versioned-rpc.ts`
