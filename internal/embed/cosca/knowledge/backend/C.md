# 14 — C INTELLIGENCE

> Stack 14 da Cosca Engineering Intelligence Matrix.
> Doutrina da língua de fronteira da família: single-purpose libs com interface C estável — o "latim" que todas as outras línguas chamam por FFI.

## MISSÃO
Escrever C no padrão Cosca como a camada de fronteira/ABI da família: bibliotecas single-purpose com interface C estável, build Meson/CMake com fallback Autotools, embedded/RTOS e allocators customizados onde a latência manda, portabilidade como religião e comportamento estritamente definido (C11/C17, sem UB).

## PRINCÍPIOS CORE
1. **Interface C estável como ABI universal** — o header público é o contrato; quem muda a ABI quebra o mundo (CPL.3) · UNIVERSAL
2. **Single-purpose libs** — cada lib faz uma coisa; a fronteira C é pequena, documentada e imutável na prática · STRONG
3. **Portabilidade como religião** — C11/C17 com `-Wall -Wextra -Wpedantic`, sem UB (Lifetime.1); se compilar C++, compilar como C++ também (CPL.1-3) · UNIVERSAL
4. **Memória explícita** — sem heap onde possível; allocators customizados (rpmalloc/jemalloc) quando a latência é o requisito · STRONG
5. **Concorrência com shape** — structured concurrency estilo Go em C (libmill/libdill) ou OpenMP/MPI para HPC; nunca threads soltas e mutexes espalhados · CONTEXTUAL

## REGRAS DE DECISÃO
- Libs do Cosca com núcleo crítico (hash, cripto, serialização) são candidatas a C: interface estável e consumível de Go/Rust/C++/C# por FFI.
- Build: Meson ou CMake como default; Autotools só como fallback para portabilidade histórica.
- Embedded/MCU: Zephyr ou FreeRTOS, `static` em tudo, sem heap onde o domínio permitir.
- Desktop/server com exigência de latência: rpmalloc/jemalloc por lib, nunca `malloc` desnudo no hot path.
- Subconjunto comum: escrever código que compila como C e como C++ (CPL.1-3) para ampliar o campo de consumo.
- Macros somente para generic code (modelo klib); toda macro precisa de teste; macro que esconde fluxo é crime.
- Falha de ABI é fail-closed: versionar o header, nunca silenciar `-Wall -Wextra -Wpedantic`, validar struct layouts em teste.

## ANTI-PATTERNS
`interface C que muda a cada release` · `lib monolítica com 200 funções exportadas` · `UB escondido (cast de ponteiro, out-of-bounds, uninitialized)` · `heap desnudo no hot path` · `macros que escondem lógica ou não são testadas` · `threads soltas com mutexes espalhados` · `build só com Autotools herdado sem upgrade` · `C "pensado como C++" com over-engineering (templates manuais, hierarquia fictícia)` · `ignorar warnings do compilador`

## CHECKLIST
- [ ] Header público pequeno, estável e versionado; sem breaking change sem bump de ABI
- [ ] Build Meson/CMake reproduzível; Autotools como fallback documentado
- [ ] Compila com `-Wall -Wextra -Wpedantic` sem warnings; sem UB conhecido (Lifetime.1)
- [ ] Compila como C++ quando o subconjunto comum se aplica (CPL.1-3)
- [ ] `static` como default; heap só onde justificado e via allocator explícito
- [ ] Macros restritas a generic code (klib) e cobertas por teste
- [ ] Concorrência com structured concurrency ou OpenMP/MPI; sem threads soltas
- [ ] Teste de ABI/layout (struct sizes/offsets) para detectar quebra silenciosa
- [ ] FFI de exemplo em pelo menos uma outra língua (Go/Rust/C++/C#) no repo

## A REGRA
C é o latim da família: as outras línguas chamam, o header é o contrato, e quem viola a ABI ou esconde UB trai a confiança do Dom — single-purpose, estável e definido, ou não é C no padrão Cosca.

## REFERÊNCIAS
CppCoreGuidelines CPL (C language) · CppCoreGuidelines Lifetime · awesome-c (aleksandar-todorovic) · Meson · CMake · Autotools · Zephyr · FreeRTOS · libmill · libdill · OpenMP · MPI · klib · rpmalloc · jemalloc · PADRAO-COSCA §6 · backend/README.md

## CONHECIMENTO (schema _SCHEMA.md)

```
ID: backend-025
DOMAIN: backend
TITLE: Interface C estável como ABI universal de fronteira
PROBLEM: Cada língua reimplementa o núcleo crítico e diverge; a fronteira muda e quebra todos os consumidores
CONTEXT: Núcleos críticos (hash, cripto, serialização) consumidos por Go/Rust/C++/C# via FFI
PRINCIPLE: O header público é o contrato; uma ABI estável é o "latim" que todas as línguas falam
RECOMMENDATION: Expor um header pequeno e versionado; congelar ABI; change de breaking = bump de versão + plano de migração
WHEN_TO_USE: Núcleo crítico multi-consumidor onde a consistência entre línguas vale mais que o conforto de cada uma
WHEN_NOT_TO_USE: Feature puramente interna a uma língua — C é fronteira, não o corpo inteiro
TRADE_OFFS: Custo de manter C+FFI vs consistência e uma única verdade do núcleo
EXAMPLE: lib que expõe `cosca_hash_ctx` + 5 funções puras, consumida via cgo/cxxbridge
COUNTER_EXAMPLE: Reimplementar SHA-256 em Go, Rust, C# e C++ com quatro bugfix streams
FAILURE_MODES: ABI quebrada silenciosamente (mudou struct, não mudou versão) corrompendo consumidores
REFERENCES: CppCoreGuidelines CPL.3 · PADRAO-COSCA §6
CONFIDENCE: UNIVERSAL
SOURCE: aleksandar-todorovic/awesome-c + CppCoreGuidelines CPL
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: backend-026
DOMAIN: backend
TITLE: Embedded/RTOS estático sem heap onde possível
PROBLEM: MCU sem heap previsível: alocação dinâmica fragmenta, diverge timing e trava em latência
CONTEXT: Zephyr/FreeRTOS em microcontroladores, domínios com restrição de memória
PRINCIPLE: Em embedded, memória é determinística ou falha; `static` e alocação em tempo de compilação são a regra
RECOMMENDATION: Variáveis e buffers estáticos, tarefas com stack fixa; heap só com pool/allocator dedicado e auditado
WHEN_TO_USE: MCU, RTOS, domínios hard-real-time
WHEN_NOT_TO_USE: Server/desktop com RAM abundante — `static` gigante aí é desperdício de memória virtual
TRADE_OFFS: Flexibilidade perdida vs determinismo e previsibilidade de latência
EXAMPLE: Buffer de telemetria `static uint8_t buf[512]` compartilhado por tarefa única, sem malloc
COUNTER_EXAMPLE: `malloc` por pacote em tarefa de RTOS de alta frequência fragmentando o heap
FAILURE_MODES: Buffer estático compartilhado entre tarefas sem sincronização = race e corrupção
REFERENCES: Zephyr · FreeRTOS · awesome-c (embedded)
CONFIDENCE: STRONG
SOURCE: aleksandar-todorovic/awesome-c + CppCoreGuidelines
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: backend-027
DOMAIN: backend
TITLE: Allocators customizados para latência
PROBLEM: `malloc` desnudo no hot path custa syscall/lock e a latência viola o SLO
CONTEXT: Server/desktop com p95 de latência como requisito (cache, proxies, parsers)
PRINCIPLE: O allocator é parte do design de latência; o heap padrão é o fallback, não o default
RECOMMENDATION: rpmalloc/jemalloc por arena/thread; medir antes e depois com benchmark de p95
WHEN_TO_USE: Hot path com alocação frequente e requisito de latência medido
WHEN_NOT_TO_USE: Código de inicialização/cold path onde o ganho não aparece no p95
TRADE_OFFS: Complexidade de arena/lifetime vs latência e cache locality
EXAMPLE: jemalloc por thread local no parser de pacotes; p95 cai e o benchmark prova
COUNTER_EXAMPLE: Trocar malloc por jemalloc "porque sim" sem benchmark e sem entender o lifetime
FAILURE_MODES: Arena liberada enquanto ponteiros ainda em uso = use-after-free silencioso
REFERENCES: rpmalloc · jemalloc · awesome-c (allocators)
CONFIDENCE: STRONG
SOURCE: aleksandar-todorovic/awesome-c + CppCoreGuidelines
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: backend-028
DOMAIN: backend
TITLE: Structured concurrency estilo Go em C
PROBLEM: Threads soltas com mutexes espalhados criam deadlocks, races e shutdown impossível
CONTEXT: Concorrência em C de médio porte, onde goroutines seriam a resposta natural
PRINCIPLE: A concorrência tem shape e escopo; canais/goroutines (libmill/libdill) dão structured concurrency sem o GC
RECOMMENDATION: libmill/libdill para corrotinas/canais com escopo; OpenMP/MPI para HPC massivo
WHEN_TO_USE: I/O concorrente em C (servers, brokers) ou HPC com paralelismo de dados
WHEN_NOT_TO_USE: Concorrência trivial de 2-3 threads — o overhead do scheduler não compensa
TRADE_OFFS: Scheduler próprio vs controle fino de thread/mutex à mão
EXAMPLE: Servidor em libdill: um `go` por conexão, canais para coordenar, shutdown por cancelação de escopo
COUNTER_EXAMPLE: `pthread` + `mutex` espalhado por 10 arquivos com lock order nunca documentado
FAILURE_MODES: Corrotina que bloqueia no meio de uma syscall trava o scheduler inteiro
REFERENCES: libmill · libdill · OpenMP · MPI
CONFIDENCE: CONTEXTUAL
SOURCE: aleksandar-todorovic/awesome-c
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: backend-029
DOMAIN: backend
TITLE: Portabilidade como religião e comportamento definido
PROBLEM: C que "funciona" na máquina do autor explode em outro compilador/arch — UB, tamanho de tipo, endian
CONTEXT: C11/C17 multi-plataforma, kernel e libs da família
PRINCIPLE: Comportamento definido é o único contrato portável; warnings são erros e UB é proibido (Lifetime.1)
RECOMMENDATION: `-Wall -Wextra -Wpedantic` como baseline, sanitizers no CI, subconjunto comum compilável como C++ (CPL.1-3)
WHEN_TO_USE: Toda lib C da casa; obrigatório no núcleo consumido por FFI
WHEN_NOT_TO_USE: Não há — código C fora desse regime é debt, não feature
TRADE_OFFS: Tempo de conformidade/porta vs sobrevivência em outra plataforma sem diagnóstico
EXAMPLE: CI roda GCC+Clang com UBSan/ASan e cross para arm64; structs com ints de largura fixa
COUNTER_EXAMPLE: `int` para tamanho assumindo 32-bit e cast de ponteiro para alinhar "do jeito que sempre fiz"
FAILURE_MODES: UB otimizado pelo compilador (UB pode ser usado para otimizar "de forma surpreendente")
REFERENCES: CppCoreGuidelines CPL.1-3, Lifetime.1 · awesome-c (portability)
CONFIDENCE: UNIVERSAL
SOURCE: aleksandar-todorovic/awesome-c + CppCoreGuidelines CPL
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```
