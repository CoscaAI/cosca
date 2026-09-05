# DECISÃO 2026-08-01 — ROCm + Ollama local (independência de IA externa)

## Contexto
O Don quer o Cosca **100% independente de IA externa**. Testou a hipótese por voz
(02/08 00:47-00:49): sem provedor externo, o Kernel fica mudo. Aprovou a instalação
de Ollama local. Em seguida decidiu: primeiro instalar **AMD ROCm** para a GPU
funcionar corretamente, depois Ollama com suporte ROCm.

## Ordem aprovada pelo Don
1. ~~Instalar ROCm~~ → CONCLUÍDO (ver estado)
2. Instalar Ollama com suporte ROCm
3. Baixar modelo local (Qwen3 4B ou Llama 3.2 — cabe nos 12GB VRAM)
4. Configurar Cosca: provider local + fallback deepseek
5. Teste E2E por voz + commit

## Regra operacional nova (ordem do Don)
- **Qualquer comando `sudo` é enviado ao Don para ele executar** — o Kernel NUNCA
  roda sudo/root. O Don copia e cola, o Kernel valida.

## Estado da instalação ROCm (2026-08-01 22:05-22:15)
- ✅ Repositório: `/etc/apt/keyrings/rocm.gpg` + `/etc/apt/sources.list.d/rocm.list`
  (entrada `deb [arch=amd64 signed-by=/etc/apt/keyrings/rocm.gpg] https://repo.radeon.com/rocm/apt/latest noble main`)
- ✅ Pin de prioridade: `/etc/apt/preferences.d/rocm-pin-1001` (Pin-Priority 1001 no repo.radeon.com)
- ✅ Instalado: ROCm 7.2.4 — 30 pacotes (rocm-hip-libraries, rocm-opencl-runtime,
  hipblas, hipfft, hiprand, hipsolver, hipsparse, rocblas, rocfft, rccl, rocalution,
  hsa-rocr, rocminfo 1.0.0.70204-93~24.04, rocm-core 7.2.4.70204-93~24.04)
- ✅ Grupos: `cosca` adicionado a `render` + `video`
- ⚠️ VALIDAÇÃO PENDENTE: rocminfo deu "Permission denied no /dev/kfd" porque a
  sessão foi aberta antes do usermod → **Don vai reiniciar o PC (sudo reboot)**
- ⚠️ `rocminfo 5.7.1-3build1` do Ubuntu foi rejeitado em favor do `1.0.0.70204` da AMD
  (era a causa do "Unable to locate package"/conflito de dependências)

## Passos seguintes pós-reboot
1. Validar: `rocminfo` → esperado `Agent gfx1031` + Marketing Name Radeon RX 6700 XT
2. Validar: `hipInfo` / compilação HIP de teste
3. Instalar Ollama com HSA_OVERRIDE_GFX_VERSION se necessário (gfx1031 suportado nativo)
4. Configurar provider + failover
5. Registrar learning no cosca-kernel/learnings.md

## Hardware (diagnosticado)
- GPU: AMD Radeon RX 6700 XT (Navi 22, RDNA 2, gfx1031, 12GB — 11GB utilizável)
- CPU: 16 cores | RAM: 31GB | Ubuntu 24.04.4 LTS noble | kernel 7.0.0-28-generic
- Driver: amdgpu nativo do kernel (sem DKMS) — display funcionando
- `/dev/kfd` presente | `/dev/dri/renderD128` presente

## Aprovado pelo Don em 2026-08-01
Plano de execução apresentado com arquivos afetados, testes, rollback, tempo (~15-25min)
e confiança (85%). Don autorizou e determinou: comandos sudo vão para ele.
