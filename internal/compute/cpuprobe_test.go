package compute

import (
	"encoding/json"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// =============================================================================
// CPU Probe — real machine
// =============================================================================

func TestProbeCPU_RealMachine(t *testing.T) {
	info := ProbeCPU()

	if info.LogicalCPUs == 0 {
		t.Error("LogicalCPUs should be > 0")
	}
	if info.LogicalCPUs != runtime.NumCPU() {
		t.Errorf("LogicalCPUs = %d, want runtime.NumCPU() = %d", info.LogicalCPUs, runtime.NumCPU())
	}
	if info.Arch != runtime.GOARCH {
		t.Errorf("Arch = %q, want %q", info.Arch, runtime.GOARCH)
	}
	if !info.Detected || !info.Available || !info.Usable {
		t.Errorf("detected/available/usable should be true, got %v/%v/%v",
			info.Detected, info.Available, info.Usable)
	}
	if runtime.GOOS == "linux" {
		if info.Vendor == "" {
			t.Error("Vendor should be non-empty on Linux")
		}
		if info.PhysicalCores == 0 {
			t.Error("PhysicalCores should be > 0 on Linux (physical id/core id present)")
		}
		if info.ModelName == "" {
			t.Error("ModelName should be non-empty on Linux")
		}
		if len(info.Features) == 0 {
			t.Error("Features should not be empty on a real CPU")
		}
	}
}

func TestProbeCPU_FeaturesPresent(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("amd64/Linux only assertions")
	}
	info := ProbeCPU()
	// Flags presentes na maioria dos x86-64 modernos (e no host deste ambiente).
	for _, want := range []string{"fpu", "sse", "sse2", "avx", "avx2", "fma", "aes", "bmi1", "bmi2"} {
		if !info.Features[want] {
			t.Errorf("feature %q should be present", want)
		}
	}
}

func TestProbeCPU_SnapshotIncludesCPU(t *testing.T) {
	snap := ProbeHardware()
	if runtime.GOOS == "linux" && snap.CPU.Vendor == "" {
		t.Error("ProbeHardware should populate CPU discovery (vendor non-empty on Linux)")
	}
	if snap.CPU.LogicalCPUs != runtime.NumCPU() {
		t.Errorf("snapshot CPU.LogicalCPUs = %d, want %d", snap.CPU.LogicalCPUs, runtime.NumCPU())
	}
}

func TestProbeCPU_Concurrent(t *testing.T) {
	// Validação de concorrência do probe (o race detector é o juiz real).
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = ProbeCPU()
		}()
	}
	wg.Wait()
}

// =============================================================================
// CPU Probe — fake environments
// =============================================================================

func fakeCPUSourcesEmpty() cpuSources {
	return cpuSources{
		readFile: func(string) string { return "" },
		listDir:  func(string) []string { return nil },
		numCPU:   func() int { return 0 },
		goArch:   func() string { return "amd64" },
	}
}

const fakeCPUMultiPackage = `processor	: 0
vendor_id	: GenuineIntel
cpu family	: 6
model		: 165
model name	: Intel(R) Core(TM) i7-10700K CPU @ 3.80GHz
stepping	: 5
cpu MHz		: 4700.000
physical id	: 0
core id		: 0
flags		: fpu sse sse2 ssse3 fma avx avx2 aes bmi1 bmi2
processor	: 1
physical id	: 0
core id		: 0
flags		: fpu sse sse2 ssse3 fma avx avx2 aes bmi1 bmi2
processor	: 2
physical id	: 0
core id		: 1
flags		: fpu sse sse2 ssse3 fma avx avx2 aes bmi1 bmi2
processor	: 3
physical id	: 0
core id		: 1
flags		: fpu sse sse2 ssse3 fma avx avx2 aes bmi1 bmi2
processor	: 4
physical id	: 1
core id		: 0
flags		: fpu sse sse2 ssse3 fma avx avx2 aes bmi1 bmi2
processor	: 5
physical id	: 1
core id		: 0
flags		: fpu sse sse2 ssse3 fma avx avx2 aes bmi1 bmi2
processor	: 6
physical id	: 1
core id		: 1
flags		: fpu sse sse2 ssse3 fma avx avx2 aes bmi1 bmi2
processor	: 7
physical id	: 1
core id		: 1
flags		: fpu sse sse2 ssse3 fma avx avx2 aes bmi1 bmi2
`

func TestProbeCPU_FakeFull(t *testing.T) {
	s := fakeCPUSourcesEmpty()
	s.numCPU = func() int { return 8 }
	s.readFile = func(path string) string {
		switch path {
		case "/proc/cpuinfo":
			return fakeCPUMultiPackage
		case "/sys/devices/system/cpu/cpu0/cache/index0/level":
			return "1"
		case "/sys/devices/system/cpu/cpu0/cache/index0/size":
			return "32K"
		case "/sys/devices/system/cpu/cpu0/cache/index0/type":
			return "Data"
		case "/sys/devices/system/cpu/cpu0/cache/index2/level":
			return "2"
		case "/sys/devices/system/cpu/cpu0/cache/index2/size":
			return "512K"
		case "/sys/devices/system/cpu/cpu0/cache/index2/type":
			return "Unified"
		case "/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq":
			return "5100000"
		case "/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq":
			return "4700000"
		}
		return ""
	}
	s.listDir = func(path string) []string {
		if path == "/sys/devices/system/cpu/cpu0/cache" {
			return []string{"index0", "index2"}
		}
		return nil
	}

	info := probeCPU(s)
	if info.Vendor != CPUIntel {
		t.Errorf("vendor = %q, want intel", info.Vendor)
	}
	if info.Family != "6" || info.Model != "165" || info.Stepping != "5" {
		t.Errorf("family/model/stepping = %q/%q/%q, want 6/165/5",
			info.Family, info.Model, info.Stepping)
	}
	if info.ModelName != "Intel(R) Core(TM) i7-10700K CPU @ 3.80GHz" {
		t.Errorf("model name = %q", info.ModelName)
	}
	if info.LogicalCPUs != 8 {
		t.Errorf("logical = %d, want 8", info.LogicalCPUs)
	}
	if info.PhysicalCores != 4 {
		t.Errorf("physical cores = %d, want 4 (2 packages × 2)", info.PhysicalCores)
	}
	if info.Packages != 2 {
		t.Errorf("packages = %d, want 2", info.Packages)
	}
	if info.Features["avx2"] != true || info.Features["sse4_1"] == true {
		t.Errorf("features mismatch: avx2=%v (want true), sse4_1=%v (want false)",
			info.Features["avx2"], info.Features["sse4_1"])
	}
	if info.Frequencies.BaseMHz != 3800 {
		t.Errorf("base = %d, want 3800 (from model name)", info.Frequencies.BaseMHz)
	}
	if info.Frequencies.MaxMHz != 5100 {
		t.Errorf("max = %d, want 5100", info.Frequencies.MaxMHz)
	}
	if info.Frequencies.CurrentMHz != 4700 {
		t.Errorf("current = %d, want 4700", info.Frequencies.CurrentMHz)
	}
	if len(info.Caches) != 2 || info.Caches[0].Level != 1 || info.Caches[0].SizeKB != 32 ||
		info.Caches[1].Level != 2 || info.Caches[1].SizeKB != 512 {
		t.Errorf("caches = %+v, want L1 32K + L2 512K", info.Caches)
	}
	if !info.Detected || !info.Available || !info.Usable {
		t.Errorf("tiers = %v/%v/%v, want true/true/true",
			info.Detected, info.Available, info.Usable)
	}
	if info.Capacity.Physical != 4 || info.Capacity.Visible != 8 || info.Capacity.Effective != 0 {
		t.Errorf("capacity = %+v, want physical=4 visible=8 effective=0", info.Capacity)
	}
}

func TestProbeCPU_PartialInfo(t *testing.T) {
	// /proc/cpuinfo presente mas sysfs (cache/cpufreq) indisponível — a função
	// não falha: caches vazios, frequência atual via "cpu MHz", limitação registrada.
	s := fakeCPUSourcesEmpty()
	s.numCPU = func() int { return 16 }
	s.readFile = func(path string) string {
		if path == "/proc/cpuinfo" {
			return fakeCPUMultiPackage
		}
		return ""
	}
	info := probeCPU(s)
	if len(info.Caches) != 0 {
		t.Errorf("caches = %+v, want empty without sysfs", info.Caches)
	}
	if info.Frequencies.CurrentMHz != 4700 {
		t.Errorf("current = %d, want 4700 (fallback from cpu MHz)", info.Frequencies.CurrentMHz)
	}
	if len(info.Limitations) == 0 {
		t.Error("limitations should be recorded when sysfs sources are missing")
	}
}

func TestProbeCPU_Fallback_EmptySources(t *testing.T) {
	// Nenhuma fonte disponível: função NUNCA falha; modelo zero-value + limitation.
	info := probeCPU(fakeCPUSourcesEmpty())
	if info.LogicalCPUs != 0 || info.PhysicalCores != 0 {
		t.Errorf("expected zero-value counts, got %d/%d", info.LogicalCPUs, info.PhysicalCores)
	}
	if info.Detected || info.Available || info.Usable {
		t.Error("tiers should be false in empty environment")
	}
	if info.Features == nil {
		t.Error("Features map should be non-nil (serializable)")
	}
	if len(info.Limitations) == 0 {
		t.Error("limitations should be recorded in empty environment")
	}
}

func TestProbeCPU_FeatureAbsentIsFalse(t *testing.T) {
	s := fakeCPUSourcesEmpty()
	s.numCPU = func() int { return 4 }
	s.readFile = func(path string) string {
		if path == "/proc/cpuinfo" {
			// CPU sem AVX-512 e sem AVX2.
			return "processor	: 0\nvendor_id	: GenuineIntel\nmodel name	: Test CPU\nflags		: fpu sse sse2\n"
		}
		return ""
	}
	info := probeCPU(s)
	if info.Features["avx512f"] != false {
		t.Error("avx512f should be false (flag ausente)")
	}
	if info.Features["avx2"] != false {
		t.Error("avx2 should be false (flag ausente)")
	}
	if info.Features["sse2"] != true {
		t.Error("sse2 should be true (flag presente)")
	}
}

func TestProbeCPU_ARMVendorFallback(t *testing.T) {
	s := fakeCPUSourcesEmpty()
	s.goArch = func() string { return "arm64" }
	info := probeCPU(s)
	if info.Vendor != CPUARM {
		t.Errorf("vendor = %q, want arm (GOARCH fallback)", info.Vendor)
	}
}

// =============================================================================
// Parsers
// =============================================================================

func TestParseCacheSizeKB(t *testing.T) {
	tests := []struct{ in string; want int }{
		{"32K", 32},
		{"512K", 512},
		{"98304K", 98304},
		{"3M", 3072},
		{"1M", 1024},
		{"junk", 0},
		{"", 0},
	}
	for _, tt := range tests {
		if got := parseCacheSizeKB(tt.in); got != tt.want {
			t.Errorf("parseCacheSizeKB(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestBaseMHzFromModelName(t *testing.T) {
	if got := baseMHzFromModelName("Intel(R) Core(TM) i7-10700K CPU @ 3.80GHz"); got != 3800 {
		t.Errorf("i7-10700K base = %d, want 3800", got)
	}
	if got := baseMHzFromModelName("AMD Ryzen 7 5700X3D 8-Core Processor"); got != 0 {
		t.Errorf("5700X3D base = %d, want 0 (clock não exposto no nome)", got)
	}
	if got := baseMHzFromModelName(""); got != 0 {
		t.Errorf("empty base = %d, want 0", got)
	}
}

func TestParseFrequencies(t *testing.T) {
	s := fakeCPUSourcesEmpty()
	s.readFile = func(path string) string {
		switch path {
		case "/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq":
			return "4151384"
		case "/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq":
			return "4050068"
		}
		return ""
	}
	f := parseFrequencies(s, 0)
	if f.MaxMHz != 4151 {
		t.Errorf("max = %d, want 4151", f.MaxMHz)
	}
	if f.CurrentMHz != 4050 {
		t.Errorf("current = %d, want 4050", f.CurrentMHz)
	}

	// sysfs ausente → fallback para o "cpu MHz" do /proc/cpuinfo.
	s2 := fakeCPUSourcesEmpty()
	f2 := parseFrequencies(s2, 4048.647)
	if f2.CurrentMHz != 4048 {
		t.Errorf("current fallback = %d, want 4048", f2.CurrentMHz)
	}
	if f2.MaxMHz != 0 {
		t.Errorf("max without sysfs = %d, want 0", f2.MaxMHz)
	}
}

func TestParseCPUInfo_NoPhysicalIDs(t *testing.T) {
	// VM/container sem physical id/core id: cores físicos = 0 (desconhecido),
	// sem crash.
	out := "processor	: 0\nvendor_id	: AuthenticAMD\nflags		: fpu sse\nprocessor	: 1\nflags		: fpu sse\n"
	_, _, _, _, _, cores, pkgs, _, _ := parseCPUInfo(out)
	if cores != 0 {
		t.Errorf("physical cores = %d, want 0 without physical id", cores)
	}
	if pkgs != 0 {
		t.Errorf("packages = %d, want 0 without physical id", pkgs)
	}
}

// =============================================================================
// Serialization
// =============================================================================

func TestCPUInfo_JSONSerialization(t *testing.T) {
	info := CPUInfo{
		Vendor:        CPUAMD,
		Family:        "25",
		Model:         "33",
		Stepping:      "2",
		ModelName:     "AMD Ryzen 7 5700X3D 8-Core Processor",
		Arch:          "amd64",
		LogicalCPUs:   16,
		PhysicalCores: 8,
		Packages:      1,
		Frequencies:   CPUFrequencies{BaseMHz: 0, MaxMHz: 4151, CurrentMHz: 4050},
		Caches:        []CPUCacheInfo{{Level: 1, SizeKB: 32, Type: "Data"}},
		Features:      map[string]bool{"avx2": true, "avx512f": false},
		Detected:      true,
		Available:     true,
		Usable:        true,
		Capacity:      CPUCapacity{Physical: 8, Visible: 16},
		Limitations:   []string{"clock base não exposto em user space"},
	}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)
	for _, want := range []string{
		`"vendor":"amd"`, `"family":"25"`, `"model":"33"`, `"stepping":"2"`,
		`"logical_cpus":16`, `"physical_cores":8`, `"packages":1`,
		`"max_mhz":4151`, `"level":1`, `"size_kb":32`,
		`"avx2":true`, `"avx512f":false`,
		`"detected":true`, `"available":true`, `"usable":true`,
		`"physical":8`, `"visible":16`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("JSON missing %q: %s", want, s)
		}
	}
	// effective=0 (zero-value = desconhecido) é omitido pelo omitempty.
	if strings.Contains(s, `"effective"`) {
		t.Errorf("JSON should omit effective when 0: %s", s)
	}
}

func TestCPUInfo_JSONZeroValue(t *testing.T) {
	// Zero-value deve serializar sem error e com os campos required presentes.
	info := CPUInfo{}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal zero value: %v", err)
	}
	for _, want := range []string{`"vendor":""`, `"arch":""`, `"features":null`,
		`"detected":false`, `"available":false`, `"usable":false`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("zero JSON missing %q: %s", want, string(data))
		}
	}
}
