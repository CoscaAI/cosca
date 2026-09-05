package compute

import (
	"encoding/json"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// =============================================================================
// Environment Probe — real machine
// =============================================================================

func TestProbeEnvironment_RealMachine(t *testing.T) {
	info := ProbeEnvironment()

	// A função nunca falha; GOOS/GOARCH sempre presentes.
	if info.GOOS == "" || info.GOARCH == "" {
		t.Errorf("GOOS/GOARCH = %q/%q, want non-empty", info.GOOS, info.GOARCH)
	}
	// Detected é derivado de sinais Linux (/proc, /sys, cgroup) — no Windows
	// nenhum sinal existe em user space, então Detected=false é o comportamento
	// honesto da plataforma. A asserção de detecção é Linux-only.
	if runtime.GOOS == "linux" && info.Detected != true {
		t.Error("Detected should be true (pelo menos um sinal observado)")
	}
	// Na jaula bwrap (COSCA_JAILED=1), Type deve refletir isso.
	if info.InJail && info.Type != EnvJail {
		t.Errorf("in jail but Type = %q, want jail", info.Type)
	}
	// Se /sys está montado, topologia completa é possível.
	if info.SysfsMounted && len(info.Types) == 0 && info.Type != EnvBareMetal {
		t.Errorf("sysfs mounted on bare metal but Type = %q", info.Type)
	}
}

func TestProbeEnvironment_SnapshotIncludesEnvironment(t *testing.T) {
	snap := ProbeHardware()
	if snap.Environment.GOARCH == "" {
		t.Error("snapshot should include environment discovery (GOARCH)")
	}
	// Detected só é verdadeiro quando há sinais Linux observáveis (ver acima).
	if runtime.GOOS == "linux" && !snap.Environment.Detected {
		t.Error("snapshot environment should be detected")
	}
}

func TestProbeEnvironment_Concurrent(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = ProbeEnvironment()
		}()
	}
	wg.Wait()
}

// =============================================================================
// Environment Probe — fake environments
// =============================================================================

func fakeEnvSources() envSources {
	return envSources{
		readFile: func(string) string { return "" },
		getenv:   func(string) string { return "" },
		numCPU:   func() int { return 16 },
		goos:     func() string { return "linux" },
		goarch:   func() string { return "amd64" },
	}
}

// fakeJail simula a jaula bwrap: COSCA_JAILED=1, sem /sys, sem cgroup de container.
func fakeJail() envSources {
	s := fakeEnvSources()
	s.getenv = func(k string) string {
		if k == "COSCA_JAILED" {
			return "1"
		}
		return ""
	}
	return s
}

// fakeContainer simula um container: cgroup com docker + cpuset restrito + sysfs v2.
func fakeContainer() envSources {
	s := fakeEnvSources()
	s.readFile = func(path string) string {
		switch path {
		case "/proc/self/cgroup":
			return "0::/system.slice/docker-abc123.scope"
		case "/sys/fs/cgroup/cgroup.controllers":
			return "cpuset cpu io memory\n"
		case "/proc/self/status":
			return "Cpus_allowed_list:\t0-7\n"
		}
		return ""
	}
	return s
}

// fakeVM simula uma VM: flag hypervisor no cpuinfo, sem container, sem jail.
func fakeVM() envSources {
	s := fakeEnvSources()
	s.readFile = func(path string) string {
		switch path {
		case "/proc/cpuinfo":
			return "processor\t: 0\nvendor_id\t: AuthenticAMD\nflags\t\t: fpu avx2 hypervisor\n"
		}
		return ""
	}
	return s
}

// fakeBareMetal simula host real: sysfs de topologia presente, sem sinais de VM/container.
func fakeBareMetal() envSources {
	s := fakeEnvSources()
	s.readFile = func(path string) string {
		switch path {
		case "/sys/devices/system/node/node0/cpulist":
			return "0-15"
		case "/proc/self/cgroup":
			return "0::/\n"
		}
		return ""
	}
	return s
}

func TestProbeEnvironment_FakeJail(t *testing.T) {
	info := probeEnvironment(fakeJail())

	if !info.InJail {
		t.Error("InJail should be true (COSCA_JAILED=1)")
	}
	if info.Type != EnvJail {
		t.Errorf("Type = %q, want jail", info.Type)
	}
	if info.SysfsMounted {
		t.Error("SysfsMounted should be false (jaula sem /sys)")
	}
	if !info.Detected || !info.Available || !info.Usable {
		t.Errorf("tiers = %v/%v/%v, want true/true/true", info.Detected, info.Available, info.Usable)
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

func TestProbeEnvironment_FakeContainer(t *testing.T) {
	info := probeEnvironment(fakeContainer())

	if !info.InContainer {
		t.Error("InContainer should be true (docker cgroup)")
	}
	if info.Type != EnvContainer {
		t.Errorf("Type = %q, want container", info.Type)
	}
	if info.CgroupVersion != "v2" {
		t.Errorf("CgroupVersion = %q, want v2", info.CgroupVersion)
	}
	if !strings.Contains(info.CgroupControllers, "memory") {
		t.Errorf("controllers = %q, want memory included", info.CgroupControllers)
	}
	if info.EffectiveCPUs != "0-7" {
		t.Errorf("EffectiveCPUs = %q, want 0-7 (cpuset restrito)", info.EffectiveCPUs)
	}
	// Cpuset restrito → limitação registrada (effective < visible).
	found := false
	for _, l := range info.Limitations {
		if strings.Contains(l, "cpuset") {
			found = true
		}
	}
	if !found {
		t.Errorf("limitation about restricted cpuset should be recorded: %v", info.Limitations)
	}
}

func TestProbeEnvironment_FakeVM(t *testing.T) {
	info := probeEnvironment(fakeVM())

	if !info.InVM {
		t.Error("InVM should be true (hypervisor flag)")
	}
	if info.Type != EnvVM {
		t.Errorf("Type = %q, want virtualized", info.Type)
	}
	if info.InContainer || info.InJail {
		t.Error("InContainer/InJail should be false")
	}
}

func TestProbeEnvironment_FakeBareMetal(t *testing.T) {
	info := probeEnvironment(fakeBareMetal())

	if info.InJail || info.InContainer || info.InVM {
		t.Error("no jail/container/vm signals expected on bare metal")
	}
	if !info.SysfsMounted {
		t.Error("SysfsMounted should be true (node0/cpulist present)")
	}
	if info.Type != EnvBareMetal {
		t.Errorf("Type = %q, want bare-metal", info.Type)
	}
}

func TestProbeEnvironment_Unknown(t *testing.T) {
	// Nenhum sinal: unknown, mas a função nunca falha.
	info := probeEnvironment(fakeEnvSources())

	if info.Detected {
		t.Error("Detected should be false (nenhum sinal)")
	}
	if info.Type != EnvUnknown {
		t.Errorf("Type = %q, want unknown", info.Type)
	}
	if len(info.Limitations) == 0 {
		t.Error("limitations should be recorded when unknown")
	}
}

// =============================================================================
// Parsers
// =============================================================================

func TestDetectContainer(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"0::/system.slice/docker-abc.scope", true},
		{"1:name=systemd:/kubepods/burstable/pod123", true},
		{"0::/system.slice/podman-xyz.scope", true},
		{"1:name=systemd:/lxc/container1", true},
		{"0::/", false},
		{"", false},
		{"1:name=systemd:/user.slice/user-1000.slice", false},
	}
	for _, tt := range tests {
		if got := detectContainer(tt.in); got != tt.want {
			t.Errorf("detectContainer(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestParseCpusAllowed(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"Cpus_allowed_list:\t0-7\n", "0-7"},
		{"Cpus_allowed_list:\t0,2,4\n", "0,2,4"},
		{"Cpus_allowed_list:\t0-15\nMems_allowed_list:\t0\n", "0-15"},
		{"no cpus here", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := parseCpusAllowed(tt.in); got != tt.want {
			t.Errorf("parseCpusAllowed(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCpusAllowedAll(t *testing.T) {
	s := fakeEnvSources()
	if got := cpusAllowedAll(s); got != "0-15" {
		t.Errorf("cpusAllowedAll(16) = %q, want 0-15", got)
	}
	s1 := fakeEnvSources()
	s1.numCPU = func() int { return 1 }
	if got := cpusAllowedAll(s1); got != "0" {
		t.Errorf("cpusAllowedAll(1) = %q, want 0", got)
	}
	s0 := fakeEnvSources()
	s0.numCPU = func() int { return 0 }
	if got := cpusAllowedAll(s0); got != "" {
		t.Errorf("cpusAllowedAll(0) = %q, want empty", got)
	}
}

func TestProbeEnvironment_EffectiveFull(t *testing.T) {
	// Cpuset = tudo (0-15) com 16 CPUs → sem limitação de cpuset.
	s := fakeEnvSources()
	s.readFile = func(path string) string {
		if path == "/proc/self/status" {
			return "Cpus_allowed_list:\t0-15\n"
		}
		return ""
	}
	info := probeEnvironment(s)
	if info.EffectiveCPUs != "0-15" {
		t.Errorf("EffectiveCPUs = %q, want 0-15", info.EffectiveCPUs)
	}
	for _, l := range info.Limitations {
		if strings.Contains(l, "cpuset") {
			t.Errorf("no cpuset limitation expected when full: %v", info.Limitations)
		}
	}
}

// =============================================================================
// Serialization
// =============================================================================

func TestEnvironmentInfo_JSONSerialization(t *testing.T) {
	info := EnvironmentInfo{
		Type:              EnvContainer,
		Types:             []EnvironmentType{EnvContainer},
		InContainer:       true,
		SysfsMounted:      true,
		CgroupVersion:     "v2",
		CgroupControllers: "cpuset cpu io memory",
		EffectiveCPUs:     "0-7",
		GOOS:              "linux",
		GOARCH:            "amd64",
		Detected:          true,
		Available:         true,
		Usable:            true,
		Limitations:       []string{"cpuset efetivo restrito (0-7)"},
	}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)
	for _, want := range []string{
		`"type":"container"`, `"in_container":true`, `"sysfs_mounted":true`,
		`"cgroup_version":"v2"`, `"effective_cpus":"0-7"`, `"goos":"linux"`,
		`"goarch":"amd64"`, `"detected":true`, `"available":true`, `"usable":true`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("JSON missing %q: %s", want, s)
		}
	}
}

func TestEnvironmentInfo_JSONZeroValue(t *testing.T) {
	info := EnvironmentInfo{}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal zero value: %v", err)
	}
	for _, want := range []string{`"detected":false`, `"available":false`, `"usable":false`, `"in_jail":false`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("zero JSON missing %q: %s", want, string(data))
		}
	}
}
