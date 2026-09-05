# ADR-026: Seleção do modelo LLM para o agente Hermes+Ollama (RTX 5060 Ti 16GB / 32GB DDR5)

> **Status:** Proposed (aguardando aprovação do Don) | **Owner:** cosca-architecture | **Last Updated:** 2026-08-29
> **Revisão:** proposta para o Don. **Design aditivo — não quebra o kernel.**
> **Nota de honestidade:** a máquina onde esta ADR foi escrita mede outra config (AMD/ROCm, RX 6700 XT,
> conforme PROJECT_CONTEXT). Os números de VRAM/quantização aqui são **estimativas fundamentadas** de
> arquitetura de GGUF + memoria, não medições locais; os marcadores `⚠️` indicam o que é estimativa e o que
> precisa de verificação empírica no hardware-alvo antes de Validar.

---

## 0. Contexto — a dúvida

O Don perguntou: **"qual o melhor modelo LLM para rodar o Hermes+Ollama autonomamente nesta config?"**

Config-alvo medida/declarada:
- **GPU:** NVIDIA RTX 5060 Ti **16GB** VRAM (Blackwell, CUDA 12.8+, Tensor Cores). ⚠️ largura de banda
  GDDR7 ~448 GB/s (bus 128-bit) — é o que limita tokens/s em modelos densos.
- **RAM:** 32GB DDR5-3600 dual channel (⚠️ ~57–60 GB/s).
- **Stack:** Ollama 0.33.1 (medido) + framework **Hermes** (agente/orquestrador local). Ollama com modelo
  atual **Qwen3-Coder-30B** a ~64k de contexto.
- **OS:** Windows 11 (≠ Linux — relevante p/ backend do Ollama: CUDA em vez de ROCm/Vulkan; CUDA 12.8+
  exigido por Blackwell).
- **Persistência atual local:** modelos já baixados: `qwen2.5-coder`, `qwen2.5vl:7b`, `llava`, `bge-m3`,
  `nomic-embed-text` (medido).

**Objetivo do agente:** (1) executar ferramentas no Windows 11; (2) **sem alucinar** (tool-calling grounded,
verificação determinística); (3) mínima interação humana; (4) o mais autônomo possível, buscando conhecimento
na internet quando não souber.

A dúvida real, destilada: **qual modelo, em que quantização, aguenta o trade-off qualidade × VRAM × contexto
× velocidade sem virar "tortura de performance"?** E qual é o **ceiling realista de contexto** dado que 64k em
16GB de VRAM é, na minha avaliação, **impraticável** para um modelo ~30B.

---

## 1. Decisão

**Modelo primário: `qwen3-coder:30b-a3b-instruct`** (MoE, ~30.5B total / ~3.3B ativos), na quantização
**`Q4_K_M`**, com **cache KV quantizado (Q8_0)** e **ceiling de contexto realista ≈ 32k** (confortável
**16k–24k**). Se o Don valorizar raciocínio geral + flexibilidade de tool-calling mais que especialização em
código, o drop-in de mesma pegada é **`qwen3:30b-a3b-instruct`** (mesma geometria de GGUF/KV — a orientação
operacional abaixo permanece idêntica).

**Por que esse, em uma frase:** é o único candidato que junta (a) **tool-calling nativo** de classe-líder em
GGUF, (b) **melhor raciocínio de código por FLOP** (MoE 3B ativos → rápido na 5060 Ti, essencial p/ autonomia),
e (c) pegada que cabe na 16GB **com offload parcial** para os 32GB de RAM — aceitando um `Q3_K_M` (~14GB) como
perfil "zero-offload" se o offload degradar demais.

**Decisões derivadas (esclarecem o escopo da ADR):**
1. **64k NÃO é praticável** em 16GB com um ~30B. Proponho ceiling de ~32k com KV quant; recomendo
   operar em **16k–24k** como ponto de conforto (justificativa matemática na §2). O ceiling real de 64k
   só é viável com um modelo **dense ~14B** (ex: Phi-4, a custo de tool-calling fraco e ctx de 16k do próprio
   modelo) ou **MoE ~16B** (ex: DeepSeek-Coder-V2-Lite, a custo de tool-calling/qualidade).
2. **Quantização:** `Q4_K_M` é o teto de qualidade que ainda entra; `Q3_K_M` é o plano-B "100% GPU, sem offload";
   `Q2_K` só se necessidade extrema (perda de qualidade perceptível em tool-calling).
3. **Backend Ollama no Windows:** CUDA (padrão) com CUDA 12.8+ (é o que a Blackwell exige). Vulkan é o plano-B
   se houver problema de driver/hash em CUDA.
4. **Anti-alucinação:** o modelo NÃO é a linha de defesa. Ele é a "compreensão"; a **verificação é
   determinística em código** (schema JSON + normalização de parâmetros + execução da ferramenta + conferência
   do resultado). O modelo só entra com tool-calling **grounded** e **structured output** — nunca autoridade.

---

## 2. Racional

### 2.1 — A matemática da VRAM que decide o caso (⚠️ estimativa, validar no alvo)

Pegada do modelo em GGUF (estimativa por parâmetro, média de GGUF publicados):
- `Q4_K_M` ≈ 4.7–4.9 bits/param; `Q5_K_M` ≈ 5.7; `Q6_K` ≈ 6.6.
- **Qwen3-30B-A3B (`Q4_K_M`) ≈ 18.6GB** → **não cabe** em 16GB → exige offload de ~3GB para RAM.
- `Q3_K_M` ≈ 14GB → **cabe** em 16GB com folga para KV.
- `Q2_K` ≈ 11GB → cabe, mas qualidade cai.

KV cache para Qwen3-30B-A3B (48 layers, GQA 8 KV-heads, head_dim 128):
- **1 token ≈ 192 KB** em FP16; **≈ 96 KB** em Q8_0 (KV quantizado).
- **64k ctx:** FP16 ≈ **12.6GB**; Q8_0 ≈ **6.3GB**.
- **32k ctx:** FP16 ≈ **6.3GB**; Q8_0 ≈ **3.1GB**.
- **16k ctx:** FP16 ≈ **3.1GB**; Q8_0 ≈ **1.6GB**.

**Conclusão prática:**
- **64k é inviável.** Com FP16 (12.6GB KV) + weights (~18.6GB Q4) = ~31GB demandas sobre 16GB de VRAM. Mesmo
  com KV Q8_0, weights+KV ≈ 24GB > 16GB → quase todo o KV iria para RAM → o attention leria dezenas de GB via
  PCIe por passo de inferência → **colapso de throughput**, exatamente a "tortura de performance" que a ADR
  quer evitar. (⚠️ PCIe 4.0 x16 ≈ 32GB/s; ler 6.3GB de KV por token ≈ ~0.2 s/token **só de I/O**. Inviável.)
- **Sweet spot real:** `Q3_K_M` (~14GB) + 16k ctx Q8_0 KV (~1.6GB) ≈ **15.6GB → 100% GPU, zero offload**. É o
  perfil mais limpo de determinismo e velocidade.
- **Perfil Q4_K_M:** ~86% das layers na GPU (~15GB), ~3GB na CPU → blend de velocidade (~70–85% do teto,
  ⚠️ depende de como o Ollama reparte MoE entre GPU/CPU). Com 32GB de RAM, o offload é **aceitável**, mas é o
  maior risco de latência.

### 2.2 — Por que MoE 30B-A3B e não dense (~30B)

Um **dense 30B** (Qwen3-Coder-30B denso, Qwen2.5-Coder-32B, GLM-4-32B) **lê ~30B de pesos por token** →
memória-bandwidth-bound na 448 GB/s da 5060 Ti. Um **MoE A3B lê só ~3.3B ativos por token** → a 448 GB/s isso
é ~5–9x mais eficiente de banda. Em 16GB, o trade-off é cristalino: **dense 30B não cabe em Q4 e é lento;
MoE 30B-A3B não cabe "sozinho" em Q4 mas é rápido e cabe com offload, ou cabe 100% em Q3_K_M.**

É o coração da escolha: **velocidade (prioridade #6) só é alcançável na 16GB com um MoE de ~3B ativos.**

### 2.3 — Tool-calling / structured output (prioridade #1)

`Qwen3-Coder-30B-A3B-Instruct` tem **tool-calling nativo** no formato de chat (schema `tools` → `tool_call`
estruturado), treinado em dados de uso de ferramentas + código, e é um dos melhores em precisão de
function-calling na faixa local. Isso é o que permite o anti-alucinação da família: o **JSON do tool_call
é analisado e validado por schema determinístico; parâmetros alucinados são normalizados para a chave canônica
antes da validação** (padrão já implementado no caminho de tool-calls, ver learnings do cosca-backend). Se o
JSON vier malformado → **rejeita e re-tenta** (retry), não "adivinha".

### 2.4 — Contra-alucinação é arquitetura, não modelo

A escolha de modelo **não** é o mecanismo anti-alucinação — é um dos insumos. A linha de defesa real
(consistente com ADR-012/ADR-022 e a primitiva epistêmica `Observation`/OBSERVED/INFERRED/UNKNOWN):
1. **Structured output** (schema JSON imposto, temperature=0) — o modelo não fala livre, ele preenche um contrato.
2. **Verificação determinística em código** — o resultado da ferramenta é checado de forma mecânica; o modelo
   nunca decide sozinho.
3. **Normalização de parâmetros** — chaves alucinadas → canônicas, antes do schema.
4. **Grounded knowledge** — quando não sabe, o agente **busca na internet** e cita proveniência, em vez de
   "inventar" (postura do ADR-017: "incomplete, not evidence of absence").
5. **2 Zonas (ADR-012)** — a IA local **compreende**; o determinístico **decide**.

`Qwen3-Coder-30B-A3B` suporta 1 e 4 bem (structured output robusto + base de conhecimento ampla p/ busca
agêntica). Os pontos 2, 3, 5 são responsabilidade do Hermes/Cosca, não do modelo.

### 2.5 — Windows 11 (≠ Linux)

- Ollama no Windows+NVIDIA usa o backend **CUDA** (llama.cpp CUDA). **Blackwell exige CUDA 12.8+** e driver
  NVIDIA ≥ 570.x (Game Ready/Studio). Overhead de driver WDDM é maior que no Linux WSL; verificar split
  GPU/CPU em `ollama ps` pós-run.
- **Vulkan** é o plano-B (usar se CUDA der problema de mistura; no Windows, compilação Vulkan do llama.cpp é
  otimizada e pode ser boa p/ FP16), mas CUDA permanece o default mais rápido.
- GGUF/offload são idênticos em conceito; a única diferença é o backend e a necessidade de configurar a
  quantização do KV cache (⚠️ verificar chave exata de ativação no Ollama 0.33: `OLLAMA_CONTEXT_LENGTH`,
  `--cache-type-q8_0` no `Modelfile`/`PARAMETER`).

---

## 3. Alternativas consideradas

> Coluna "Veredito" já traz a decisão. `⚠️` = estimativa de dimensionamento (GGUF público + geometria do KV);
> confirmar no alvo antes de Validar.

| Modelo | Total (ativos) | Q4_K_M | Cabe 100% 16GB? | 64k viável? | Tool-calling | Código | Velocidade t/s (⚠️) | GGUF/Ollama | Veredito |
|---|---|---|---|---|---|---|---|---|---|
| **Qwen3-Coder-30B-A3B** (MoE) | 30.5B (3.3B) | ~18.6GB | ⚠️ Não (offload ~3GB) / Q3_K_M ~14GB sim | ❌ | ⭐ Excelente (nativo) | SOTA local | ⚠️ 25–40 | ✅ | **⭐ PRIMÁRIO** |
| Qwen3-30B-A3B-Instruct (MoE, geral) | 30.5B (3.3B) | ~18.6GB | ⚠️ Não / Q3 sim | ❌ | ⭐ Excelente + thinking toggle | Muito bom | ~mesmo | ✅ | ✅ drop-in (trocar se >raciocínio geral) |
| Qwen3-Coder-30B (denso) | 30.7B | ~18.6GB | ⚠️ Não | ❌ | Bom | Excelente (~= A3B) | ⚠️ 3–8 (dense) | ✅ | ❌ dense lento, não casa 16GB |
| Qwen2.5-Coder-32B | 32B dense | ~19GB | ❌ | ❌ | Bom | Muito bom | ⚠️ 3–8 | ✅ | ❌ dense, pior em tool vs Qwen3 |
| DeepSeek-Coder-V2-Lite (MoE) | 16B (2.4B) | ~9.3GB | ✅ Sim | ✅ (32-48k) | ⚠️ Médio/fraca | Bom | ⚠️ 30–50 | ✅ | ⚠️ fallback p/ ctx alto; perde em tool |
| DeepSeek-Coder-V2 | 236B (21B) | ~140GB | ❌ | ❌ | — | — | impossível | ⚠️ | ❌ inviável |
| GLM-4-32B (dense) | 32B | ~19GB | ❌ | ❌ | Bom | Muito bom | ⚠️ 3–8 | ✅ | ❌ dense |
| GLM-4.5-Air (MoE) | 106B (12B) | ~66GB | ❌ | ❌ | ⭐ Excelente | Excelente | impossível | ✅ | ❌ inviável em 16GB |
| Mistral-Small 3.2 (24B, dense) | 24B | ~14GB | ⚠️ Sim (sob ~2GB KV) | ⚠️ (KV limitado) | ⭐ Excelente (fn-call forte) | Bom | ⚠️ 10–20 | ✅ | ⚠️ alternativa SOLO tool forte + Q4, ctx limitado |
| Codestral 25.01 (22B) | 22B | ~13GB | ✅ sim | ⚠️ | ⚠️ Fraca/função-call limitada | ⭐ Excelente código | ⚠️ 12–22 | ✅ | ❌ tool-calling fraco |
| Devstral (24B) | 24B | ~14GB | ⚠️ sim | ⚠️ | ⭐ Forte (foco agêntico) | Muito bom | ⚠️ 10–20 | ✅ | ⚠️ alternativa agêntica |
| Phi-4 (14B, dense) | 14B | ~8.5GB | ✅ Sim (mta folga) | ❌ (modelo ctx 16k) | ⚠️ Nativo fraco | Bom | ⚠️ 25–40 | ✅ | ❌ tool fraco + ctx 16k |
| Phi-4-mini (3.8B) | 3.8B | ~2.4GB | ✅ | — | Fraca | Fraco | rápida | ✅ | ❌ |
| Llama-3.1/3.3 8B | 8B | ~5GB | ✅ | ✅ (128k) | Muito bom | Bom | rápida | ✅ | ✅ fallback rápido/leve |
| Llama-3.3 70B | 70B | ~40GB | ❌ | ❌ | — | — | impossível | ✅ | ❌ 70B inviável em 16GB |

---

## 4. Consequências

**Positivas:**
- **Autonomia real:** MoE 3B ativos → inferência rápida o suficiente p/ o loop agente tentar, verificar, iterar
  sem esperar minutos — a confiança p/ "mínima interação humana".
- **Tool-calling grounded:** JSON de tool_call validado por schema + verificação determinística → ataca a
  prioridade #1 de forma sustentável.
- **Um modelo cobre tudo:** código + tool-calling + raciocínio geral no mesmo GGUF (drop-in para o 30B geral
  sem trocar infra de KV).

**Negativas / trade-offs explícitos:**
- **64k é descartado.** O Don deve tratar "64k" como **não meta** para esta config. Ceiling realista ≈ 32k com
  KV Q8; conforto em 16k–24k. Para contextos além disso: **comprimir antes de expandir** (resumo de contexto,
  compactação — ver engine de compactação de contexto já existente) em vez de aumentar o ctx.
- **Offload no Q4:** ~3GB de pesos vão p/ RAM → latência marginal e blend de velocidade (~70–85% do teto). Se o
  Don exigir "tudo em GPU, sem offload", mudar para **Q3_K_M** (~14GB, 100% GPU) — com leve perda de qualidade.
- **Plano de memorização:** o offload de MoE entre GPU/CPU (granularidade de experts) é o maior risco técnico
  a medir — **estimar velocidade real antes de assumir**.
- **Backend:** depende de CUDA 12.8+ no Windows. Se o driver/backend não estiver OK, o plano-B é Vulkan; ambos
  precisam de `ollama ps` para confirmar split GPU/CPU.

**O que NÃO muda:** não altera o kernel, a lei do Cofre (ADR-012), nem o padrão anti-alucinação em código
(ADR-022 + ferramentas). A escolha é **additive**: só troca o modelo/quantização servida ao Hermes.

---

## 5. Teste de aceite

1. **Fit na VRAM:** carregar `qwen3-coder:30b-a3b-instruct` (`Q4_K_M`) + 16k ctx KV Q8 e confirmar via
   `ollama ps` que a maioria das layers/do KV está na GPU (e quantificar o offload em GB).
2. **Teto de contexto:** subir `num_ctx` progressivamente (8k→16k→32k→64k) e medir tokens/s; **registrar onde
   o throughput colapsa** (>50% de queda). Esperado: colapso entre 32k e 64k → confirma o ceiling de 32k.
3. **Structured output:** chamar uma tool com schema estrito via Hermes e validar que o `tool_call` parseia,
   normaliza parâmetros e passa no JSON Schema **sem** iteração de retry (temperatura 0).
4. **Tool-calling grounded:** um cenário que exige buscar na internet + executar ferramenta no Windows, e
   confirmar que o resultado é conferido deterministicamente (o modelo NÃO é autoridade final).
5. **Sem alucinação:** um caso com dado ausente → o agente **busca/pesquisa** (postura "incomplete, not
   evidence of absence") em vez de inventar; assert de que não há conclusão sem verificação.
6. **Comparável ao status quo:** medir tokens/s e sucesso de tool-calling no Q4_K_M vs Q3_K_M vs o modelo
   anterior (Qwen3-Coder-30B dense, se ainda instalar) e documentar o ganho.
7. `ollama ps` e `go build ./...` limpos; nenhuma dependência externa de nuvem introduzida (Lei do Cofre).

---

*Autor: cosca-architecture. Decisão fundamentada em arquitetura de GGUF/KV e trade-off memoria × compute da
RTX 5060 Ti 16GB. Escolho **Qwen3-Coder-30B-A3B-Instruct Q4_K_M** com **KV quantizado e ceiling de ~32k**,
recusando 64k como impraticável em 16GB. O modelo é a "compreensão"; a verificação determinística em código é
a "decisão". Resolve a dúvida: **dense 30B é inviável/lento na 16GB; o caminho é MoE 30B-A3B.***
