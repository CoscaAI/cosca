# 04 — JAVA INTELLIGENCE

> Stack 04 da Cosca Engineering Intelligence Matrix.
> Doutrina do backend canônico da casa em Java: Spring Boot, arquitetura de referência de monorepo, virtual threads e dinheiro como BigDecimal.

## MISSÃO
Construir backends Java 21 com Spring Boot 3 adotando a arquitetura de referência do monorepo da casa (apps/libs/shared/infrastructure), arquitetura imposta por teste (ArchUnit), consistência eventual via outbox e configuração fail-closed sem segredos no código.

## PRINCÍPIOS CORE
1. **Monorepo com BOM unificado** — `apps/` (api, worker) + `libs/` (domain, security, observability) + `infrastructure/db` + `shared/` (contracts) sob gradle/maven com version catalog · UNIVERSAL
2. **Arquitetura imposta por teste** — ArchUnit faz a arquitetura falhar no CI quando alguém viola as boundaries; a arquitetura é código de teste, não documento · STRONG
3. **Virtual threads para concorrência** — Java 21 virtual threads como default para I/O-bound; sem pools gigantes e thread-per-request como na era pré-Loom · STRONG
4. **Dinheiro é BigDecimal** — nunca `double`/`float` para valores monetários; arredondamento explícito no ponto de decisão · UNIVERSAL
5. **Config fail-closed** — `@ConfigurationProperties` tipado; segredos nunca no código; Jasypt para props cifradas · UNIVERSAL

## REGRAS DE DECISÃO
- Bounded context define os pacotes `controller/service/repository/domain` — o pacote é a fronteira, ArchUnit a vigia.
- Spring Boot 3 + Java 21: WebFlux reativo ou MVC com virtual threads, conforme o perfil de I/O do serviço — escolha documentada, não por modismo.
- Workers como consumidores Kafka + job-scheduling durável (Temporal/Conductor) separados da API; a API não faz job em background.
- Migrations sempre versionadas (Flyway/Liquibase) na `infrastructure/db`; nunca schema evoluído na mão.
- Imagem via Jib (build sem daemon Docker) integrada ao build; nunca Dockerfile improvisado com imagem gigante.
- Contracts de API em `shared/` (OpenAPI + proto/ para gRPC); o contrato é a verdade, não a implementação.
- ORM (Hibernate/JPA/MyBatis) com migrations versionadas e DI por interfaces — o domínio depende da abstração, não do driver.
- Microservices apenas quando o domínio exige: service discovery + flow control (Eureka/Sentinel); outbox para consistência eventual ao cruzar serviços.
- Observabilidade vendor-neutral via Micrometer/OTel; nunca trap de vendor no instrumentação.
- Testes de base a topo: JUnit + BDD + ArchUnit; a arquitetura entra no pipeline de teste como regra de primeiro nível.

## ANTI-PATTERNS
`float/double para dinheiro` · `anemic model + services vazios` · `secrets no application.yml ou no código` · `migrations na mão (flyway_bypass)` · `controller gordo com regra de negócio` · `virtual threads como bala de prata p/ CPU-bound` · `saga implementada na mão em vez de Temporal/Conductor` · `DTO anêmico repetindo entidade 1:1` · `job de worker rodando dentro do app da API` · `pom.xml/build.gradle sem version catalog (BOM duplicado)`

## CHECKLIST
- [ ] Bounded context expresso em pacotes e vigiado por ArchUnit
- [ ] Money representado por `BigDecimal` em todas as entradas de valor monetário
- [ ] `@ConfigurationProperties` tipado; nenhum segredo no código; Jasypt para props cifradas
- [ ] Migrations versionadas (Flyway/Liquibase) na `infrastructure/db`
- [ ] Imagem gerada por Jib; build sem daemon Docker
- [ ] Contracts OpenAPI/proto em `shared/`
- [ ] Worker Kafka + job-scheduling (Temporal/Conductor) separados da API
- [ ] Outbox pattern em operações que cruzam serviços
- [ ] Observabilidade Micrometer/OTel vendor-neutral
- [ ] Testes JUnit + BDD + ArchUnit de base a topo no CI

## A REGRA
Java no padrão Cosca é monorepo com fronteiras vigiadas por teste, virtual threads quando I/O-bound, dinheiro só como BigDecimal e nada de segredo no código — a arquitetura é imposta no CI, não em reunião.

## REFERÊNCIAS
Spring Boot 3 docs (docs.spring.io) · JEP 444 (virtual threads) · ArchUnit docs · Temporal/Conductor · Flyway/Liquibase · Jib (Google) · Micrometer/OTel · Jasypt · OpenAPI · PADRAO-COSCA §6 · backend/README.md

## CONHECIMENTO (schema _SCHEMA.md)

```
ID: backend-005
DOMAIN: backend
TITLE: Arquitetura imposta por teste com ArchUnit
PROBLEM: Arquitetura degrada silenciosamente; pacotes viram bagunça e ninguém percebe até a refatoração doer
CONTEXT: Backend Java com bounded context em pacotes
PRINCIPLE: A arquitetura é uma regra executável: se não compila num teste, não entra no main
RECOMMENDATION: ArchUnit no pipeline de teste checando dependências entre camadas (controller→service→repository), imports de pacote proibido e formato de classes
WHEN_TO_USE: Qualquer monorepo Java/Spring Boot com mais de um bounded context
WHEN_NOT_TO_USE: Lambda/micro-serviço de um único handler onde fronteira não existe
TRADE_OFFS: Testes que falham em refactor cosmético vs integridade arquitetural garantida no CI
EXAMPLE: Regra ArchUnit: `classes in controller` não dependem de `repository`; services não referenciam framework
COUNTER_EXAMPLE: "Não precisamos de ArchUnit, a equipe se lembra da regra" — e a regra quebra no primeiro prazo apertado
FAILURE_MODES: Regra demasiado rígida (checa classes de teste, reflection) gerando falso-positivo e ignora geral
REFERENCES: ArchUnit user guide
CONFIDENCE: STRONG
SOURCE: awesome-java + PADRAO-COSCA §6
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: backend-006
DOMAIN: backend
TITLE: Outbox pattern para consistência eventual
PROBLEM: Publicar evento no Kafka e escrever no banco não é atômico; falha no meio gera estado divergente
CONTEXT: Operação mutável que precisa cruzar serviço via mensageria
PRINCIPLE: A verdade está no banco; o evento é derivado dela — grava-se na mesma transação e entrega-se depois
RECOMMENDATION: Escrever o evento numa tabela outbox dentro da transação da entidade; um relay/publicador entrega ao Kafka com retry e idempotência
WHEN_TO_USE: Operação mutável que publica evento que outro serviço consome
WHEN_NOT_TO_USE: Comunicação síncrona request/response sem necessidade de entrega garantida
TRADE_OFFS: Uma tabela + relay a mais vs consistência eventual sem dupla escrita
EXAMPLE: Pedido criado → transação grava pedido + `outbox` row → relay envia ao Kafka → consumidor deduplica por id
COUNTER_EXAMPLE: `save(pedido)` depois `kafka.send(...)` fora da transação; se o processo morre no meio, perdeu o evento
FAILURE_MODES: Relay sem idempotência entregando o mesmo evento duas vezes; outbox crescendo sem purge
REFERENCES: Microservices.io outbox pattern · Debezium outbox
CONFIDENCE: UNIVERSAL
SOURCE: awesome-java + PADRAO-COSCA §6
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: backend-007
DOMAIN: backend
TITLE: Virtual threads para concorrência I/O-bound (Java 21)
PROBLEM: Thread-per-request pré-Loom limita concorrência ao tamanho do pool de threads do SO
CONTEXT: Servidor web Java 21 (Spring Boot 3, Tomcat/Loom)
PRINCIPLE: Virtual threads são milhões de tasks mapeadas a poucas threads de plataforma; bloqueio vira custo zero
RECOMMENDATION: Habilitar virtual threads para payload I/O-bound (DB, HTTP, file); manter thread pools reais para tarefas CPU-bound e código síncrono legado (pinning)
WHEN_TO_USE: Web/worker com muito I/O e pouca CPU
WHEN_NOT_TO_USE: CPU-bound intensivo (criptografia, parse pesado) onde virtual threads não ganham nada
TRADE_OFFS: Simplicidade enorme de concorrência vs pinning em código síncrono nativo e bloqueio de lock de monitor
EXAMPLE: `spring.threads.virtual.enabled=true` — o servidor escala para milhares de requests sem pools gigantes
COUNTER_EXAMPLE: Sem Loom: pool de 200 threads e fila de espera com 90% ocioso aguardando DB
FAILURE_MODES: `synchronized`/locks de monitor pinando a carrier thread e neutralizando o ganho
REFERENCES: JEP 444 (Java 21 virtual threads) · Spring Boot Loom docs
CONFIDENCE: STRONG
SOURCE: awesome-java + JEP 444
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: backend-008
DOMAIN: backend
TITLE: Dinheiro é BigDecimal, nunca ponto flutuante
PROBLEM: double/float perdem exatidão em somas e comparam igualdades quebradas — dinheiro errado é confiança perdida
CONTEXT: Qualquer operação com valor monetário no backend
PRINCIPLE: Valor monetário é decimal exato; o tipo de ponto flutuante binário não representa centavos com fidelidade
RECOMMENDATION: `BigDecimal` para representar e operar dinheiro; definir scale e `RoundingMode` explicitamente no ponto de decisão (fatura, split, imposto)
WHEN_TO_USE: Preços, saldos, valores de fatura, taxas
WHEN_NOT_TO_USE: Contagem de itens, índices, quantidades inteiras — int/long bastam
TRADE_OFFS: Verbosidade e desempenho menor vs exatidão e auditabilidade
EXAMPLE: `new BigDecimal("19.90")` + `setScale(2, RoundingMode.HALF_UP)` para valor com centavos
COUNTER_EXAMPLE: `double total = 0.1 + 0.2;` — total é `0.30000000000000004`, e a fatura sai errada
FAILURE_MODES: `BigDecimal(double)` no construtor reintroduzindo a imprecisão; arredondamento implícito por localização
REFERENCES: Java BigDecimal docs · JEP 306 (java.math)
CONFIDENCE: UNIVERSAL
SOURCE: awesome-java + PADRAO-COSCA §6
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: backend-009
DOMAIN: backend
TITLE: Config tipada e segredos fora do código
PROBLEM: Segredo no application.yml ou no source vaza no git; config não tipada vira string mágica espalhada
CONTEXT: Spring Boot application config
PRINCIPLE: Config é contrato tipado e imutável; segredo é injetado do ambiente, nunca versionado
RECOMMENDATION: `@ConfigurationProperties` com POJO validado (ex.: `app.database.*`); segredos via env/vault; props cifradas com Jasypt para o que não é segredo mas não pode ser legível
WHEN_TO_USE: Toda configuração de serviço Spring Boot
WHEN_NOT_TO_USE: Flags locais de sessão/request — isso é estado, não config
TRADE_OFFS: Um POJO de config por módulo vs propriedades espalhadas em `@Value` ilegíveis
EXAMPLE: `@ConfigurationProperties("app.db") record DbProps(String url, String user) {}` + `@EnableConfigurationProperties`
COUNTER_EXAMPLE: `spring.datasource.password=admin123` versionado no application.yml
FAILURE_MODES: Rotação de segredo exige rebuild porque o secret foi embutido no build; Jasypt com chave no próprio código (nada cifrado)
REFERENCES: Spring Boot configuration docs · Jasypt Spring Boot
CONFIDENCE: UNIVERSAL
SOURCE: awesome-java + PADRAO-COSCA §6
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```
