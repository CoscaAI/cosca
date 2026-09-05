package compute

import (
	"encoding/json"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// =============================================================================
// Topology Probe — real machine
// =============================================================================

func TestProbeTopology_RealMachine(t *testing.T) {
	info := ProbeTopology()

	// A função NUNCA falha como um todo: sempre ≥ 1 node (real ou fallback).
	if len(info.Nodes) == 0 {
		t.Error("Nodes should never be empty (fallback single node expected)")
	}
	if !info.Detected {
		t.Error("Detected should be true (≥ 1 node reported)")
	}
	// Packages é derivado de /proc/cpuinfo (physical id) — não existe no
	// Windows, onde a topologia cai para o fallback de 1 node sem packages.
	if runtime.GOOS == "linux" && info.Packages == 0 {
		t.Error("Packages should be > 0 on a real machine (physical id present)")
	}
	if runtime.GOOS == "linux" {
		// Nenhum CPU fora do range lógico observado.
		for _, n := range info.Nodes {
			for _, cpu := range n.CPUs {
				if cpu >= runtime.NumCPU() && runtime.NumCPU() > 0 {
					t.Errorf("node %d reports CPU %d >= runtime.NumCPU() %d", n.ID, cpu, runtime.NumCPU())
				}
			}
		}
	}
	// Invariante estrutural: toda CPU mapeada tem node válido e vice-versa.
	for _, n := range info.Nodes {
		for _, cpu := range n.CPUs {
			if got, ok := info.CPUToNode[cpu]; !ok || got != n.ID {
				t.Errorf("CPUToNode[%d] = %d (ok=%v), want %d (node %d)", cpu, got, ok, n.ID, n.ID)
			}
		}
	}
}

func TestProbeTopology_SnapshotIncludesTopology(t *testing.T) {
	snap := ProbeHardware()
	if len(snap.Topology.Nodes) == 0 {
		t.Error("ProbeHardware should populate topology discovery (≥ 1 node)")
	}
	if !snap.Topology.Detected {
		t.Error("snapshot topology should be detected")
	}
}

func TestProbeTopology_Concurrent(t *testing.T) {
	// Validação de concorrência do probe (o race detector é o juiz real).
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = ProbeTopology()
		}()
	}
	wg.Wait()
}

// =============================================================================
// Topology Probe — fake environments
// =============================================================================

func fakeTopologySourcesEmpty() topologySources {
	return topologySources{
		readFile: func(string) string { return "" },
		listDir:  func(string) []string { return nil },
		numCPU:   func() int { return 0 },
	}
}

// fakeMultiNode simula um sistema com 2 NUMA nodes (16 CPUs: 0-7 e 8-15),
// cada um com meminfo próprio e distâncias assimétricas (10/21).
func fakeMultiNode() topologySources {
	s := fakeTopologySourcesEmpty()
	s.numCPU = func() int { return 16 }
	s.listDir = func(path string) []string {
		if path == "/sys/devices/system/node" {
			return []string{"node0", "node1"}
		}
		return nil
	}
	s.readFile = func(path string) string {
		switch path {
		case "/sys/devices/system/node/node0/cpulist":
			return "0-7"
		case "/sys/devices/system/node/node0/meminfo":
			return "Node 0 MemTotal:    16777216 kB\nNode 0 MemFree:      8388608 kB\n"
		case "/sys/devices/system/node/node0/distance":
			return "10 21"
		case "/sys/devices/system/node/node1/cpulist":
			return "8-15"
		case "/sys/devices/system/node/node1/meminfo":
			return "Node 1 MemTotal:    16777216 kB\nNode 1 MemFree:      4194304 kB\n"
		case "/sys/devices/system/node/node1/distance":
			return "21 10"
		case "/proc/cpuinfo":
			return fakeTopologyCPUInfo
		case "/sys/devices/system/cpu/cpu0/topology/thread_siblings_list":
			return "0,8"
		case "/sys/devices/system/cpu/cpu1/topology/thread_siblings_list":
			return "1,9"
		case "/sys/devices/system/cpu/cpu2/topology/thread_siblings_list":
			return "2,10"
		case "/sys/devices/system/cpu/cpu3/topology/thread_siblings_list":
			return "3,11"
		case "/sys/devices/system/cpu/cpu4/topology/thread_siblings_list":
			return "4,12"
		case "/sys/devices/system/cpu/cpu5/topology/thread_siblings_list":
			return "5,13"
		case "/sys/devices/system/cpu/cpu6/topology/thread_siblings_list":
			return "6,14"
		case "/sys/devices/system/cpu/cpu7/topology/thread_siblings_list":
			return "7,15"
		case "/sys/devices/system/cpu/cpu8/topology/thread_siblings_list":
			return "0,8"
		case "/sys/devices/system/cpu/cpu9/topology/thread_siblings_list":
			return "1,9"
		case "/sys/devices/system/cpu/cpu10/topology/thread_siblings_list":
			return "2,10"
		case "/sys/devices/system/cpu/cpu11/topology/thread_siblings_list":
			return "3,11"
		case "/sys/devices/system/cpu/cpu12/topology/thread_siblings_list":
			return "4,12"
		case "/sys/devices/system/cpu/cpu13/topology/thread_siblings_list":
			return "5,13"
		case "/sys/devices/system/cpu/cpu14/topology/thread_siblings_list":
			return "6,14"
		case "/sys/devices/system/cpu/cpu15/topology/thread_siblings_list":
			return "7,15"
		}
		return ""
	}
	return s
}

// fakeTopologyCPUInfo: 16 processadores, 2 packages (físico), 8 cores físicos,
// com thread siblings espelhados (0↔8, 1↔9, ...).
const fakeTopologyCPUInfo = `processor	: 0
physical id	: 0
core id		: 0
processor	: 1
physical id	: 0
core id		: 1
processor	: 2
physical id	: 0
core id		: 2
processor	: 3
physical id	: 0
core id		: 3
processor	: 4
physical id	: 0
core id		: 4
processor	: 5
physical id	: 0
core id		: 5
processor	: 6
physical id	: 0
core id		: 6
processor	: 7
physical id	: 0
core id		: 7
processor	: 8
physical id	: 1
core id		: 0
processor	: 9
physical id	: 1
core id		: 1
processor	: 10
physical id	: 1
core id		: 2
processor	: 11
physical id	: 1
core id		: 3
processor	: 12
physical id	: 1
core id		: 4
processor	: 13
physical id	: 1
core id		: 5
processor	: 14
physical id	: 1
core id		: 6
processor	: 15
physical id	: 1
core id		: 7
`

func TestProbeTopology_FakeMultiNode(t *testing.T) {
	info := probeTopology(fakeMultiNode())

	if len(info.Nodes) != 2 {
		t.Fatalf("nodes = %d, want 2", len(info.Nodes))
	}
	if info.Synthetic {
		t.Error("Synthetic should be false (sysfs real nodes observed)")
	}
	node0, node1 := info.Nodes[0], info.Nodes[1]
	if node0.ID != 0 || node1.ID != 1 {
		t.Errorf("node ids = %d/%d, want 0/1", node0.ID, node1.ID)
	}
	if len(node0.CPUs) != 8 || node0.CPUs[0] != 0 || node0.CPUs[7] != 7 {
		t.Errorf("node0 CPUs = %v, want 0-7", node0.CPUs)
	}
	if len(node1.CPUs) != 8 || node1.CPUs[0] != 8 || node1.CPUs[7] != 15 {
		t.Errorf("node1 CPUs = %v, want 8-15", node1.CPUs)
	}
	if node0.MemTotal != 16777216*1024 || node0.MemFree != 8388608*1024 {
		t.Errorf("node0 mem = %d/%d, want %d/%d",
			node0.MemTotal, node0.MemFree, 16777216*1024, 8388608*1024)
	}
	if node1.MemTotal != 16777216*1024 || node1.MemFree != 4194304*1024 {
		t.Errorf("node1 mem = %d/%d, want %d/%d",
			node1.MemTotal, node1.MemFree, 16777216*1024, 4194304*1024)
	}
	if node0.Distance[0] != 10 || node0.Distance[1] != 21 {
		t.Errorf("node0 distance = %v, want {0:10 1:21}", node0.Distance)
	}
	if node1.Distance[0] != 21 || node1.Distance[1] != 10 {
		t.Errorf("node1 distance = %v, want {0:21 1:10}", node1.Distance)
	}

	// CPU → node mapping.
	if info.CPUToNode[0] != 0 || info.CPUToNode[7] != 0 {
		t.Errorf("CPUToNode 0/7 = %d/%d, want 0/0", info.CPUToNode[0], info.CPUToNode[7])
	}
	if info.CPUToNode[8] != 1 || info.CPUToNode[15] != 1 {
		t.Errorf("CPUToNode 8/15 = %d/%d, want 1/1", info.CPUToNode[8], info.CPUToNode[15])
	}
	if len(info.CPUToNode) != 16 {
		t.Errorf("CPUToNode size = %d, want 16", len(info.CPUToNode))
	}

	// Topologia por CPU: packages, cores por package, thread siblings.
	if info.Packages != 2 {
		t.Errorf("packages = %d, want 2", info.Packages)
	}
	if info.CoresPerPackage[0] != 8 || info.CoresPerPackage[1] != 8 {
		t.Errorf("cores per package = %v, want {0:8 1:8}", info.CoresPerPackage)
	}
	if len(info.CPUTopology) != 16 {
		t.Fatalf("cpu topology = %d entries, want 16", len(info.CPUTopology))
	}
	if got := info.CPUTopology[0]; got.PackageID != 0 || got.CoreID != 0 || len(got.ThreadSiblings) != 2 || got.ThreadSiblings[0] != 0 || got.ThreadSiblings[1] != 8 {
		t.Errorf("cpu0 topo = %+v, want pkg 0 core 0 siblings [0 8]", got)
	}
	if got := info.CPUTopology[15]; got.PackageID != 1 || got.CoreID != 7 || len(got.ThreadSiblings) != 2 || got.ThreadSiblings[0] != 7 || got.ThreadSiblings[1] != 15 {
		t.Errorf("cpu15 topo = %+v, want pkg 1 core 7 siblings [7 15]", got)
	}

	if !info.Detected || !info.Available || !info.Usable {
		t.Errorf("tiers = %v/%v/%v, want true/true/true",
			info.Detected, info.Available, info.Usable)
	}
}

func TestProbeTopology_FakeNoNUMA(t *testing.T) {
	// Sem sysfs (ex: jaula bwrap sem /sys) → fallback de 1 node sintético.
	s := fakeTopologySourcesEmpty()
	s.numCPU = func() int { return 16 }
	s.readFile = func(path string) string {
		switch path {
		case "/proc/meminfo":
			return "MemTotal:       33554432 kB\nMemFree:        25165824 kB\n"
		case "/proc/cpuinfo":
			return fakeTopologyCPUInfo
		}
		return ""
	}

	info := probeTopology(s)
	if len(info.Nodes) != 1 {
		t.Fatalf("nodes = %d, want 1 (fallback)", len(info.Nodes))
	}
	if !info.Synthetic {
		t.Error("Synthetic should be true (sysfs absent)")
	}
	n := info.Nodes[0]
	if n.ID != 0 || len(n.CPUs) != 16 || n.CPUs[0] != 0 || n.CPUs[15] != 15 {
		t.Errorf("fallback node = %+v, want id 0 CPUs 0-15", n)
	}
	if n.MemTotal != 33554432*1024 || n.MemFree != 25165824*1024 {
		t.Errorf("fallback mem = %d/%d, want %d/%d (from /proc/meminfo)",
			n.MemTotal, n.MemFree, 33554432*1024, 25165824*1024)
	}
	if n.Distance[0] != 10 || len(n.Distance) != 1 {
		t.Errorf("fallback distance = %v, want {0:10}", n.Distance)
	}
	if info.CPUToNode[0] != 0 || info.CPUToNode[15] != 0 {
		t.Errorf("CPUToNode = %v, want all → 0", info.CPUToNode)
	}
	// A topologia por CPU continua derivável do /proc/cpuinfo.
	if info.Packages != 2 {
		t.Errorf("packages = %d, want 2 (cpuinfo ainda legível)", info.Packages)
	}
	if !info.Detected || !info.Available || !info.Usable {
		t.Errorf("tiers = %v/%v/%v, want true/true/true",
			info.Detected, info.Available, info.Usable)
	}
	found := false
	for _, l := range info.Limitations {
		if strings.Contains(l, "sysfs") {
			found = true
		}
	}
	if !found {
		t.Errorf("limitation about missing sysfs should be recorded: %v", info.Limitations)
	}
}

func TestProbeTopology_PartialInfo(t *testing.T) {
	// sysfs presente mas cpulist/meminfo/distance ausentes → campos vazios e
	// limitações registradas; a função NUNCA falha.
	s := fakeTopologySourcesEmpty()
	s.numCPU = func() int { return 4 }
	s.listDir = func(path string) []string {
		if path == "/sys/devices/system/node" {
			return []string{"node0"}
		}
		return nil
	}
	info := probeTopology(s)

	if len(info.Nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(info.Nodes))
	}
	if info.Synthetic {
		t.Error("Synthetic should be false (sysfs dir exists)")
	}
	n := info.Nodes[0]
	if len(n.CPUs) != 0 {
		t.Errorf("node CPUs = %v, want empty (cpulist ausente)", n.CPUs)
	}
	if n.MemTotal != 0 || n.MemFree != 0 {
		t.Errorf("node mem = %d/%d, want 0/0 (meminfo ausente)", n.MemTotal, n.MemFree)
	}
	if len(n.Distance) != 0 {
		t.Errorf("node distance = %v, want empty (distance ausente)", n.Distance)
	}
	// Sem cpulist, não há mapeamento → Available false, mas sem crash.
	if info.Detected != true {
		t.Error("Detected should be true (node reportado)")
	}
	if info.Available {
		t.Error("Available should be false (sem mapeamento CPU→node)")
	}
	foundLimitation := false
	for _, l := range info.Limitations {
		if strings.Contains(l, "cpulist") || strings.Contains(l, "meminfo") || strings.Contains(l, "distance") {
			foundLimitation = true
		}
	}
	if !foundLimitation {
		t.Errorf("limitations should mention missing sources: %v", info.Limitations)
	}
}

func TestProbeTopology_Fallback_EmptySources(t *testing.T) {
	// Nenhuma fonte disponível: função NUNCA falha; fallback 1 node + limitation.
	info := probeTopology(fakeTopologySourcesEmpty())
	if len(info.Nodes) != 1 {
		t.Fatalf("nodes = %d, want 1 (fallback)", len(info.Nodes))
	}
	if !info.Synthetic {
		t.Error("Synthetic should be true")
	}
	if info.Nodes[0].ID != 0 || len(info.Nodes[0].CPUs) != 0 {
		t.Errorf("fallback node = %+v, want id 0 sem CPUs (numCPU=0)", info.Nodes[0])
	}
	if len(info.Limitations) == 0 {
		t.Error("limitations should be recorded in empty environment")
	}
}

// =============================================================================
// Parsers
// =============================================================================

func TestParseCPUList(t *testing.T) {
	tests := []struct {
		in   string
		want []int
	}{
		{"0-7", []int{0, 1, 2, 3, 4, 5, 6, 7}},
		{"0-7,16-23", []int{0, 1, 2, 3, 4, 5, 6, 7, 16, 17, 18, 19, 20, 21, 22, 23}},
		{"0,1,2,3", []int{0, 1, 2, 3}},
		{"0", []int{0}},
		{" 0 - 3 , 8 - 11 ", []int{0, 1, 2, 3, 8, 9, 10, 11}},
		{"0-3,0-3", []int{0, 1, 2, 3}},        // de-dupe
		{"4-6,4-6,0", []int{0, 4, 5, 6}},      // desordenado + duplicado
		{"", nil},
		{"junk", nil},
		{"5-2", nil}, // range invertido
		{"a-b", nil},
		{"0,,2", []int{0, 2}}, // entradas vazias ignoradas
	}
	for _, tt := range tests {
		got := parseCPUList(tt.in)
		if len(got) != len(tt.want) {
			t.Errorf("parseCPUList(%q) = %v, want %v", tt.in, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("parseCPUList(%q) = %v, want %v", tt.in, got, tt.want)
				break
			}
		}
	}
}

func TestParseNodeMemInfo(t *testing.T) {
	// Formato por-node do sysfs.
	nodeOut := "Node 0 MemTotal:    16777216 kB\nNode 0 MemFree:      8388608 kB\nNode 0 MemUsed:      8388608 kB\n"
	total, free := parseNodeMemInfo(nodeOut)
	if total != 16777216*1024 || free != 8388608*1024 {
		t.Errorf("node meminfo = %d/%d, want %d/%d", total, free, 16777216*1024, 8388608*1024)
	}

	// Formato global do /proc/meminfo também aceito (fallback).
	procOut := "MemTotal:       33554432 kB\nMemFree:        25165824 kB\nMemAvailable:   26214400 kB\n"
	total2, free2 := parseNodeMemInfo(procOut)
	if total2 != 33554432*1024 || free2 != 25165824*1024 {
		t.Errorf("proc meminfo = %d/%d, want %d/%d", total2, free2, 33554432*1024, 25165824*1024)
	}

	// Vazio → zeros, sem crash.
	if total3, free3 := parseNodeMemInfo(""); total3 != 0 || free3 != 0 {
		t.Errorf("empty meminfo = %d/%d, want 0/0", total3, free3)
	}
}

func TestParseDistance(t *testing.T) {
	tests := []struct {
		in   string
		want map[int]int
	}{
		{"10", map[int]int{0: 10}},
		{"10 21", map[int]int{0: 10, 1: 21}},
		{"10 21 20", map[int]int{0: 10, 1: 21, 2: 20}},
		{"10,21", map[int]int{0: 10, 1: 21}},
		{"", nil},
		{"junk", nil},
	}
	for _, tt := range tests {
		got := parseDistance(tt.in)
		if len(got) != len(tt.want) {
			t.Errorf("parseDistance(%q) = %v, want %v", tt.in, got, tt.want)
			continue
		}
		for k, v := range tt.want {
			if got[k] != v {
				t.Errorf("parseDistance(%q)[%d] = %d, want %d", tt.in, k, got[k], v)
			}
		}
	}
}

func TestParseCPUInfoPerCPU(t *testing.T) {
	slots := parseCPUInfoPerCPU(fakeTopologyCPUInfo)
	if len(slots) != 16 {
		t.Fatalf("slots = %d, want 16", len(slots))
	}
	if slots[0].packageID != 0 || slots[0].coreID != 0 || !slots[0].hasPkg {
		t.Errorf("slot 0 = %+v, want pkg 0 core 0", slots[0])
	}
	if slots[7].packageID != 0 || slots[7].coreID != 7 {
		t.Errorf("slot 7 = %+v, want pkg 0 core 7", slots[7])
	}
	if slots[8].packageID != 1 || slots[8].coreID != 0 {
		t.Errorf("slot 8 = %+v, want pkg 1 core 0", slots[8])
	}
	if slots[15].packageID != 1 || slots[15].coreID != 7 {
		t.Errorf("slot 15 = %+v, want pkg 1 core 7", slots[15])
	}

	// Sem physical id → hasPkg false (package 0 ambíguo, não reportado como real).
	noPkg := "processor	: 0\ncore id		: 0\nprocessor	: 1\ncore id		: 1\n"
	slots2 := parseCPUInfoPerCPU(noPkg)
	if slots2[0].hasPkg || slots2[0].packageID != 0 {
		t.Errorf("slot 0 sem physical id: hasPkg=%v, want false", slots2[0].hasPkg)
	}
	if slots2[1].coreID != 1 {
		t.Errorf("slot 1 core = %d, want 1", slots2[1].coreID)
	}
}

func TestBuildCPUTopology_FallbackThreads(t *testing.T) {
	// sysfs de thread siblings ausente → ThreadSiblings vazio, sem crash.
	s := fakeTopologySourcesEmpty()
	s.numCPU = func() int { return 2 }
	slots := parseCPUInfoPerCPU("processor	: 0\nphysical id	: 0\ncore id		: 0\nprocessor	: 1\nphysical id	: 0\ncore id		: 1\n")
	topo := buildCPUTopology(s, slots, s.numCPU())
	if len(topo) != 2 {
		t.Fatalf("topo = %d entries, want 2", len(topo))
	}
	if topo[0].ID != 0 || topo[0].PackageID != 0 || topo[0].CoreID != 0 {
		t.Errorf("topo[0] = %+v", topo[0])
	}
	if len(topo[0].ThreadSiblings) != 0 {
		t.Errorf("topo[0].ThreadSiblings = %v, want empty", topo[0].ThreadSiblings)
	}
}

func TestPackagesFromSlots(t *testing.T) {
	slots := parseCPUInfoPerCPU(fakeTopologyCPUInfo)
	pkgs, perPkg := packagesFromSlots(slots)
	if pkgs != 2 {
		t.Errorf("packages = %d, want 2", pkgs)
	}
	if perPkg[0] != 8 || perPkg[1] != 8 {
		t.Errorf("cores per package = %v, want {0:8 1:8}", perPkg)
	}

	// Sem physical id → (0, {}).
	noPkg := parseCPUInfoPerCPU("processor	: 0\ncore id		: 0\n")
	pkgs2, perPkg2 := packagesFromSlots(noPkg)
	if pkgs2 != 0 || len(perPkg2) != 0 {
		t.Errorf("no physical id: packages=%d perPkg=%v, want 0/{}", pkgs2, perPkg2)
	}
}

// =============================================================================
// Invariants
// =============================================================================

func TestProbeTopology_Invariant_NodeCPUsSubsetOfTotal(t *testing.T) {
	// CPUs de cada node ⊆ CPUs totais (união de todos os nodes).
	info := probeTopology(fakeMultiNode())

	total := map[int]bool{}
	for _, n := range info.Nodes {
		for _, cpu := range n.CPUs {
			total[cpu] = true
		}
	}
	for _, n := range info.Nodes {
		for _, cpu := range n.CPUs {
			if !total[cpu] {
				t.Errorf("node %d CPU %d not in total set (impossible)", n.ID, cpu)
			}
		}
	}
	// A união é exatamente o range lógico observado (0..15 aqui).
	if len(total) != 16 {
		t.Errorf("total distinct CPUs = %d, want 16", len(total))
	}
}

func TestProbeTopology_Invariant_CPUToNodeConsistent(t *testing.T) {
	// Todo CPU mapeado pertence a exatamente um node cujo cpulist o contém.
	info := probeTopology(fakeMultiNode())
	for cpu, node := range info.CPUToNode {
		contained := false
		for _, n := range info.Nodes {
			if n.ID == node {
				for _, c := range n.CPUs {
					if c == cpu {
						contained = true
					}
				}
			}
		}
		if !contained {
			t.Errorf("CPU %d mapped to node %d which does not list it", cpu, node)
		}
	}
}

// =============================================================================
// Serialization
// =============================================================================

func TestTopologyInfo_JSONSerialization(t *testing.T) {
	info := TopologyInfo{
		Nodes: []NUMANode{
			{ID: 0, CPUs: []int{0, 1, 2, 3}, MemTotal: 17179869184, MemFree: 8589934592,
				Distance: map[int]int{0: 10, 1: 21}},
			{ID: 1, CPUs: []int{4, 5, 6, 7}, MemTotal: 17179869184, MemFree: 4294967296,
				Distance: map[int]int{0: 21, 1: 10}},
		},
		CPUToNode:       map[int]int{0: 0, 1: 0, 2: 0, 3: 0, 4: 1, 5: 1, 6: 1, 7: 1},
		CPUTopology:     []CPUTopology{{ID: 0, PackageID: 1, CoreID: 2, ThreadSiblings: []int{0, 4}}},
		Packages:        2,
		CoresPerPackage: map[int]int{0: 4, 1: 4},
		Detected:        true,
		Available:       true,
		Usable:          true,
		Limitations:     []string{"hierarquia de dies/CCX/CCD não determinável"},
	}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)
	for _, want := range []string{
		`"nodes"`, `"id":0`, `"cpus":[0,1,2,3]`, `"mem_total":17179869184`,
		`"distance":{"0":10,"1":21}`,
		`"cpu_to_node":{"0":0,"1":0,"2":0,"3":0,"4":1,"5":1,"6":1,"7":1}`,
		`"cpu_topology"`, `"package_id":1`, `"core_id":2`, `"thread_siblings":[0,4]`,
		`"packages":2`, `"cores_per_package":{"0":4,"1":4}`,
		`"detected":true`, `"available":true`, `"usable":true`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("JSON missing %q: %s", want, s)
		}
	}
}

func TestTopologyInfo_JSONZeroValue(t *testing.T) {
	// Zero-value deve serializar sem error e com os campos required presentes.
	info := TopologyInfo{}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal zero value: %v", err)
	}
	for _, want := range []string{`"detected":false`, `"available":false`, `"usable":false`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("zero JSON missing %q: %s", want, string(data))
		}
	}
}
