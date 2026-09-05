package compute

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"
)

// =============================================================================
// Memory Probe — real machine
// =============================================================================

func TestProbeMemory_RealMachine(t *testing.T) {
	info := ProbeMemory()

	if runtime.GOOS == "linux" {
		if info.PhysicalBytes == 0 {
			t.Error("PhysicalBytes should be > 0 on Linux")
		}
		if info.VisibleBytes != info.PhysicalBytes {
			t.Errorf("VisibleBytes = %d, want == PhysicalBytes %d (MemTotal reflete o que o OS vê)",
				info.VisibleBytes, info.PhysicalBytes)
		}
		if info.AvailableBytes == 0 {
			t.Error("AvailableBytes should be > 0 on Linux")
		}
		if info.EffectiveBytes == 0 {
			t.Error("EffectiveBytes should be > 0 on Linux")
		}
		if !info.Detected || !info.Available || !info.Usable {
			t.Errorf("tiers should be true, got detected=%v available=%v usable=%v",
				info.Detected, info.Available, info.Usable)
		}
		if len(info.Limitations) == 0 {
			t.Log("no limitations recorded (expected: physical/visible caveat is always noted)")
		}
	}
	if info.PageSize == 0 {
		t.Error("PageSize should be > 0")
	}
}

func TestProbeMemory_SnapshotIncludesMemory(t *testing.T) {
	snap := ProbeHardware()
	if runtime.GOOS == "linux" && snap.Memory.PhysicalBytes == 0 {
		t.Error("ProbeHardware should populate memory discovery (physical > 0 on Linux)")
	}
	if snap.Memory.PhysicalBytes > 0 && snap.Memory.EffectiveBytes == 0 {
		t.Error("effective should be > 0 when memory is detected")
	}
}

// =============================================================================
// Memory Probe — fake environments
// =============================================================================

func fakeMemorySourcesEmpty() memorySources {
	return memorySources{
		readFile: func(string) string { return "" },
		pageSize: func() int64 { return 0 },
		rlimitAS: func() (uint64, bool) { return 0, false },
	}
}

const fakeMemInfo = `MemTotal:       16777216 kB
MemFree:        10485760 kB
MemAvailable:   12582912 kB
Buffers:          195096 kB
Cached:          3014944 kB
SwapTotal:       2097152 kB
SwapFree:        1048576 kB
`

func TestProbeMemory_FakeFull(t *testing.T) {
	s := fakeMemorySourcesEmpty()
	s.pageSize = func() int64 { return 4096 }
	s.readFile = func(path string) string {
		switch path {
		case "/proc/meminfo":
			return fakeMemInfo
		}
		return ""
	}
	// Sem cgroup nem rlimit finito → effective = visible.
	s.rlimitAS = func() (uint64, bool) { return 0, false }

	info := probeMemory(s)
	if info.PhysicalBytes != 16777216*1024 {
		t.Errorf("physical = %d, want %d", info.PhysicalBytes, 16777216*1024)
	}
	if info.VisibleBytes != info.PhysicalBytes {
		t.Errorf("visible = %d, want == physical %d", info.VisibleBytes, info.PhysicalBytes)
	}
	if info.AvailableBytes != 12582912*1024 {
		t.Errorf("available = %d, want %d (MemAvailable, não MemFree)", info.AvailableBytes, 12582912*1024)
	}
	if info.EffectiveBytes != info.VisibleBytes {
		t.Errorf("effective = %d, want visible %d (sem limite finito)", info.EffectiveBytes, info.VisibleBytes)
	}
	if info.PageSize != 4096 {
		t.Errorf("page size = %d, want 4096", info.PageSize)
	}
	if info.SwapTotalBytes != 2097152*1024 || info.SwapAvailableBytes != 1048576*1024 {
		t.Errorf("swap = %d/%d, want %d/%d",
			info.SwapTotalBytes, info.SwapAvailableBytes, 2097152*1024, 1048576*1024)
	}
	if !info.Detected || !info.Available || !info.Usable {
		t.Errorf("tiers = %v/%v/%v, want true/true/true",
			info.Detected, info.Available, info.Usable)
	}
}

func TestProbeMemory_PartialInfo(t *testing.T) {
	// /proc/meminfo presente mas cgroup e rlimit indisponíveis — a função não
	// falha: effective = visible, limitação registrada.
	s := fakeMemorySourcesEmpty()
	s.pageSize = func() int64 { return 4096 }
	s.readFile = func(path string) string {
		if path == "/proc/meminfo" {
			return fakeMemInfo
		}
		return ""
	}
	s.rlimitAS = func() (uint64, bool) { return 0, false }

	info := probeMemory(s)
	if info.EffectiveBytes != info.VisibleBytes {
		t.Errorf("effective = %d, want visible %d (fallback)", info.EffectiveBytes, info.VisibleBytes)
	}
	found := false
	for _, l := range info.Limitations {
		if strings.Contains(l, "limite efetivo = visível") {
			found = true
		}
	}
	if !found {
		t.Errorf("limitation about effective fallback missing: %v", info.Limitations)
	}
}

func TestProbeMemory_Fallback_EmptySources(t *testing.T) {
	// Nenhuma fonte disponível: função NUNCA falha; modelo zero-value + limitation.
	info := probeMemory(fakeMemorySourcesEmpty())
	if info.PhysicalBytes != 0 || info.AvailableBytes != 0 || info.EffectiveBytes != 0 {
		t.Errorf("expected zero-value bytes, got %d/%d/%d",
			info.PhysicalBytes, info.AvailableBytes, info.EffectiveBytes)
	}
	if info.Detected || info.Available || info.Usable {
		t.Error("tiers should be false in empty environment")
	}
	if len(info.Limitations) == 0 {
		t.Error("limitations should be recorded in empty environment")
	}
}

func TestProbeMemory_Effective_CgroupFake(t *testing.T) {
	// cgroup v2 simulado: caminho + memory.max finito → effective = min(visible, limit).
	s := fakeMemorySourcesEmpty()
	s.pageSize = func() int64 { return 4096 }
	s.rlimitAS = func() (uint64, bool) { return 0, false }
	s.readFile = func(path string) string {
		switch path {
		case "/proc/meminfo":
			return fakeMemInfo
		case "/proc/self/cgroup":
			return "0::/user.slice/test.scope"
		case "/sys/fs/cgroup/user.slice/test.scope/memory.max":
			return "1073741824" // 1 GiB
		}
		return ""
	}

	info := probeMemory(s)
	if info.EffectiveBytes != 1073741824 {
		t.Errorf("effective = %d, want %d (cgroup memory.max)", info.EffectiveBytes, 1073741824)
	}
	if info.VisibleBytes <= info.EffectiveBytes {
		t.Errorf("visible %d should exceed effective %d (cgroup limita)",
			info.VisibleBytes, info.EffectiveBytes)
	}
	found := false
	for _, l := range info.Limitations {
		if strings.Contains(l, "mais restritivo") {
			found = true
		}
	}
	if !found {
		t.Errorf("limitation about stricter effective missing: %v", info.Limitations)
	}
}

func TestProbeMemory_Effective_CgroupMaxIsNoLimit(t *testing.T) {
	// "max" no memory.max = sem limite finito → effective = visible (fallback).
	s := fakeMemorySourcesEmpty()
	s.pageSize = func() int64 { return 4096 }
	s.rlimitAS = func() (uint64, bool) { return 0, false }
	s.readFile = func(path string) string {
		switch path {
		case "/proc/meminfo":
			return fakeMemInfo
		case "/proc/self/cgroup":
			return "0::/user.slice/test.scope"
		case "/sys/fs/cgroup/user.slice/test.scope/memory.max":
			return "max"
		}
		return ""
	}

	info := probeMemory(s)
	if info.EffectiveBytes != info.VisibleBytes {
		t.Errorf("effective = %d, want visible %d (memory.max = max → sem limite)",
			info.EffectiveBytes, info.VisibleBytes)
	}
}

func TestProbeMemory_Effective_RLIMITASFake(t *testing.T) {
	// RLIMIT_AS finito, sem cgroup → effective = min(visible, rlimit).
	s := fakeMemorySourcesEmpty()
	s.pageSize = func() int64 { return 4096 }
	s.rlimitAS = func() (uint64, bool) { return 1 << 30, true } // 1 GiB
	s.readFile = func(path string) string {
		if path == "/proc/meminfo" {
			return fakeMemInfo
		}
		return ""
	}

	info := probeMemory(s)
	if info.EffectiveBytes != 1<<30 {
		t.Errorf("effective = %d, want %d (RLIMIT_AS)", info.EffectiveBytes, int64(1)<<30)
	}
}

// =============================================================================
// Invariants
// =============================================================================

func TestProbeMemory_Invariant_AvailableLeVisibleLePhysical(t *testing.T) {
	// Com todas as fontes presentes, Available ≤ Visible ≤ Physical.
	// Onde o invariante pode NÃO valer (documentado): overcommit — a soma do
	// address space comprometido (Committed_AS) pode exceder a RAM física
	// (o kernel permite commit além do físico; as camadas aqui são de
	// contabilidade do /proc/meminfo, não de address space reservado).
	s := fakeMemorySourcesEmpty()
	s.pageSize = func() int64 { return 4096 }
	s.rlimitAS = func() (uint64, bool) { return 0, false }
	s.readFile = func(path string) string {
		if path == "/proc/meminfo" {
			return fakeMemInfo
		}
		return ""
	}

	info := probeMemory(s)
	if info.AvailableBytes > info.VisibleBytes {
		t.Errorf("invariant violated: available %d > visible %d", info.AvailableBytes, info.VisibleBytes)
	}
	if info.VisibleBytes > info.PhysicalBytes {
		t.Errorf("invariant violated: visible %d > physical %d", info.VisibleBytes, info.PhysicalBytes)
	}
	if info.AvailableBytes > info.PhysicalBytes {
		t.Errorf("invariant violated: available %d > physical %d", info.AvailableBytes, info.PhysicalBytes)
	}
}

func TestProbeMemory_Invariant_MemAvailablePreferred(t *testing.T) {
	// MemFree ≠ MemAvailable: com ambos presentes, MemAvailable vence; com
	// MemAvailable ausente, cai para MemFree (documentado no parser).
	out := `MemTotal: 16777216 kB
MemFree:  10485760 kB
MemAvailable: 12582912 kB
SwapTotal: 2097152 kB
SwapFree:  1048576 kB
`
	_, available, _, _ := parseMemInfo(out)
	if available != 12582912*1024 {
		t.Errorf("available = %d, want MemAvailable %d (≠ MemFree)", available, 12582912*1024)
	}

	outNoAvail := `MemTotal: 16777216 kB
MemFree:  10485760 kB
SwapTotal: 2097152 kB
SwapFree:  1048576 kB
`
	_, available2, _, _ := parseMemInfo(outNoAvail)
	if available2 != 10485760*1024 {
		t.Errorf("available = %d, want MemFree fallback %d", available2, 10485760*1024)
	}
}

// =============================================================================
// Parsers
// =============================================================================

func TestParseCgroupPathFromProcSelf(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"0::/user.slice/test.scope", "/user.slice/test.scope"},
		{"2:memory:/docker/abc123", "/docker/abc123"},
		{"2:cpu,memory:/a/b", "/a/b"},
		{"0::/", "/"},
		{"", ""},
		{"garbage", ""},
	}
	for _, tt := range tests {
		if got := cgroupPathFromProcSelf(tt.in); got != tt.want {
			t.Errorf("cgroupPathFromProcSelf(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseCgroupMemoryLimit_V2(t *testing.T) {
	s := fakeMemorySourcesEmpty()
	s.readFile = func(path string) string {
		switch path {
		case "/proc/self/cgroup":
			return "0::/user.slice/test.scope"
		case "/sys/fs/cgroup/user.slice/test.scope/memory.max":
			return "536870912"
		}
		return ""
	}
	limit, ok := parseCgroupMemoryLimit(s)
	if !ok || limit != 536870912 {
		t.Errorf("v2 limit = %d ok=%v, want 536870912/true", limit, ok)
	}

	// "max" = ilimitado → sem limite finito.
	s.readFile = func(path string) string {
		switch path {
		case "/proc/self/cgroup":
			return "0::/user.slice/test.scope"
		case "/sys/fs/cgroup/user.slice/test.scope/memory.max":
			return "max"
		}
		return ""
	}
	limit, ok = parseCgroupMemoryLimit(s)
	if ok {
		t.Errorf("v2 'max' should not report a finite limit, got %d", limit)
	}

	// cgroup legível mas memory.max ausente → (0, false).
	s.readFile = func(path string) string {
		if path == "/proc/self/cgroup" {
			return "0::/user.slice/test.scope"
		}
		return ""
	}
	limit, ok = parseCgroupMemoryLimit(s)
	if ok {
		t.Errorf("missing memory.max should not report a limit, got %d", limit)
	}
}

func TestParseCgroupMemoryLimit_V1(t *testing.T) {
	s := fakeMemorySourcesEmpty()
	s.readFile = func(path string) string {
		switch path {
		case "/proc/self/cgroup":
			return "2:memory:/docker/abc123"
		case "/sys/fs/cgroup/memory/docker/abc123/memory.limit_in_bytes":
			return "268435456"
		}
		return ""
	}
	limit, ok := parseCgroupMemoryLimit(s)
	if !ok || limit != 268435456 {
		t.Errorf("v1 limit = %d ok=%v, want 268435456/true", limit, ok)
	}

	// Sentinela v1 (≈2^63-1024 = "sem limite") → sem limite finito.
	s.readFile = func(path string) string {
		switch path {
		case "/proc/self/cgroup":
			return "2:memory:/docker/abc123"
		case "/sys/fs/cgroup/memory/docker/abc123/memory.limit_in_bytes":
			return "9223372036854771712"
		}
		return ""
	}
	limit, ok = parseCgroupMemoryLimit(s)
	if ok {
		t.Errorf("v1 sentinel should not report a finite limit, got %d", limit)
	}
}

func TestResolveEffective(t *testing.T) {
	tests := []struct {
		name    string
		visible int64
		cgL     int64
		cgOK    bool
		rlC     uint64
		rlOK    bool
		want    int64
	}{
		{"no limits", 16000000000, 0, false, 0, false, 16000000000},
		{"cgroup stricter", 16000000000, 1073741824, true, 0, false, 1073741824},
		{"rlimit stricter", 16000000000, 0, false, 1 << 30, true, 1 << 30},
		{"both, strictest wins", 16000000000, 1 << 29, true, 1 << 30, true, 1 << 29},
		{"cgroup larger ignored", 16000000000, 1 << 40, true, 0, false, 16000000000},
		{"zero visible", 0, 1073741824, true, 1 << 30, true, 0},
	}
	for _, tt := range tests {
		if got := resolveEffective(tt.visible, tt.cgL, tt.cgOK, tt.rlC, tt.rlOK); got != tt.want {
			t.Errorf("%s: resolveEffective = %d, want %d", tt.name, got, tt.want)
		}
	}
}

// =============================================================================
// Serialization
// =============================================================================

func TestMemoryInfo_JSONSerialization(t *testing.T) {
	info := MemoryInfo{
		PhysicalBytes:      16777216 * 1024,
		VisibleBytes:       16777216 * 1024,
		AvailableBytes:     12582912 * 1024,
		EffectiveBytes:     1073741824,
		PageSize:           4096,
		SwapTotalBytes:     2097152 * 1024,
		SwapAvailableBytes: 1048576 * 1024,
		Detected:           true,
		Available:          true,
		Usable:             true,
		Limitations:        []string{"cgroup memory limit imposto"},
	}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)
	for _, want := range []string{
		`"physical_bytes":17179869184`, `"visible_bytes":17179869184`,
		`"available_bytes":12884901888`, `"effective_bytes":1073741824`,
		`"page_size":4096`, `"swap_total_bytes":2147483648`,
		`"swap_available_bytes":1073741824`,
		`"detected":true`, `"available":true`, `"usable":true`,
		`"limitations":["cgroup memory limit imposto"]`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("JSON missing %q: %s", want, s)
		}
	}
}

func TestMemoryInfo_JSONZeroValue(t *testing.T) {
	// Zero-value deve serializar sem erro e com os campos required presentes.
	info := MemoryInfo{}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal zero value: %v", err)
	}
	for _, want := range []string{`"detected":false`, `"available":false`, `"usable":false`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("zero JSON missing %q: %s", want, string(data))
		}
	}
	// bytes 0 (zero-value = desconhecido) são omitidos pelo omitempty.
	for _, skip := range []string{`"physical_bytes"`, `"effective_bytes"`} {
		if strings.Contains(string(data), skip) {
			t.Errorf("zero JSON should omit %q: %s", skip, string(data))
		}
	}
}
