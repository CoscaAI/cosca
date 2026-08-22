package compute

import (
	"os"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// =============================================================================
// CPU Probe — Fase 1 (discovery)
// =============================================================================
//
// A camada de descoberta de CPU em user space. Só informação observável sem
// privilégio: runtime.NumCPU, /proc/cpuinfo (vendor/model/flags), sysfs
// (/sys/devices/system/cpu/cpu0/cache e cpufreq). Nenhum acesso a
// /proc/sys/kernel/*, governor, drivers ou afinidade (KERNEL ACCESS POLICY).
//
// Toda fonte é best-effort: informação parcial é registrada (campo zero-value
// + Limitations), nunca um erro que derrube a função.

// CPUVendor identifica a família do fabricante da CPU.
type CPUVendor string

const (
	CPUAMD   CPUVendor = "amd"
	CPUIntel CPUVendor = "intel"
	CPUARM   CPUVendor = "arm"
	CPUOther CPUVendor = "other"
)

// CPUCacheInfo descreve um nível de cache da CPU (lido de
// /sys/devices/system/cpu/cpu0/cache/index* — leitura de user space).
type CPUCacheInfo struct {
	Level  int    `json:"level"`           // 1, 2, 3
	SizeKB int    `json:"size_kb"`         // 0 = desconhecido
	Type   string `json:"type,omitempty"`  // "Data" | "Instruction" | "Unified"
}

// CPUFrequencies reporta clocks da CPU em MHz (0 = desconhecido). Base é
// derivada do model name (Intel expõe "CPU @ 3.80GHz"; AMD 5700X3D não expõe
// o clock no nome — fica 0 e é registrado como limitação). Max vem de
// cpuinfo_max_freq; Current de scaling_cur_freq (fallback: "cpu MHz" do
// /proc/cpuinfo).
type CPUFrequencies struct {
	BaseMHz    int64 `json:"base_mhz,omitempty"`
	MaxMHz     int64 `json:"max_mhz,omitempty"`
	CurrentMHz int64 `json:"current_mhz,omitempty"`
}

// CPUCapacity prepara o modelo para a futura camada de limits (containers/
// cgroups). Fase 1 popula Physical e Visible; Effective fica 0 (=desconhecido)
// até a integração completa de limits.
type CPUCapacity struct {
	Physical  int `json:"physical,omitempty"`  // núcleos físicos do host
	Visible   int `json:"visible,omitempty"`   // CPUs que o processo enxerga
	Effective int `json:"effective,omitempty"` // limite efetivo (fase futura)
}

// CPUInfo é a fotografia read-only de descoberta da CPU (Fase 1). JSON
// serializável, espelhando o padrão do GPUInfo.
//
// Três camadas conceituais (Fase 1: detected == available == usable):
//   - Detected:  o hardware existe / foi observado;
//   - Available: o processo pode usar (runtime);
//   - Usable:    o workload pode usar.
type CPUInfo struct {
	Vendor       CPUVendor `json:"vendor"`
	Family       string    `json:"family,omitempty"`    // cpuinfo "cpu family"
	Model        string    `json:"model,omitempty"`     // cpuinfo "model" (id do stepping)
	Stepping     string    `json:"stepping,omitempty"`  // cpuinfo "stepping"
	ModelName    string    `json:"model_name,omitempty"` // nome de marketing
	Arch         string    `json:"arch"`                // runtime.GOARCH
	LogicalCPUs  int       `json:"logical_cpus"`
	PhysicalCores int      `json:"physical_cores,omitempty"`
	Packages     int       `json:"packages,omitempty"`
	Frequencies  CPUFrequencies `json:"frequencies,omitempty"`
	Caches       []CPUCacheInfo `json:"caches,omitempty"`
	Features     map[string]bool `json:"features"` // flags do /proc/cpuinfo (união)

	// Camadas detected/available/usable (Fase 1: iguais).
	Detected  bool `json:"detected"`
	Available bool `json:"available"`
	Usable    bool `json:"usable"`

	// Camada de capacidade preparada para a fase de limits.
	Capacity CPUCapacity `json:"capacity,omitempty"`

	// Limitations registra o que não foi determinável em user space.
	Limitations []string `json:"limitations,omitempty"`
}

// cpuSources abstrai as fontes read-only para o probeCPU poder ser exercitado
// contra um ambiente fake nos testes. Produção usa defaultCPUSources.
type cpuSources struct {
	readFile func(path string) string
	listDir  func(path string) []string
	numCPU   func() int
	goArch   func() string
}

// ProbeCPU descobre a capacidade da CPU. Estritamente read-only e best-effort:
// nenhuma fonte falha a função como um todo — informação parcial é registrada
// no modelo (zero-value + Limitations).
func ProbeCPU() CPUInfo {
	return probeCPU(defaultCPUSources())
}

func probeCPU(s cpuSources) CPUInfo {
	info := CPUInfo{
		Arch:        s.goArch(),
		LogicalCPUs: s.numCPU(),
		Features:    map[string]bool{},
	}

	// /proc/cpuinfo: vendor, family/model/stepping, cores físicos, packages,
	// flags (features) e "cpu MHz" (fallback de frequência atual).
	out := s.readFile("/proc/cpuinfo")
	vendor, family, model, stepping, modelName, physCores, packages, features, cpuMHz := parseCPUInfo(out)
	info.Vendor = vendor
	info.Family = family
	info.Model = model
	info.Stepping = stepping
	info.ModelName = modelName
	info.PhysicalCores = physCores
	info.Packages = packages
	info.Features = features

	// sysfs: caches (cpu0) e frequências (cpufreq).
	info.Caches = parseCaches(s)
	info.Frequencies = parseFrequencies(s, cpuMHz)
	if info.Frequencies.BaseMHz == 0 {
		info.Frequencies.BaseMHz = baseMHzFromModelName(modelName)
	}

	// ARM não expõe vendor_id no /proc/cpuinfo — usa o GOARCH como fallback.
	if info.Vendor == "" && strings.Contains(strings.ToLower(info.Arch), "arm") {
		info.Vendor = CPUARM
	}

	// Camadas detected/available/usable (Fase 1: idênticas).
	info.Detected = info.LogicalCPUs > 0
	info.Available = info.Detected
	info.Usable = info.Detected

	// Camada de capacidade preparada (Effective reservado para a fase de limits).
	info.Capacity.Physical = info.PhysicalCores
	info.Capacity.Visible = info.LogicalCPUs

	// Limitações registradas — nunca contornadas (KERNEL ACCESS POLICY).
	if info.PhysicalCores == 0 {
		info.Limitations = append(info.Limitations,
			"núcleos físicos não determináveis (physical id/core id ausentes — possível VM/container)")
	}
	if info.Vendor == "" {
		info.Limitations = append(info.Limitations,
			"vendor não determinável em user space neste ambiente")
	}
	if info.Frequencies.BaseMHz == 0 && info.Frequencies.MaxMHz == 0 && info.Frequencies.CurrentMHz == 0 {
		info.Limitations = append(info.Limitations,
			"frequências não expostas pelo kernel (sysfs cpufreq indisponível)")
	} else {
		if info.Frequencies.MaxMHz == 0 {
			info.Limitations = append(info.Limitations,
				"frequência máxima não exposta pelo kernel (sysfs cpufreq indisponível)")
		}
		if info.Frequencies.BaseMHz == 0 {
			info.Limitations = append(info.Limitations,
				"clock base não exposto em user space (model name sem GHz; sysfs não expõe o P-state base)")
		}
	}

	return info
}

// =============================================================================
// Default (real) sources
// =============================================================================

func defaultCPUSources() cpuSources {
	return cpuSources{
		readFile: func(path string) string {
			data, err := os.ReadFile(path)
			if err != nil {
				return ""
			}
			return strings.TrimSpace(string(data))
		},
		listDir: func(path string) []string {
			entries, err := os.ReadDir(path)
			if err != nil {
				return nil
			}
			names := make([]string, 0, len(entries))
			for _, e := range entries {
				names = append(names, e.Name())
			}
			return names
		},
		numCPU: runtime.NumCPU,
		goArch: func() string { return runtime.GOARCH },
	}
}

// =============================================================================
// Parsers (puros, unit-testáveis)
// =============================================================================

// parseCPUInfo extrai os campos de descoberta do /proc/cpuinfo. Features é a
// união das flags de TODOS os processors (o esperado é serem idênticas). Cores
// físicos = pares únicos (physical id, core id); packages = physical ids únicos.
func parseCPUInfo(out string) (vendor CPUVendor, family, model, stepping, modelName string,
	physicalCores, packages int, features map[string]bool, cpuMHz float64) {
	features = map[string]bool{}
	seenCores := map[string]bool{}
	seenPackages := map[string]bool{}

	phys, core := "", ""
	for _, line := range strings.Split(out, "\n") {
		fields := strings.SplitN(line, ":", 2)
		if len(fields) != 2 {
			continue
		}
		key := strings.TrimSpace(fields[0])
		val := strings.TrimSpace(fields[1])

		switch key {
		case "processor":
			if phys != "" && core != "" {
				seenCores[phys+"|"+core] = true
			}
			phys, core = "", ""
		case "vendor_id":
			switch val {
			case "AuthenticAMD":
				vendor = CPUAMD
			case "GenuineIntel":
				vendor = CPUIntel
			default:
				if vendor == "" {
					vendor = CPUOther
				}
			}
		case "cpu family":
			if family == "" {
				family = val
			}
		case "model":
			if model == "" {
				model = val
			}
		case "stepping":
			if stepping == "" {
				stepping = val
			}
		case "model name":
			if modelName == "" {
				modelName = val
			}
		case "physical id":
			phys = val
			seenPackages[val] = true
		case "core id":
			core = val
		case "flags":
			for _, f := range strings.Fields(val) {
				features[f] = true
			}
		case "cpu MHz":
			if cpuMHz == 0 {
				if v, err := strconv.ParseFloat(val, 64); err == nil {
					cpuMHz = v
				}
			}
		}
	}
	if phys != "" && core != "" {
		seenCores[phys+"|"+core] = true
	}
	return vendor, family, model, stepping, modelName,
		len(seenCores), len(seenPackages), features, cpuMHz
}

// parseCaches lê os níveis de cache do /sys/devices/system/cpu/cpu0/cache/
// (index0..N), ordenados por nível. User space puro.
func parseCaches(s cpuSources) []CPUCacheInfo {
	const base = "/sys/devices/system/cpu/cpu0/cache"
	var caches []CPUCacheInfo
	for _, name := range s.listDir(base) {
		if !strings.HasPrefix(name, "index") {
			continue
		}
		dir := base + "/" + name
		level, err := strconv.Atoi(strings.TrimSpace(s.readFile(dir + "/level")))
		if err != nil || level <= 0 {
			continue
		}
		caches = append(caches, CPUCacheInfo{
			Level:  level,
			SizeKB: parseCacheSizeKB(s.readFile(dir + "/size")),
			Type:   s.readFile(dir + "/type"),
		})
	}
	sort.Slice(caches, func(i, j int) bool { return caches[i].Level < caches[j].Level })
	return caches
}

// parseCacheSizeKB converte "32K" → 32, "98304K" → 98304, "3M" → 3072.
func parseCacheSizeKB(size string) int {
	size = strings.TrimSpace(strings.ToUpper(size))
	if size == "" {
		return 0
	}
	mult := 1
	switch {
	case strings.HasSuffix(size, "K"):
		size = strings.TrimSuffix(size, "K")
	case strings.HasSuffix(size, "M"):
		size = strings.TrimSuffix(size, "M")
		mult = 1024
	case strings.HasSuffix(size, "G"):
		size = strings.TrimSuffix(size, "G")
		mult = 1024 * 1024
	}
	n, err := strconv.Atoi(size)
	if err != nil {
		return 0
	}
	return n * mult
}

// parseFrequencies lê o cpufreq do sysfs (kHz → MHz). Current usa
// scaling_cur_freq; se o sysfs não expuser, usa o "cpu MHz" do /proc/cpuinfo.
func parseFrequencies(s cpuSources, cpuinfoMHz float64) CPUFrequencies {
	const base = "/sys/devices/system/cpu/cpu0/cpufreq"
	f := CPUFrequencies{}
	if maxKHz, err := strconv.ParseInt(strings.TrimSpace(s.readFile(base+"/cpuinfo_max_freq")), 10, 64); err == nil && maxKHz > 0 {
		f.MaxMHz = maxKHz / 1000
	}
	if curKHz, err := strconv.ParseInt(strings.TrimSpace(s.readFile(base+"/scaling_cur_freq")), 10, 64); err == nil && curKHz > 0 {
		f.CurrentMHz = curKHz / 1000
	} else if cpuinfoMHz > 0 {
		f.CurrentMHz = int64(cpuinfoMHz)
	}
	return f
}

var ghzPattern = regexp.MustCompile(`([0-9]+\.[0-9]+)GHz`)

// baseMHzFromModelName extrai o clock base do model name ("CPU @ 3.80GHz").
// AMD 5700X3D não expõe o clock no nome → 0 (registrado como limitação).
func baseMHzFromModelName(modelName string) int64 {
	m := ghzPattern.FindStringSubmatch(modelName)
	if len(m) < 2 {
		return 0
	}
	ghz, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0
	}
	return int64(ghz * 1000)
}
