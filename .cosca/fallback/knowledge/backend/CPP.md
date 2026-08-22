# 04 — C++ INTELLIGENCE

> Stack 04 da Cosca Engineering Intelligence Matrix.
> Doutrina C++ no padrão Cosca: RAII como regra-mãe, Guidelines apoiadas em perfis, ponteiros gerenciados por default.

## MISSÃO
Construir C++ moderno no padrão Cosca: CMake + Conan, compilação estrita (`-Wall -Wconversion`) + clang-tidy no CI, seguindo as CppCoreGuidelines — RAII como regra-mãe, ownership explícita via smart pointers/GSL, e os perfis de segurança como gates de qualidade.

## PRINCÍPIOS CORE
1. **RAII como regra-mãe** — recurso adquirido é liberado pelo destructor; nunca `new`/`delete` crus (R.1, E.6) · UNIVERSAL
2. **Ownership explícita** — `unique_ptr` por default, `shared_ptr` só via `make_shared` + `weak_ptr` para ciclos, raw pointers apenas não-owning (R.3) · UNIVERSAL
3. **GSL no lugar de ponteiros crus** — `span`, `not_null`, `owner` eliminam aritmética e invariantes implícitas de ponteiro · STRONG
4. **Perfis Type/Bounds/Lifetime como gates** — reinterpret_cast veta; UB e violações de lifetime não passam no review nem no CI · STRONG
5. **Const-correctness e erros por exceção tipada** — imutável por default, métodos const, `constexpr` (Con.1-3); exceções tipadas com invariante no ctor e `noexcept` no nunca-throw · UNIVERSAL

## REGRAS DE DECISÃO
- `CMake` + `Conan` como cadeia de build/deps; `-Wall -Wconversion` + `clang-tidy` rodando no CI (fail-closed).
- `unique_ptr` é o default; `shared_ptr` só com `make_shared` e ciclo quebrado por `weak_ptr`.
- Raw pointer no código-fonte só para não-owning (parâmetro de observação, regra R.3); demais usos de `span`/`not_null`/`owner`.
- Perfis Type, Bounds e Lifetime ativos: nenhum `reinterpret_cast`, nenhum index fora de `span`, nenhum uso após free.
- Erro por exceção tipada: lançar por valor, capturar por referência const; invariante garantida no constructor; `noexcept` nos destrutores e movers.
- Dependências em DAG sem ciclos (A.4); código estável separado do instável (A.1); Pimpl para ABI estável (I.27).
- Compile-time sobre run-time sempre que custo-zero (P.5): `constexpr`, templates, `if constexpr`.

## ANTI-PATTERNS
`new`/`delete` crus e `[]` manual` · `shared_ptr` em todo lugar para não pensar em ownership` · `get()` e cópia de ponteiro cru saindo da fronteira RAII` · `reinterpret_cast` para "ganhar tempo"` · `método que deveria ser const sem const` · `raw pointer owning por engano (R.3 violado)` · `memcpy/aritmética de ponteiro em vez de span` · `uso após free que o Lifetime profile pegaria` · `dependência circular entre libs`

## CHECKLIST
- [ ] `-Wall -Wconversion` sem warnings; `clang-tidy` limpo no CI
- [ ] Zero `new`/`delete` crus em código de produção
- [ ] `unique_ptr` default; `shared_ptr` só via `make_shared`; ciclos com `weak_ptr`
- [ ] Raw pointers 100% não-owning; fronteiras de ownership documentadas
- [ ] GSL `span`/`not_null`/`owner` onde ponteiro cru apareceria
- [ ] Perfis Type/Bounds/Lifetime ativos; zero `reinterpret_cast`
- [ ] Const-correctness: imutável por default, métodos const, `constexpr` onde possível
- [ ] Exceções tipadas; invariantes no ctor; `noexcept` em nunca-throw
- [ ] Grafo de libs em DAG; Pimpl para superfícies de ABI estável

## A REGRA
C++ seguro é C++ que não deixa o risco morar no código: RAII garante o recurso, os smart pointers declaram a ownership, o GSL elimina o ponteiro cru, e os perfis Type/Bounds/Lifetime travam a UB no portão — disciplina que o compilador e o clang-tidy impõem, não só a boa vontade.

## REFERÊNCIAS
isocpp/CppCoreGuidelines (github.com/isocpp/CppCoreGuidelines) · Microsoft GSL (gsl-lite/Microsoft-GSL) · CppCoreGuidelines.md §R, §Con, §E · PADRAO-COSCA §6 · awesome-cpp (listas de build: CMake/Conan) · clang-tidy (cppcoreguidelines-*) · backend/README.md

## CONHECIMENTO (schema _SCHEMA.md)

```
ID: backend-015
DOMAIN: backend
TITLE: RAII como regra-mãe de gestão de recurso
PROBLEM: Recurso (memória, fd, lock, conexão) vaza ou é liberado duas vezes quando o erro interrompe o fluxo
CONTEXT: C++ moderno, qualquer recurso com acquire/release
PRINCIPLE: A aquisição é a inicialização; o destructor é o release — a linguagem garante liberação em qualquer saída de escopo
RECOMMENDATION: Toda posse de recurso num objeto RAII; nunca `new`/`delete` crus (R.1, E.6); dtor nunca lança
WHEN_TO_USE: Qualquer alocação, handle, lock, stream ou conexão com posse
WHEN_NOT_TO_USE: Observação não-owning (dai usa ponteiro não-owning ou span, sem RAII próprio)
TRADE_OFFS: Regras de lifetime herdadas do escopo vs liberação automática à prova de erros
EXAMPLE: `std::unique_ptr<Foo>` / `std::lock_guard` / `std::ifstream` — release automático no fim do escopo
COUNTER_EXAMPLE: `Foo* f = new Foo();` seguido de `delete` espalhado em cada caminho de retorno
FAILURE_MODES: Destructor que lança durante unwinding chama std::terminate — dtor nunca-throw
REFERENCES: CppCoreGuidelines R.1, E.6; C.35
CONFIDENCE: UNIVERSAL
SOURCE: isocpp/CppCoreGuidelines
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: backend-016
DOMAIN: backend
TITLE: GSL span no lugar de ponteiro cru + aritmética
PROBLEM: Ponteiro cru com tamanho implícito abre portas para out-of-bounds, UB e APIs que não dizem o tamanho
CONTEXT: APIs de biblioteca, buffers, fatias de arrays/contêineres
PRINCIPLE: Uma visão contígua de dados carrega o tamanho junto — o próprio tipo elimina a aritmética de ponteiro
RECOMMENDATION: Usar `gsl::span` para parâmetros/retornos que veem sequências contíguas; iterar por range-for ou índice verificado
WHEN_TO_USE: Função que recebe/produz fatias de `std::vector`, `std::array`, C-array, string buffers
WHEN_NOT_TO_USE: Posse (ownership) — span não-owning; para dados não-contíguos use abstração própria
TRADE_OFFS: Dependência do GSL e índices que podem lançar vs eliminação de UB por aritmética
EXAMPLE: `void process(gsl::span<const std::byte> bytes)` — chamador passa `std::vector` direto, tamanho embutido
COUNTER_EXAMPLE: `void process(const std::byte* p, size_t n)` com `p[i]` sem verificação e quem chama acerta o n
FAILURE_MODES: Guardar span além da vida do container subjacente (dangling view) — lifetime do escopo dono
REFERENCES: CppCoreGuidelines R.3, F.60; Microsoft GSL
CONFIDENCE: STRONG
SOURCE: isocpp/CppCoreGuidelines + Microsoft GSL
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: backend-017
DOMAIN: backend
TITLE: Perfis Type, Bounds e Lifetime como gates de segurança
PROBLEM: UB de casts, estouro de índice e uso-após-free passam silenciosamente em builds de otimização
CONTEXT: C++ de produção; review e CI
PRINCIPLE: Categorias de segurança centrais viram regras mecânicas, verificáveis por ferramenta — não por disciplina
RECOMMENDATION: Ativar os perfis Type/Bounds/Lifetime (clang-tidy cppcoreguidelines-*) como gate de CI; veta reinterpret_cast
WHEN_TO_USE: Todo código C++ novo; revisões de código existente em hotspots de segurança
WHEN_NOT_TO_USE: Legado não-migrável roda num gate à parte — mas nunca "silencioso" (fail-closed)
TRADE_OFFS: Falsos positivos e refactor obrigatório vs eliminação de UB na porta de entrada
EXAMPLE: Uso de `span.at(i)`/índices verificados e lifetime check que aponta dtor em escopo errado antes do merge
COUNTER_EXAMPLE: `reinterpret_cast<Foo*>(raw)` aprovado em review porque "o layout é conhecido"
FAILURE_MODES: Perfil desligado por dificuldade técnica deixa a classe inteira de UB voltar a passar
REFERENCES: CppCoreGuidelines "Profiles"; clang-tidy cppcoreguidelines-*
CONFIDENCE: STRONG
SOURCE: isocpp/CppCoreGuidelines Profiles + clang-tidy
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: backend-018
DOMAIN: backend
TITLE: Const-correctness como contrato de imutabilidade
PROBLEM: Estado que deveria ser imutável muda por efeito colateral; otimizações e raciocínio concorrente ficam impossíveis
CONTEXT: APIs de biblioteca, DTOs, dados compartilhados em threads
PRINCIPLE: Se não muda, declara const: variáveis imutáveis por default, métodos que não mutam são const, o que dá para computar em compile-time é constexpr
RECOMMENDATION: Marcar tudo const por default (Con.1-3); só desconst quando há motivo real; const-correctness faz a assinatura dizer a verdade
WHEN_TO_USE: Assinatura de métodos, parâmetros de leitura, membros que nunca mudam, constantes de compile-time
WHEN_NOT_TO_USE: Mutators legítimos; imutabilidade de objetos compartilhados pede const, não const_cast (que é cheiro de mau design)
TRADE_OFFS: Mais anotações e propagação de const nas chamadas vs contratos claros e thread-safety por leitura
EXAMPLE: `constexpr double pi = 3.14159;` e `int size() const noexcept;` — chamadores sabem que não muta
COUNTER_EXAMPLE: Getter não-const obrigando toda API cliente a segurar estado mutável para apenas ler
FAILURE_MODES: `mutable` e `const_cast` vazando imutabilidade — invariantes que dependiam do const deixam de valer
REFERENCES: CppCoreGuidelines Con.1, Con.2, Con.3
CONFIDENCE: UNIVERSAL
SOURCE: isocpp/CppCoreGuidelines
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: backend-019
DOMAIN: backend
TITLE: Pimpl para ABI estável
PROBLEM: Mudança em membro privado (campo, include) quebra compilação e ABI de consumidores da biblioteca
CONTEXT: Bibliotecas compartilhadas com ABI pública estável
PRINCIPLE: A parte que muda fica atrás de um ponteiro opaco; a superfície pública não recompila o consumidor
RECOMMENDATION: Classe pública guarda `std::unique_ptr<Impl>` (pimpl), dtor declarado e definido fora, movers implementados; nunca raw `delete`
WHEN_TO_USE: Bibliotecas .so/.dll com ABI longa; headers públicos de terceiros
WHEN_NOT_TO_USE: Header-only/templates; classes pequenas de hot path onde a indireção custa caro
TRADE_OFFS: Indireção e alocação extra vs ABI estável e tempo de compilação menor para consumidores
EXAMPLE: `class Widget { ... private: struct Impl; std::unique_ptr<Impl> m_impl; };` com implementação em .cpp
COUNTER_EXAMPLE: Todos os membros privados expostos no header; adicionar um campo recompila e quebra a .so
FAILURE_MODES: Dtor inline com Impl incompleto e `unique_ptr` que instancia deleção — declara dtor no .cpp
REFERENCES: CppCoreGuidelines I.27; A.1, A.4 (separação estável/instável, DAG)
CONFIDENCE: STRONG
SOURCE: isocpp/CppCoreGuidelines
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```
