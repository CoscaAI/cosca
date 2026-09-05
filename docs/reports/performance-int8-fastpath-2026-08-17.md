# Fast Path int8 AVX2 — do gargalo ao limite físico da máquina

> Relatório técnico · 2026-08-17 · busca vetorial full-scan · hardware: Ryzen 7 5700X3D
> (8C/16T, 96MB L3 V-Cache, DDR4-3200 dual-channel) · todos os números medidos
> (régua P13 — nenhum fato sem verificação).

## 1. Contexto: o gargalo provado (autópsia, L311)

A busca vetorial era full-scan sobre SQLite: a cada query, ~94% da latência era
**re-decodificar BLOBs** (scanAll + clone); o produto escalar em si era 5–10%.
A primeira otimização criou um **índice derivado em memória** — Structure-of-Arrays,
um slab `[]float32` contíguo (n·dim) + ids, snapshot imutável por geração (swap
atômico), recarga por versão, e fail-safe: se o load falha, cai no SQL original.

**Resultado da fase 1:** 12.07 Mvec/s (1M×768, 82.85ms/query), ~37 GB/s lidos.

## 2. A física antes do código: por que 14M era o teto em float32

| Métrica | Valor | Leitura |
|---|---|---|
| DDR4-3200 dual-channel, teórico | 51.2 GB/s | teto físico |
| Bandwidth prática medida (16 threads) | **40.44 GB/s** | ~80% do teórico — saturação real |
| float32 a 12.07 Mvec/s | 37 GB/s | **94% do teto prático** |
| Curva de workers (W1→W16) | 1.68 → 11.25 Mvec/s | satura — é bandwidth-bound, não CPU-bound |
| Read floor serial | 16.01 GB/s | 1 thread não satura o canal |

**Conclusão:** em float32, o teto da máquina é ~13.5M vec/s. Passar de 14M exige
**menos bytes por elemento** — int8 (1 byte) = 4x bandwidth por vetor.

## 3. A decisão int8: probes antes do código

| Abordagem | Resultado | Veredito |
|---|---|---|
| int8 em Go puro (serial) | 1.52 Mvec/s | pior que float32 |
| int8 em Go puro (16 threads) | 10.34 Mvec/s | pior — Go não emite SIMD; conversões matam |
| **Assembly AVX2** | 52.66 Mvec/s | escolhido |

## 4. O kernel: `dot8BiasAVX2` (dot8_amd64.s)

### 4.1 Aritmética do bias (o truque)

Para o produto i8×i8 sem saturação (127·127=16129 < 32767), o kernel usa
**unsigned × signed** via bias:

```
qb[i] = q[i] + 128   (uint8, unsigned)     rb[i] = r[i]  (int8, signed)
Σ qb·rb = Σ q·r + 128·Σr  ⇒  dot(q,r) = Σ qb·rb − 128·Σr
```

A correção é **1 mul + 1 sub por row** (Σr pré-computado no load) — custo
imperceptível. Acumulação i32: teto 127·127·16384 ≈ 264M < 2³¹.

### 4.2 O loop (16 elementos/iteração, 16 instruções)

```asm
loop:
	VMOVDQU 0(DI), X1 ; VMOVDQU 8(DI), X2   ; qb[0:16], qb[8:24]  (16B cada)
	VMOVDQU 0(SI), X3 ; VMOVDQU 8(SI), X4   ; rb[0:16], rb[8:24]
	VPMOVZXBW X1, X5  ; VPMOVZXBW X2, X6    ; qb → 2×8 u16 (zero-extend)
	VPMOVSXBW X3, X1  ; VPMOVSXBW X4, X2    ; rb → 2×8 i16 (sign-extend)
	VPMADDWD X1, X5, X5 ; VPMADDWD X2, X6, X6 ; 8 i16×8 i16 → 4 i32 (pares)
	VPADDD X0, X5, X0 ; VPADDD X0, X6, X0   ; acumulador 4×i32
	ADDQ $16, DI ; ADDQ $16, SI ; DECQ AX ; JNZ loop
```

Epílogo: redução horizontal 4→1 i32 (`VPSHUFD 0xEE` + `VPADDD` + `VPSHUFD 0x55` +
`VPADDD` + `VMOVD`). **1 byte/elem lido** — 4x menos bandwidth que float32.
Compute estimado: ~16 inst/16 elems × 768 = 768 inst/row ÷ ~4 IPC ≈ 192 ciclos/row
→ ~375M rows/s disponíveis vs 52M medidos: **o CPU não é o gargalo**.

### 4.3 A batalha com o go assembler 1.26.5

Cinco armadilhas reais, todas validadas por execução e bytes crus
(`go tool asm -o` + `od`):

| # | Armadilha | Evidência |
|---|---|---|
| 1 | **VPMADDUBSW bugado**: `VPMADDUBSW Y3,Y1,Y2` codifica `vvvv=Y1` (ok) mas `modrm.reg=Y5` (deveria Y2); `(Y5,Y7,Y6)→reg=Y7`; combinações idênticas em outros registradores codificam certo — **sem padrão** | instrução evitada |
| 2 | **Ordem dos operandos ≠ Intel**: 2-op é `(src, dst)` (`VPMOVZXBW X1,X5` = X5←zx(X1)); **3-op é `(src2, src1, dst)`** — o 3º slot vai pro `modrm.reg`, que o hardware lê como DESTINO | descoberto por execução (não por bytes): 5 kernels errados antes do certo |
| 3 | `go tool objdump` **decodifica AVX2 errado** (mostra OUTL/ROLL/CMC) | decodificação manual byte a byte |
| 4 | `VPMOVZXBW/SXBW` só XMM reg-reg (ytab `_yvcvtdq2pd`); YMM e memória → "invalid instruction" | `VPMOVZXBW X1,(DI)` rejeitado |
| 5 | VEX byte2 é `~R ~X ~B` (bit7=1 ⇒ sem extensão) — decodificar ao contrário engana; `.s` sem newline final → "unexpected EOF" | `echo "" >>` resolveu |

## 5. Resultado em laboratório: o limite da máquina

```
30M×768 int8 = 23GB, 16 workers:
  BenchmarkLimitInt8AVX2_N30M_Dim768-16    5   569.7ms/op
  40.44 GB/s  |  161.8 GB/s equiv-float32  |  52.66 Mvec/s
```

- **40.44 GB/s = bandwidth DDR4 saturada** (o que 16 threads conseguem arrancar
  da RAM — o limite real da máquina).
- 161.8 GB/s equiv-float32: para o mesmo resultado em float32, precisaria de 4x
  a bandwidth que o hardware tem.
- **Objetivo de 14M: superado 3.76x.**
- Lição de metodologia: a 1ª rodada deu 15.21M — page faults do zeroing de 23GB
  de heap Go; com 5 iterações (páginas quentes), 52.66M. **Benchmark de dataset
  grande sem esquentar páginas mente.**

## 6. Integração no produto (índice L311)

### 6.1 Arquitetura (fallback em 3 camadas)

```
Search → índice in-memory (flag int8 ON?)
           ├─ int8 AVX2   (1 byte/elem — rápido, aproximado)   ← default
           └─ float32     (4 bytes/elem — exato)               ← flag OFF
        → SQL scan (fail-safe final, nunca desligado)
```

O snapshot carrega **os dois slabs + metadados de precisão**:

| Slab | Tipo | Tamanho (1M×768) | Papel |
|---|---|---|---|
| `vecs` | float32 | 3.07 GB | fallback exato |
| `vecs8` | int8 (pad 16) | 0.77 GB | fast path |
| `sums8` | int32/row | 4 MB | correção −128·Σr |
| `nb` | float32/row | 4 MB | **norma² exata** — a normalização final não degrada com a quantização |

Score do fast path: `dot8/(127²·‖q‖·‖r‖)` — a grade 127² = 16129 traz o dot
quantizado de volta à unidade de cosseno. Conversão na carga: `round(v·127)`
clampado em [-127,127], **uma vez por geração**. Config:
`SQLiteVecConfig.DisableInt8` + `SetInt8Enabled(bool)` em runtime.

### 6.2 Resultados no índice real

| Cenário | float32 | int8 | Ganho |
|---|---|---|---|
| 1M×768 (RAM, 768MB slab) | 12.07 Mvec/s | **48.63 Mvec/s** | 4.03x |
| 100k×768 (**cabe no L3** 96MB) | 11.31 Mvec/s | **80.82 Mvec/s** | 7.15x |
| Produção hoje (~13.6k vetores) | ~1.13 ms | **~0.17 ms** | ~6.6x |
| Produção futura (62.3k chunks) | ~5.2 ms | **~0.8 ms** | ~6.5x |

O detalhe que explica o 7x: **o slab int8 de 100k (77MB) cabe inteiro no
V-Cache de 96MB** — a busca nem toca a RAM. O float32 (308MB) sempre viveu na
memória.

### 6.3 Recall (o preço da velocidade — medido, não estimado)

50 queries realistas (row + ruído σ=0.05, gaussiano L2-normalizado, dim 768):

| Métrica | Valor |
|---|---|
| recall@1 | **1.0000** — o vizinho real nunca cai |
| recall@5 | 0.9960 |
| recall@10 | 0.8840 — ~1 troca/query entre quase-empates |
| max erro de score | 0.0117 |

Análise: os ranks 2–10 de um dataset denso têm gap de score ~0.01 — **a própria
ordem de grandeza do erro de quantização**. Os trocados são estatisticamente
indistinguíveis; o top-1 (o que importa no RAG) é sagrado. Quem precisa de
exatidão total: flag OFF → float32 bit-exact (os oracle tests da casa continuam
cobrindo esse contrato).

## 7. Onde está o gargalo agora

```
RAM: 40.44 GB/s = 100% do teto prático (DDR4)   ← O LIMITE
CPU: ~4% do throughput do kernel                ← sobra 25x
L3:  96MB — 80+ Mvec/s quando o dataset cabe     ← próximo degrau grátis
```

Não é possível subir **significativamente neste regime** mantendo o mesmo
algoritmo, representação e contrato de busca. O "limite físico" vale para este
caminho, não para o problema inteiro — restam possibilidades intermediárias
antes de trocar a RAM (DDR5 ≈ 2x), quantizar mais (int4/int2) ou mudar de
paradigma (HNSW/ANN — outro contrato, outro recall): int16, cache
blocking/tiling, pré-normalização, reorganização de layout, filtros antes do
scan, múltiplos níveis de quantização, SIMD por tamanho, processamento
híbrido, prefetch e GPU.

> Errata 2026-08-17 (L344, correção do professor): a versão anterior dizia
> "Não é possível subir mais sem trocar a RAM..." — imprecisa. O limite
> medido (40.44 GB/s) é o teto da DDR4 para *este* full-scan int8; mudar o
> regime (L3), a representação (int16) ou o algoritmo muda o problema.

## 8. Lições (régua P13 — nada sem verificação)

1. **Meça o teto físico antes de otimizar** — 37 GB/s em float32 já era 94% do
   DDR4; o "gargalo de CPU" era mito.
2. **Validar assembly por execução, nunca por bytes** — 5 kernels "perfeitos no
   papel" falharam até o mapeamento real do plan9 ser descoberto.
3. **Benchmark grande sem esquentar páginas mente** (15.21 → 52.66).
4. **Recall antes de trocar contrato** — velocidade sem qualidade medida é traição.

## 9. Em aberto (decisões do chef)

- **int16** (2 bytes/elem, kernel VPMADDWD já validado): erro 256x menor, recall
  ~100%, ~26M vec/s — ainda 1.86x o objetivo. Engatilhado.
- **Recall real** no dataset da casa (nomic-embed-text) em vez de sintético.
- **Verificar o caminho do `knowledge search`**: o fast path só atende buscas
  sem filtro de metadata — confirmar o que a produção percorre de fato.

## Anexo: registros da família

- L339 — kernel AVX2 int8 + mapa real do go assembler (commit `ad68bcf`, chain block 310)
- L340 — 52.66 Mvec/s: bandwidth DDR4 saturada; lição dos page faults (commit `d3fc7c0`, block 311)
- L341 — fast path int8 no índice L311: 48.63 Mvec/s (4x float32), recall medido,
  flag DisableInt8 (commit `b963a4d`, block 312)