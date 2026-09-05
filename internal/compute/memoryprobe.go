package compute

import (
	"os"
	"strconv"
	"strings"
)

// =============================================================================
// Memory Probe — Fase 2 (discovery)
// =============================================================================
//
// A camada de descoberta de memória em user space. Só informação observável
// sem privilégio: /proc/meminfo, page size (getpagesize), cgroup v1/v2 memory
// limit quando legível e RLIMIT_AS (getrlimit). Nenhum acesso a
// /proc/sys/kernel/*, cgroups de escrita ou ampliação de privilégios
// (KERNEL ACCESS POLICY — L312).
//
// A cadeia de capacidade é separada em quatro camadas:
//   - PHYSICAL:  memória instalada / visível ao OS (meminfo MemTotal);
//   - VISIBLE:   o que o OS/processo enxerga (em VM pode diferir do host);
//   - AVAILABLE: disponível AGORA (meminfo MemAvailable — atenção: MemFree
//                ≠ MemAvailable; MemFree não considera cache recuperável);
//   - EFFECTIVE: limite efetivamente concedido ao processo/ambiente (cgroup
//                memory limit ou RLIMIT_AS quando observáveis; fallback:
//                visible). SOMENTE OBSERVAÇÃO, nunca alteração.
//
// Toda fonte é best-effort: informação parcial é registrada (campo zero-value
// + Limitations), nunca um erro que derrube a função. "Tipo de memória"
// (DDR4/DDR5), canais e frequência da RAM não são observáveis de forma
// confiável em user space — registrados como limitação, não procurados.

// MemoryInfo é a fotografia read-only de descoberta de memória (Fase 2).
// JSON serializável, espelhando o padrão do CPUInfo/GPUInfo.
//
// Valores em BYTES. 0 = desconhecido (fonte ausente ou não observável).
//
// Três camadas conceituais (padrão da Fase 1):
//   - Detected:  a memória foi observada (physical > 0);
//   - Available: há memória disponível agora (available > 0);
//   - Usable:    o workload pode usar (effective > 0).
type MemoryInfo struct {
	PhysicalBytes      int64    `json:"physical_bytes,omitempty"`      // instalada/visível ao OS (MemTotal)
	VisibleBytes       int64    `json:"visible_bytes,omitempty"`       // o que o OS/processo enxerga
	AvailableBytes     int64    `json:"available_bytes,omitempty"`     // disponível agora (MemAvailable)
	EffectiveBytes     int64    `json:"effective_bytes,omitempty"`     // limite efetivo concedido (cgroup/rlimit; fallback visible)
	PageSize           int64    `json:"page_size,omitempty"`           // os.Getpagesize() / sysconf(_SC_PAGESIZE)
	SwapTotalBytes     int64    `json:"swap_total_bytes,omitempty"`    // SwapTotal (meminfo)
	SwapAvailableBytes int64    `json:"swap_available_bytes,omitempty"` // SwapFree (meminfo)

	Detected  bool `json:"detected"`
	Available bool `json:"available"`
	Usable    bool `json:"usable"`

	// Limitations registra o que não foi determinável em user space (e por quê).
	Limitations []string `json:"limitations,omitempty"`
}

// memorySources abstrai as fontes read-only para o probeMemory poder ser
// exercitado contra um ambiente fake nos testes. Produção usa
// defaultMemorySources.
type memorySources struct {
	readFile func(path string) string
	pageSize func() int64
	// rlimitAS devolve o limite atual de endereço (RLIMIT_AS) em bytes e true
	// quando o limite é finito (cur != RLIM_INFINITY). false = sem limite
	// observável ou getrlimit indisponível.
	rlimitAS func() (cur uint64, ok bool)
}

// ProbeMemory descobre a capacidade de memória. Estritamente read-only e
// best-effort: nenhuma fonte falha a função como um todo — informação parcial
// é registrada no modelo (zero-value + Limitations).
func ProbeMemory() MemoryInfo {
	return probeMemory(defaultMemorySources())
}

func probeMemory(s memorySources) MemoryInfo {
	info := MemoryInfo{PageSize: s.pageSize()}

	// /proc/meminfo: MemTotal (physical/visible), MemAvailable (agora),
	// SwapTotal/SwapFree. MemFree ≠ MemAvailable — MemAvailable desconta a
	// cache recuperável e o reclaimável, é a métrica real de "quanto dá pra
	// alocar já".
	out := s.readFile("/proc/meminfo")
	physical, available, swapTotal, swapFree := parseMemInfo(out)
	info.PhysicalBytes = physical
	info.VisibleBytes = physical // meminfo MemTotal já reflete o que o OS vê
	info.AvailableBytes = available
	info.SwapTotalBytes = swapTotal
	info.SwapAvailableBytes = swapFree

	// EFFECTIVE: cgroup v1/v2 memory limit quando legível e finito, depois
	// RLIMIT_AS, sempre mantendo o mais restritivo observável; fallback = visible.
	cgLimit, cgOK := parseCgroupMemoryLimit(s)
	rlLimit, rlOK := s.rlimitAS()
	info.EffectiveBytes = resolveEffective(info.VisibleBytes, cgLimit, cgOK, rlLimit, rlOK)

	// Camadas detected/available/usable.
	info.Detected = info.PhysicalBytes > 0
	info.Available = info.Detected && info.AvailableBytes > 0
	info.Usable = info.Available && info.EffectiveBytes > 0

	// Limitações registradas — nunca contornadas (KERNEL ACCESS POLICY).
	if !info.Detected {
		info.Limitations = append(info.Limitations,
			"memória total não observável em user space (meminfo indisponível)")
	} else if info.PageSize == 0 {
		info.Limitations = append(info.Limitations,
			"page size não determinável em user space")
	}
	if info.AvailableBytes == 0 && info.Detected {
		info.Limitations = append(info.Limitations,
			"memória disponível não observável (MemAvailable/MemFree ausentes no meminfo)")
	}
	if info.SwapTotalBytes == 0 && info.Detected {
		info.Limitations = append(info.Limitations,
			"swap não observável (SwapTotal ausente — possível ambiente sem swap)")
	}
	info.Limitations = append(info.Limitations,
		"memória física total do host indistinguível de visível em user space (sem acesso a ACPI/dmidecode — possível VM)",
		"tipo de memória (DDR4/DDR5), canais e frequência da RAM não observáveis de forma confiável em user space")
	if !cgOK && !rlOK && info.Detected {
		info.Limitations = append(info.Limitations,
			"limite efetivo = visível (sem cgroup memory limit finito nem RLIMIT_AS observáveis sem privilégio)")
	} else if info.Detected && info.EffectiveBytes < info.VisibleBytes {
		info.Limitations = append(info.Limitations,
			"limite efetivo mais restritivo que o visível (cgroup memory limit ou RLIMIT_AS imposto)")
	}

	return info
}

// =============================================================================
// Default (real) sources
// =============================================================================

func defaultMemorySources() memorySources {
	return memorySources{
		readFile: func(path string) string {
			data, err := os.ReadFile(path)
			if err != nil {
				return ""
			}
			return strings.TrimSpace(string(data))
		},
		pageSize: func() int64 { return int64(os.Getpagesize()) },
		// rlimitAS: fonte platform-specific (syscall.Getrlimit no Linux; no
		// Windows não há RLIMIT_AS observável — ver memoryprobe_linux.go /
		// memoryprobe_windows.go).
		rlimitAS: defaultRlimitAS,
	}
}

// =============================================================================
// Parsers (puros, unit-testáveis)
// =============================================================================

// parseMemInfo extrai MemTotal, MemAvailable, SwapTotal e SwapFree do
// /proc/meminfo, convertendo kB → bytes. Quando MemAvailable não existe
// (kernel antigo), usa MemFree como fallback (documentado na limitação).
func parseMemInfo(out string) (physical, available, swapTotal, swapFree int64) {
	var memTotal, memFree, memAvailable, swTotal, swFree int64
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		val, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}
		bytes := val * 1024 // meminfo está em kB
		switch fields[0] {
		case "MemTotal:":
			memTotal = bytes
		case "MemFree:":
			memFree = bytes
		case "MemAvailable:":
			memAvailable = bytes
		case "SwapTotal:":
			swTotal = bytes
		case "SwapFree:":
			swFree = bytes
		}
	}
	if memAvailable == 0 {
		memAvailable = memFree
	}
	return memTotal, memAvailable, swTotal, swFree
}

// parseCgroupMemoryLimit lê o limite de memória do cgroup do processo
// (v1/v2) quando legível em user space. Devolve o limite em bytes e true
// apenas quando existe um limite FINITO observável:
//   - v2: /sys/fs/cgroup + <caminho do /proc/self/cgroup> + "/memory.max"
//     (valor "max" = ilimitado → não é limite finito);
//   - v1: /sys/fs/cgroup/memory + <caminho> + "/memory.limit_in_bytes"
//     (valor sentinela ≈ 2^63-1024 = "sem limite" → não é limite finito).
//
// Não altera nada — só observação. Se nada for legível/finito, (0, false).
func parseCgroupMemoryLimit(s memorySources) (limit int64, ok bool) {
	// Caminho do cgroup do processo, relativo ao mount point.
	path := cgroupPathFromProcSelf(s.readFile("/proc/self/cgroup"))
	if path == "" {
		return 0, false
	}

	// v2: mount unified em /sys/fs/cgroup. Detectado por uma linha "0::".
	if v2 := isCgroupV2(s.readFile("/proc/self/cgroup")); v2 {
		raw := s.readFile("/sys/fs/cgroup" + path + "/memory.max")
		if raw == "" {
			return 0, false
		}
		if raw == "max" {
			return 0, false
		}
		n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil || n <= 0 {
			return 0, false
		}
		return n, true
	}

	// v1: controller memory em /sys/fs/cgroup/memory.
	raw := s.readFile("/sys/fs/cgroup/memory" + path + "/memory.limit_in_bytes")
	if raw == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	// Sentinela do v1 para "sem limite": o kernel grava um valor próximo de
	// LONG_MAX (tipicamente 2^63-4096 ≈ 8 EiB). Qualquer valor ≥ 2^62
	// (≈ 4 EiB) é tratado como ilimitado — um limite real nunca chega perto.
	if uint64(n) >= uint64(1)<<62 {
		return 0, false
	}
	return n, true
}

// cgroupPathFromProcSelf extrai o caminho do cgroup da linha "0::" (v2) ou da
// linha que contém o subsistema "memory" (v1). Formato: "<id>:<subs>:<path>".
func cgroupPathFromProcSelf(out string) string {
	if out == "" {
		return ""
	}
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			continue
		}
		subs := parts[1]
		if parts[0] == "0" && subs == "" { // v2 unificado: "0::/path"
			return parts[2]
		}
		for _, s := range strings.Split(subs, ",") {
			if s == "memory" { // v1: "2:memory:/path" (ou "memory,cpu")
				return parts[2]
			}
		}
	}
	return ""
}

// isCgroupV2 reporta se o /proc/self/cgroup indica a hierarquia unificada v2.
func isCgroupV2(out string) bool {
	return strings.HasPrefix(strings.TrimSpace(out), "0::")
}

// resolveEffective combina os limites observáveis mantendo o mais restritivo:
// começa do visível e aplica o cgroup memory limit e/ou o RLIMIT_AS quando
// finitos. Sem limite finito, o effective é o próprio visible (fallback).
func resolveEffective(visible int64, cgLimit int64, cgOK bool, rlCur uint64, rlOK bool) int64 {
	if visible <= 0 {
		return 0
	}
	effective := visible
	if cgOK && cgLimit > 0 && cgLimit < effective {
		effective = cgLimit
	}
	if rlOK && rlCur > 0 && int64(rlCur) < effective {
		effective = int64(rlCur)
	}
	return effective
}
