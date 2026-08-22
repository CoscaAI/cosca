# Compute Fabric Tuning (5700X3D)

> Como aumentar a capacidade de processamento e memória do Compute Fabric
> (ordem do Don, 2026-08-16).

## 1. O que é o Fabric

O Compute Fabric (`internal/compute/fabric.go`) gerencia os worker pools do
Cosca (agent/tool/index/io/sandbox/gpu) com escala adaptativa por load.
Os tetos dos pools são derivados de `FabricConfig` (percentuais dos cores
lógicos) via `LoadFabricConfig()` — que lê as env vars `COSCA_FABRIC_*_PERCENT`.

## 2. O tuning aplicado (serve.env)

O serve lê as vars do `~/.config/cosca/serve.env` (EnvironmentFile do systemd).
Para o 5700X3D (16 threads):

| Var | Antes | Depois | Tetos (16T) |
|-----|-------|--------|-------------|
| `COSCA_FABRIC_CPU_PERCENT` | 0.75 | **1.0** | agent 12 → **16** |
| `COSCA_FABRIC_TOOL_PERCENT` | 0.375 | **0.5** | tool 6 → **8** |
| `COSCA_FABRIC_INDEX_PERCENT` | 0.25 | **0.375** | index 4 → **6** |
| `COSCA_FABRIC_IO_PERCENT` | 0.25 | **0.375** | io 4 → **6** |
| `COSCA_FABRIC_SANDBOX_PERCENT` | 0.25 | **0.375** | sandbox 4 → **6** |

**Nota**: os tetos são MÁXIMOS — o fabric escala por demanda (load < 0.5 com
backlog → sobe; ocioso → desce). Somar os tetos (42) > 16 threads é seguro:
workers de I/O (index/io/sandbox) esperam I/O e não consomem CPU; o scheduler
adaptativo mantém o mínimo quando não há trabalho.

## 3. Como verificar

```bash
# Ver os tetos com as vars aplicadas:
COSCA_FABRIC_CPU_PERCENT=1.0 COSCA_FABRIC_TOOL_PERCENT=0.5 \
COSCA_FABRIC_INDEX_PERCENT=0.375 COSCA_FABRIC_IO_PERCENT=0.375 \
COSCA_FABRIC_SANDBOX_PERCENT=0.375 cosca fabric
```

## 4. Próximos passos (arquitetural — F7/F9)

O tuning por env é o ganho rápido. A evolução correta é ligar o Fabric ao
**Capability Model (F7)** e ao **Benchmark Contract (F9)**:
- MemoryBudget: usar `AvailableRAM`/`EffectiveBytes` (F2) em vez de
  `TotalRAM × 0.8` — hoje o budget é 25GB com 21GB realmente livres
- Pools: escalar pelo `effective_capacity` (F4 cpuset) em vez de NumCPU cego
- GPU: o pool gpu dorme (0/0) dentro da jaula (sem device nodes — F5) —
  o executor GPU precisa rodar no host para ativar os ~13 TFLOPs da RX 6700 XT
