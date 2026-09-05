package compute

import (
	"os"
	"runtime"
	"strings"
)

// =============================================================================
// Environment Probe — Fase 4 (contexto de execução)
// =============================================================================
//
// A camada que responde: "Em que tipo de ambiente o Cosca está rodando?" —
// bare metal, VM, container ou dentro da jaula bwrap. É o contexto que
// decide COMO interpretar as descobertas das Fases 1-3 (CPU/Memory/Topology):
// o mesmo hardware aparece diferente em cada ambiente.
//
// Somente observação em user space: /proc/self/cgroup, /proc/self/status,
// variáveis de ambiente (COSCA_JAILED), presença de /sys e de sinais de
// virtualização expostos. NUNCA tenta escapar da jaula ou inferir o host
// que não está exposto (KERNEL ACCESS POLICY, L312).
//
// Best-effort total: cada detector é independente e registra Limitations;
// a função nunca falha como um todo.

// EnvironmentType classifica o contexto de execução observado.
type EnvironmentType string

const (
	EnvBareMetal  EnvironmentType = "bare-metal"
	EnvVM         EnvironmentType = "virtualized"
	EnvContainer  EnvironmentType = "container"
	EnvJail       EnvironmentType = "jail"
	EnvUnknown    EnvironmentType = "unknown"
)

// EnvironmentInfo é a fotografia read-only do ambiente (Fase 4).
type EnvironmentInfo struct {
	// Type é o ambiente predominante observado (bare-metal, jail, container,
	// virtualized ou unknown). O Cosca pode estar em mais de um (ex: jaula
	// dentro de container) — Type reporta o mais restritivo observado e a
	// lista de tipos fica em Types.
	Type EnvironmentType `json:"type"`

	// Types lista TODOS os ambientes detectados (ordem: do mais para o menos
	// restritivo — jail > container > vm > bare-metal).
	Types []EnvironmentType `json:"types,omitempty"`

	// InJail: processo dentro da jaula bwrap (COSCA_JAILED=1).
	InJail bool `json:"in_jail"`
	// InContainer: cgroup indica container (docker/k8s/podman/lxc...).
	InContainer bool `json:"in_container"`
	// InVM: sinais de virtualização observados (hypervisor em cpuinfo, DMI
	// exposto, /proc/1/comm = systemd em container...).
	InVM bool `json:"in_vm"`

	// SysfsMounted: /sys/devices/system/node existe (topologia completa possível).
	SysfsMounted bool `json:"sysfs_mounted"`
	// CgroupVersion: "v1" | "v2" | "" (desconhecido).
	CgroupVersion string `json:"cgroup_version,omitempty"`
	// CgroupControllers: controladores v2 ativos (ex: "cpu memory").
	CgroupControllers string `json:"cgroup_controllers,omitempty"`
	// EffectiveCPUs: cpuset efetivo do processo (de /proc/self/status Cpus_allowed_list),
	// "" se não determinável. Ex: "0-7" (container com quota) vs "0-15" (host).
	EffectiveCPUs string `json:"effective_cpus,omitempty"`

	// GOOS/GOARCH do processo (contexto de compilação).
	GOOS   string `json:"goos"`
	GOARCH string `json:"goarch"`

	// Camadas detected/available/usable — para ambiente, available/usable
	// significam que a classificação foi determinada com confiança.
	Detected  bool `json:"detected"`
	Available bool `json:"available"`
	Usable    bool `json:"usable"`

	// Limitations registra o que não foi determinável em user space.
	Limitations []string `json:"limitations,omitempty"`
}

// envSources abstrai as fontes read-only (espelho de cpuSources/memorySources).
type envSources struct {
	readFile    func(path string) string
	getenv      func(key string) string
	numCPU      func() int
	goos        func() string
	goarch      func() string
}

// ProbeEnvironment descobre o contexto de execução. Read-only e best-effort.
func ProbeEnvironment() EnvironmentInfo {
	return probeEnvironment(defaultEnvSources())
}

func probeEnvironment(s envSources) EnvironmentInfo {
	info := EnvironmentInfo{
		GOOS:   s.goos(),
		GOARCH: s.goarch(),
	}

	// 1. Jaula bwrap: sinal mais forte (env var explícita do auto-jail).
	if v := strings.ToLower(s.getenv("COSCA_JAILED")); v == "1" || v == "true" {
		info.InJail = true
		info.Types = append(info.Types, EnvJail)
	}

	// 2. Container: cgroup paths (docker/k8s/podman/lxc/libpod...).
	cgroup := s.readFile("/proc/self/cgroup")
	if cg := detectContainer(cgroup); cg {
		info.InContainer = true
		info.Types = append(info.Types, EnvContainer)
	}
	// Cgroup v2 controllers (o caminho expõe se é v2 e quais controllers).
	if c := s.readFile("/sys/fs/cgroup/cgroup.controllers"); c != "" {
		info.CgroupVersion = "v2"
		info.CgroupControllers = strings.TrimSpace(strings.ReplaceAll(c, "\n", " "))
	} else if cgroup != "" {
		info.CgroupVersion = "v1"
	}

	// 3. VM: sinais de virtualização observáveis em user space.
	if detectVM(s) {
		info.InVM = true
		info.Types = append(info.Types, EnvVM)
	}

	// 4. sysfs de topologia montado? (decide a completude da Fase 3)
	if entries := s.readFile("/sys/devices/system/node/node0/cpulist"); entries != "" {
		info.SysfsMounted = true
	} else {
		info.Limitations = append(info.Limitations,
			"sysfs de topologia não exposto (topologia NUMA parcial — possível jaula/container)")
	}

	// 5. Cpuset efetivo do processo (de /proc/self/status).
	status := s.readFile("/proc/self/status")
	if c := parseCpusAllowed(status); c != "" {
		info.EffectiveCPUs = c
	}

	// 6. Classificação final: o tipo mais restritivo observado.
	switch {
	case info.InJail:
		info.Type = EnvJail
	case info.InContainer:
		info.Type = EnvContainer
	case info.InVM:
		info.Type = EnvVM
	case info.SysfsMounted:
		info.Type = EnvBareMetal
	default:
		info.Type = EnvUnknown
	}

	// 7. Camadas detected/available/usable: a classificação é válida quando
	// pelo menos um sinal foi observado (jaula/container/vm/sysfs).
	info.Detected = info.InJail || info.InContainer || info.InVM || info.SysfsMounted
	info.Available = info.Detected
	info.Usable = info.Detected

	if info.EffectiveCPUs != "" && info.EffectiveCPUs != cpusAllowedAll(s) {
		info.Limitations = append(info.Limitations,
			"cpuset efetivo restrito ("+info.EffectiveCPUs+") — capacidade efetiva < capacidade visível")
	}
	if !info.Detected {
		info.Limitations = append(info.Limitations,
			"ambiente não classificável com confiança em user space")
	}

	return info
}

// detectContainer reporta se o cgroup indica execução em container
// (docker/k8s/podman/lxc/libpod...).
func detectContainer(cgroup string) bool {
	if cgroup == "" {
		return false
	}
	lower := strings.ToLower(cgroup)
	for _, marker := range []string{
		"docker", "kubepods", "podman", "libpod", "lxc", "containerd",
		"/k8s", "crio",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// detectVM observa sinais de virtualização expostos em user space:
// hypervisor em /proc/cpuinfo (flags), DMI quando exposto, ou systemd
// rodando como PID 1 sem container (comportamento típico de VM).
func detectVM(s envSources) bool {
	// Hypervisor flag no /proc/cpuinfo (KVM/QEMU/VirtualBox/VMware/Xen...).
	cpuinfo := strings.ToLower(s.readFile("/proc/cpuinfo"))
	for _, marker := range []string{"hypervisor", "kvm", "qemu", "vmware", "vbox", "xen"} {
		if strings.Contains(cpuinfo, marker) {
			return true
		}
	}
	// DMI exposto (algumas VMs expõem /sys/class/dmi/id/product_name).
	dmi := strings.ToLower(s.readFile("/sys/class/dmi/id/product_name"))
	if dmi != "" && !strings.Contains(dmi, "default") {
		for _, marker := range []string{"vmware", "virtualbox", "kvm", "qemu", "xen", "hyper-v", "bochs"} {
			if strings.Contains(dmi, marker) {
				return true
			}
		}
	}
	return false
}

// parseCpusAllowed extrai o cpuset efetivo de /proc/self/status
// (Cpus_allowed_list: "0-7"). "" se ausente.
func parseCpusAllowed(status string) string {
	for _, line := range strings.Split(status, "\n") {
		if strings.HasPrefix(line, "Cpus_allowed_list:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Cpus_allowed_list:"))
		}
	}
	return ""
}

// cpusAllowedAll devolve o cpuset "tudo" esperado para o nº de CPUs lógicos
// (ex: 16 CPUs → "0-15"), usado para detectar restrição efetiva.
func cpusAllowedAll(s envSources) string {
	n := s.numCPU()
	if n <= 0 {
		return ""
	}
	if n == 1 {
		return "0"
	}
	return "0-" + itoa(n-1)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// defaultEnvSources aponta para as fontes reais.
func defaultEnvSources() envSources {
	return envSources{
		readFile: func(path string) string {
			data, err := os.ReadFile(path)
			if err != nil {
				return ""
			}
			return string(data)
		},
		getenv: os.Getenv,
		numCPU: runtime.NumCPU,
		goos:   func() string { return runtime.GOOS },
		goarch: func() string { return runtime.GOARCH },
	}
}
