package compute

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// =============================================================================
// Topology Probe — Fase 3 (discovery)
// =============================================================================
//
// A camada de descoberta de TOPOLOGIA de CPU e memória (incluindo NUMA) em
// user space. Responde: "como os recursos (CPUs, memória, NUMA nodes) estão
// ORGANIZADOS neste ambiente?" — puramente OBSERVAÇÃO, nunca modificação.
//
// Fontes (todas read-only, sem privilégio):
//   - /sys/devices/system/node/node*/cpulist  → CPUs de cada NUMA node
//   - /sys/devices/system/node/node*/meminfo   → MemTotal/MemFree por node
//   - /sys/devices/system/node/node*/distance  → distâncias entre nodes
//   - /proc/cpuinfo ("physical id"/"core id")  → packages/sockets e cores
//     (reutiliza a mesma fonte da Fase 1)
//   - /sys/devices/system/cpu/cpu*/topology/   → thread siblings por CPU
//
// A cadeia é separada em três camadas (padrão das Fases 1-2):
//   - PHYSICAL:  topologia real do hardware (sysfs);
//   - VISIBLE:   o que o ambiente expõe (na jaula bwrap o sysfs pode NÃO ser
//                montado — a topologia NUMA real fica indisponível e cai no
//                fallback de 1 node, registrado como limitação);
//   - EFFECTIVE: o que o processo pode usar (runtime.NumCPU reflete o cpuset;
//                aqui as camadas observadas são phys/visible — affinity/política
//                NUMA NUNCA é modificada, apenas observada).
//
// KERNEL ACCESS POLICY (L312): USER SPACE ONLY. Nenhum acesso a
// /proc/sys/kernel/*, nenhuma alteração de afinidade, nenhum taskset/numactl
// para MODIFICAR. Se uma informação não puder ser obtida sem privilégio:
// REGISTRAR A LIMITAÇÃO e continuar — nunca contornar.
//
// Toda fonte é best-effort: informação parcial é registrada (campo zero-value
// + Limitations), nunca um erro que derrube a função. Hierarquia de
// dies/CCX/CCD além do que /proc/cpuinfo e sysfs expõem NUNCA é inferida —
// registrada como limitação quando ausente.

// NUMANode descreve um node NUMA observado: seus CPUs, memória e distâncias
// para os demais nodes.
type NUMANode struct {
	ID       int         `json:"id"`
	CPUs     []int       `json:"cpus,omitempty"`     // expandido do cpulist
	MemTotal int64       `json:"mem_total,omitempty"` // bytes (node*/meminfo MemTotal)
	MemFree  int64       `json:"mem_free,omitempty"`  // bytes (node*/meminfo MemFree)
	Distance map[int]int `json:"distance,omitempty"`  // distância até o node i (self=10)
}

// CPUTopology descreve a topologia por CPU lógica: package/socket (physical
// id), core id e os thread siblings (CPUs que compartilham o mesmo core).
type CPUTopology struct {
	ID             int   `json:"id"`
	PackageID      int   `json:"package_id,omitempty"` // physical id (0 é válido)
	CoreID         int   `json:"core_id,omitempty"`
	ThreadSiblings []int `json:"thread_siblings,omitempty"` // sysfs topology/thread_siblings_list
}

// TopologyInfo é a fotografia read-only da topologia (Fase 3). JSON
// serializável, espelhando o padrão do CPUInfo/MemoryInfo.
//
// Três camadas conceituais (padrão das Fases 1-2):
//   - Detected:  a topologia foi observada (≥ 1 node reportado);
//   - Available: o processo pode usar (mapeamento CPU→node resolvido);
//   - Usable:    a topologia por CPU foi resolvida.
//
// Synthetic=true indica que a topologia foi ASSUMIDA (fallback de 1 node)
// porque o sysfs não expôs nodes — observação ausente, não dado real.
type TopologyInfo struct {
	Nodes           []NUMANode         `json:"nodes,omitempty"`
	CPUToNode       map[int]int        `json:"cpu_to_node,omitempty"` // derivado dos cpulists
	CPUTopology     []CPUTopology      `json:"cpu_topology,omitempty"`
	Packages        int                `json:"packages,omitempty"`
	CoresPerPackage map[int]int        `json:"cores_per_package,omitempty"`
	Synthetic       bool               `json:"synthetic,omitempty"`

	Detected  bool `json:"detected"`
	Available bool `json:"available"`
	Usable    bool `json:"usable"`

	// Limitations registra o que não foi determinável em user space (e por quê).
	Limitations []string `json:"limitations,omitempty"`
}

// topologySources abstrai as fontes read-only para o probeTopology poder ser
// exercitado contra um ambiente fake nos testes. Produção usa
// defaultTopologySources.
type topologySources struct {
	readFile func(path string) string
	listDir  func(path string) []string
	numCPU   func() int
}

// ProbeTopology descobre a topologia de CPU/memória/NUMA. Estritamente
// read-only e best-effort: nenhuma fonte falha a função como um todo —
// informação parcial é registrada no modelo (zero-value + Limitations).
func ProbeTopology() TopologyInfo {
	return probeTopology(defaultTopologySources())
}

func probeTopology(s topologySources) TopologyInfo {
	info := TopologyInfo{
		CPUToNode:       map[int]int{},
		CoresPerPackage: map[int]int{},
	}

	// 1) NUMA nodes a partir do sysfs (node*/cpulist, meminfo, distance).
	nodes := discoverNUMANodes(s)

	// 2) Fallback: sem sysfs (ex: jaula bwrap sem /sys) → 1 node sintético,
	//    CPUs [0..numCPU), memória do /proc/meminfo, distance [10].
	if len(nodes) == 0 {
		nodes = fallbackSingleNode(s)
		info.Synthetic = true
		info.Limitations = append(info.Limitations,
			"topologia NUMA não exposta pelo sysfs (/sys/devices/system/node indisponível na jaula) — assumido 1 node, distance [10]")
	}
	info.Nodes = nodes

	// 3) CPU → node mapping derivado dos cpulists (fonte autoritativa e sem
	//    privilégio; o sysfs por-CPU /cpu*/node* é symlink para dir, ilegível
	//    como arquivo).
	for _, n := range nodes {
		for _, cpu := range n.CPUs {
			if _, ok := info.CPUToNode[cpu]; !ok {
				info.CPUToNode[cpu] = n.ID
			}
		}
	}

	// 4) Topologia por CPU: physical id/core id do /proc/cpuinfo (Fase 1) +
	//    thread siblings do sysfs topology.
	slots := parseCPUInfoPerCPU(s.readFile("/proc/cpuinfo"))
	info.CPUTopology = buildCPUTopology(s, slots, s.numCPU())
	info.Packages, info.CoresPerPackage = packagesFromSlots(slots)

	// Camadas detected/available/usable.
	info.Detected = len(info.Nodes) > 0
	info.Available = info.Detected && len(info.CPUToNode) > 0
	info.Usable = info.Available && len(info.CPUTopology) > 0

	// Limitações registradas — nunca contornadas (KERNEL ACCESS POLICY).
	if info.Detected && len(info.CPUToNode) == 0 {
		info.Limitations = append(info.Limitations,
			"mapeamento CPU→NUMA não determinável (cpulist dos nodes ausente ou vazio)")
	}
	memMissing, distMissing := false, false
	for _, n := range info.Nodes {
		if n.MemTotal == 0 {
			memMissing = true
		}
		if len(n.Distance) == 0 {
			distMissing = true
		}
	}
	if memMissing {
		info.Limitations = append(info.Limitations,
			"memória por node não exposta (node*/meminfo indisponível ou MemTotal zero)")
	}
	if distMissing {
		info.Limitations = append(info.Limitations,
			"distâncias NUMA não expostas (node*/distance indisponível)")
	}
	if info.Usable {
		siblingsMissing := false
		for _, t := range info.CPUTopology {
			if len(t.ThreadSiblings) == 0 {
				siblingsMissing = true
				break
			}
		}
		if siblingsMissing {
			info.Limitations = append(info.Limitations,
				"thread siblings não expostos para todas as CPUs (sysfs topology/thread_siblings_list indisponível)")
		}
	}
	if info.Packages == 0 && len(info.CPUTopology) > 0 {
		info.Limitations = append(info.Limitations,
			"packages/cores por package não determináveis (physical id/core id ausentes no /proc/cpuinfo — possível VM)")
	}
	if info.Detected {
		info.Limitations = append(info.Limitations,
			"hierarquia de dies/CCX/CCD não determinável em user space a partir de /proc/cpuinfo e sysfs (não exposta neste ambiente)")
	}

	return info
}

// =============================================================================
// Default (real) sources
// =============================================================================

func defaultTopologySources() topologySources {
	return topologySources{
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
	}
}

// =============================================================================
// Parsers (puros, unit-testáveis)
// =============================================================================

// parseCPUList expande um cpulist ("0-7,16-23", "0,1,2,3") em uma lista
// ordenada e de-duplicada de IDs de CPU. Entrada malformada é ignorada.
func parseCPUList(s string) []int {
	seen := map[int]bool{}
	var out []int
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		lo, hi, ok := strings.Cut(part, "-")
		var a, b int
		if ok {
			var errA, errB error
			a, errA = strconv.Atoi(strings.TrimSpace(lo))
			b, errB = strconv.Atoi(strings.TrimSpace(hi))
			if errA != nil || errB != nil || a > b {
				continue
			}
		} else {
			v, err := strconv.Atoi(part)
			if err != nil {
				continue
			}
			a, b = v, v
		}
		for c := a; c <= b; c++ {
			if !seen[c] {
				seen[c] = true
				out = append(out, c)
			}
		}
	}
	sort.Ints(out)
	return out
}

// parseNodeMemInfo extrai MemTotal e MemFree do node*/meminfo (kB → bytes).
// Aceita tanto o formato por-node ("Node 0 MemTotal:  32766340 kB") quanto o
// formato global do /proc/meminfo ("MemTotal: 16777216 kB").
func parseNodeMemInfo(out string) (memTotal, memFree int64) {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		// Índice da chave: node*/meminfo tem prefixo "Node N"; /proc/meminfo não.
		keyIdx := 0
		if len(fields) >= 3 && fields[0] == "Node" {
			keyIdx = 2
		}
		key := fields[keyIdx]
		if !strings.HasSuffix(key, ":") || keyIdx+1 >= len(fields) {
			continue
		}
		val, err := strconv.ParseInt(fields[keyIdx+1], 10, 64)
		if err != nil {
			continue
		}
		bytes := val * 1024 // kB → bytes
		switch key {
		case "MemTotal:":
			memTotal = bytes
		case "MemFree:":
			memFree = bytes
		}
	}
	return memTotal, memFree
}

// parseDistance lê o arquivo distance ("10" ou "10 21 20"): a i-ésima entrada
// é a distância deste node até o node i. Converte para map[nodeID]distance.
func parseDistance(out string) map[int]int {
	out = strings.TrimSpace(out)
	if out == "" {
		return nil
	}
	d := map[int]int{}
	out = strings.ReplaceAll(out, ",", " ")
	for i, f := range strings.Fields(out) {
		v, err := strconv.Atoi(f)
		if err != nil {
			continue
		}
		d[i] = v
	}
	return d
}

// cpuSlot captura physical id e core id de um processador do /proc/cpuinfo.
type cpuSlot struct {
	packageID int
	coreID    int
	hasPkg    bool // physical id presente (0 é válido, então "presente" importa)
}

// parseCPUInfoPerCPU extrai, por processador, o physical id (socket/package)
// e o core id do /proc/cpuinfo. Reutiliza a MESMA fonte da Fase 1, mas em
// granularidade por-CPU (a Fase 1 agrega só contagens).
func parseCPUInfoPerCPU(out string) map[int]cpuSlot {
	slots := map[int]cpuSlot{}
	var cur *cpuSlot
	curID := -1
	for _, line := range strings.Split(out, "\n") {
		fields := strings.SplitN(line, ":", 2)
		if len(fields) != 2 {
			continue
		}
		key := strings.TrimSpace(fields[0])
		val := strings.TrimSpace(fields[1])
		switch key {
		case "processor":
			if cur != nil {
				slots[curID] = *cur
			}
			id, err := strconv.Atoi(val)
			if err != nil {
				curID = -1
			} else {
				curID = id
			}
			cur = &cpuSlot{}
		case "physical id":
			if cur == nil {
				continue
			}
			if v, err := strconv.Atoi(val); err == nil {
				cur.packageID = v
				cur.hasPkg = true
			}
		case "core id":
			if cur == nil {
				continue
			}
			if v, err := strconv.Atoi(val); err == nil {
				cur.coreID = v
			}
		}
	}
	if cur != nil {
		slots[curID] = *cur
	}
	return slots
}

// buildCPUTopology monta a topologia por CPU: package/core do /proc/cpuinfo
// (fallback: zero) e thread siblings do sysfs topology (best-effort).
// Considera a união dos IDs do cpuinfo e de [0, numCPU).
func buildCPUTopology(s topologySources, slots map[int]cpuSlot, numCPU int) []CPUTopology {
	ids := map[int]bool{}
	for id := range slots {
		ids[id] = true
	}
	for i := 0; i < numCPU; i++ {
		ids[i] = true
	}
	sortedIDs := make([]int, 0, len(ids))
	for id := range ids {
		sortedIDs = append(sortedIDs, id)
	}
	sort.Ints(sortedIDs)

	topo := make([]CPUTopology, 0, len(sortedIDs))
	for _, id := range sortedIDs {
		t := CPUTopology{ID: id}
		if slot, ok := slots[id]; ok {
			if slot.hasPkg {
				t.PackageID = slot.packageID
			}
			t.CoreID = slot.coreID
		}
		t.ThreadSiblings = parseCPUList(s.readFile(
			fmt.Sprintf("/sys/devices/system/cpu/cpu%d/topology/thread_siblings_list", id)))
		topo = append(topo, t)
	}
	return topo
}

// packagesFromSlots calcula o número de packages e os cores por package a
// partir dos slots do /proc/cpuinfo. Sem physical id presente, devolve (0, {}).
func packagesFromSlots(slots map[int]cpuSlot) (int, map[int]int) {
	pkgs := map[int]map[int]bool{} // pkg → set de core ids
	hasAny := false
	for _, slot := range slots {
		if !slot.hasPkg {
			continue
		}
		hasAny = true
		if pkgs[slot.packageID] == nil {
			pkgs[slot.packageID] = map[int]bool{}
		}
		pkgs[slot.packageID][slot.coreID] = true
	}
	if !hasAny {
		return 0, map[int]int{}
	}
	perPkg := map[int]int{}
	for pkg, cores := range pkgs {
		perPkg[pkg] = len(cores)
	}
	return len(pkgs), perPkg
}

// discoverNUMANodes lê /sys/devices/system/node/node*: cpulist (CPUs),
// meminfo (MemTotal/MemFree) e distance. Ordena por ID. Best-effort — cada
// campo ausente fica zero/vazio e vira limitação no chamador.
func discoverNUMANodes(s topologySources) []NUMANode {
	const base = "/sys/devices/system/node"
	var nodes []NUMANode
	for _, name := range s.listDir(base) {
		if !strings.HasPrefix(name, "node") {
			continue
		}
		id, err := strconv.Atoi(strings.TrimPrefix(name, "node"))
		if err != nil {
			continue
		}
		dir := base + "/" + name
		n := NUMANode{ID: id}
		n.CPUs = parseCPUList(s.readFile(dir + "/cpulist"))
		n.MemTotal, n.MemFree = parseNodeMemInfo(s.readFile(dir + "/meminfo"))
		n.Distance = parseDistance(s.readFile(dir + "/distance"))
		nodes = append(nodes, n)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	return nodes
}

// fallbackSingleNode constrói um node único sintético quando o sysfs não
// expõe topologia (jaula bwrap sem /sys): CPUs [0..numCPU), memória do
// /proc/meminfo (via parseNodeMemInfo, que aceita o formato global) e
// distance [10] (convenção do kernel: self-distance = 10).
func fallbackSingleNode(s topologySources) []NUMANode {
	numCPU := s.numCPU()
	cpus := make([]int, 0, numCPU)
	for i := 0; i < numCPU; i++ {
		cpus = append(cpus, i)
	}
	total, free := parseNodeMemInfo(s.readFile("/proc/meminfo"))
	return []NUMANode{{
		ID:       0,
		CPUs:     cpus,
		MemTotal: total,
		MemFree:  free,
		Distance: map[int]int{0: 10},
	}}
}
