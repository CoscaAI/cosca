package compute

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"
)

// =============================================================================
// Storage Probe — real machine
// =============================================================================

func TestProbeStorage_RealMachine(t *testing.T) {
	info := ProbeStorage()

	if runtime.GOOS == "linux" {
		if len(info.Mounts) == 0 {
			t.Error("Mounts should not be empty on Linux (/proc/mounts legível)")
		}
		if !info.Detected {
			t.Error("Detected should be true (mounts observados)")
		}
		if !info.Capabilities.Reported {
			t.Error("Capabilities.Reported should be true")
		}
		if info.Capabilities.Measured {
			t.Error("Capabilities.Measured must be false (benchmark é Fase 9)")
		}
		// O filesystem do cofre deve ser resolvido (o repo tem .cosca/).
		if info.KnowledgeDBMount == nil {
			t.Error("knowledge.db mount not resolved (esperado: .cosca/knowledge.db coberto por um mount)")
		}
		// /tmp deve ser coberto por um mount (tmpfs ou o fs raiz).
		if info.TmpMount == nil {
			t.Error("TmpMount should be resolved (/tmp coberto por um mount)")
		}
		if len(info.Limitations) == 0 {
			t.Log("no limitations recorded")
		}
	}
}

func TestProbeHardware_SnapshotIncludesStorage(t *testing.T) {
	snap := ProbeHardware()
	if snap.Storage.Detected && len(snap.Storage.Mounts) == 0 {
		t.Error("snapshot storage should carry the mounts observed")
	}
	if snap.Storage.Capabilities.Measured {
		t.Error("snapshot storage measured must be false (F9)")
	}
}

// =============================================================================
// Storage Probe — fake environments
// =============================================================================

func fakeStorageSourcesEmpty() storageSources {
	return storageSources{
		readFile: func(string) string { return "" },
		listDir:  func(string) []string { return nil },
		stat:     func(string) bool { return false },
		statfs:   func(string) (statfsInfo, bool) { return statfsInfo{}, false },
		environment: func() EnvironmentInfo { return EnvironmentInfo{} },
		getwd:    func() (string, bool) { return "/work", true },
		knowledgeDB: func() string { return "/work/.cosca/knowledge.db" },
	}
}

// fakeStorageFull monta um ambiente fake completo: mounts com statfs e sysfs.
func fakeStorageFull() storageSources {
	s := fakeStorageSourcesEmpty()
	s.readFile = func(path string) string {
		switch path {
		case "/proc/mounts":
			return fakeMounts
		case "/sys/block/nvme0n1/queue/rotational":
			return "0\n"
		case "/sys/block/nvme0n1/queue/nr_requests":
			return "128\n"
		case "/sys/block/sda/queue/rotational":
			return "1\n"
		case "/sys/block/sda/queue/nr_requests":
			return "64\n"
		}
		return ""
	}
	s.statfs = func(path string) (statfsInfo, bool) {
		switch path {
		case "/":
			return statfsInfo{TotalBytes: 1 << 40, FreeBytes: (1 << 40) - (300 << 30), AvailableBytes: 200 << 30}, true
		case "/mnt/data":
			return statfsInfo{TotalBytes: 2 << 40, FreeBytes: (2 << 40) - (1 << 40), AvailableBytes: 1 << 40}, true
		case "/tmp":
			return statfsInfo{TotalBytes: 8 << 30, FreeBytes: 4 << 30, AvailableBytes: 4 << 30}, true
		}
		return statfsInfo{}, false
	}
	s.listDir = func(path string) []string {
		if path == "/dev" {
			return []string{"sda", "sda1", "nvme0n1", "nvme0n1p2", "mmcblk0", "null", "tty0"}
		}
		return nil
	}
	return s
}

const fakeMounts = `/dev/nvme0n1p2 / ext4 rw,relatime 0 0
/dev/sda1 /mnt/data ext4 rw,relatime 0 0
tmpfs /tmp tmpfs rw,nosuid,nodev 0 0
`

func TestProbeStorage_FakeFull(t *testing.T) {
	// O fake usa literais de mounts POSIX ("/", "/mnt/data", "/tmp").
	// No Windows o mountForPath resolve via filepath.Clean, que converte os
	// separadores para "\" e quebra o prefix-match contra os literais "/" —
	// a semântica de mounts POSIX não se aplica a esta plataforma.
	if runtime.GOOS == "windows" {
		t.Skip("teste de fake valida semântica de mounts POSIX (literal '/') — não aplicável no Windows")
	}
	info := probeStorage(fakeStorageFull())

	if len(info.Mounts) != 3 {
		t.Fatalf("mounts = %d, want 3", len(info.Mounts))
	}
	// Ordem preservada de /proc/mounts.
	root := info.Mounts[0]
	if root.MountPoint != "/" || root.FSType != "ext4" || root.Device != "/dev/nvme0n1p2" {
		t.Errorf("root mount = %+v", root)
	}
	if root.TotalBytes != 1<<40 {
		t.Errorf("root total = %d, want %d", root.TotalBytes, uint64(1)<<40)
	}
	if root.UsedBytes != 300<<30 {
		t.Errorf("root used = %d, want %d", root.UsedBytes, uint64(300)<<30)
	}
	if root.AvailableBytes != 200<<30 {
		t.Errorf("root available = %d, want %d", root.AvailableBytes, uint64(200)<<30)
	}
	// NVMe: DeviceType direto, rotational=0, nr_requests lido.
	if root.DeviceType != StorageNVMe {
		t.Errorf("root device_type = %q, want nvme", root.DeviceType)
	}
	if root.Rotational == nil || *root.Rotational {
		t.Errorf("root rotational = %v, want false (nvme)", root.Rotational)
	}
	if root.NrRequests != 128 {
		t.Errorf("root nr_requests = %d, want 128", root.NrRequests)
	}
	// HDD: rotational=1 → hdd.
	data := info.Mounts[1]
	if data.DeviceType != StorageHDD {
		t.Errorf("data device_type = %q, want hdd", data.DeviceType)
	}
	if data.Rotational == nil || !*data.Rotational {
		t.Errorf("data rotational = %v, want true (hdd)", data.Rotational)
	}
	// tmpfs: tipo tmpfs, sem meio físico.
	tmp := info.Mounts[2]
	if tmp.DeviceType != StorageTmpfs {
		t.Errorf("tmp device_type = %q, want tmpfs", tmp.DeviceType)
	}
	if tmp.Rotational != nil {
		t.Errorf("tmp rotational should be nil (tmpfs não tem meio físico)")
	}

	// Focos do Cosca.
	if info.KnowledgeDBMount == nil || info.KnowledgeDBMount.MountPoint != "/" {
		t.Errorf("knowledge.db mount = %+v, want root (/)", info.KnowledgeDBMount)
	}
	if info.WorkspaceMount == nil || info.WorkspaceMount.MountPoint != "/" {
		t.Errorf("workspace mount = %+v, want root (/)", info.WorkspaceMount)
	}
	if info.TmpMount == nil || info.TmpMount.MountPoint != "/tmp" {
		t.Errorf("tmp mount = %+v, want /tmp", info.TmpMount)
	}
	if info.TmpMount.FSType != "tmpfs" {
		t.Errorf("tmp fstype = %q, want tmpfs (RAM)", info.TmpMount.FSType)
	}

	// Device nodes visíveis (só block: sd*, nvme*, mmc*).
	if len(info.DeviceNodes) != 5 {
		t.Errorf("device nodes = %v, want 5 (sda, sda1, nvme0n1, nvme0n1p2, mmcblk0)", info.DeviceNodes)
	}

	// Camadas.
	if !info.Detected || !info.Available || !info.Usable {
		t.Errorf("tiers = %v/%v/%v, want true/true/true", info.Detected, info.Available, info.Usable)
	}
	// Cadeia do Professor.
	c := info.Capabilities
	if !c.Reported || !c.Visible || !c.Usable || c.Measured {
		t.Errorf("capabilities = %+v, want reported/visible/usable=true measured=false", c)
	}
	if c.WhyNot == "" {
		t.Error("WhyNot should explain why measured is false")
	}
	if info.Provenance.Source == "" || info.Provenance.Confidence != "high" {
		t.Errorf("provenance = %+v, want source + high confidence", info.Provenance)
	}
}

func TestProbeStorage_Fallback_EmptySources(t *testing.T) {
	// Nenhuma fonte disponível: a função NUNCA falha; modelo zero-value +
	// limitations; measured sempre false.
	info := probeStorage(fakeStorageSourcesEmpty())

	if len(info.Mounts) != 0 || len(info.DeviceNodes) != 0 {
		t.Errorf("expected empty mounts/nodes, got %d/%d", len(info.Mounts), len(info.DeviceNodes))
	}
	if info.Detected || info.Available || info.Usable {
		t.Error("tiers should be false in empty environment")
	}
	if info.Capabilities.Reported || info.Capabilities.Visible || info.Capabilities.Usable || info.Capabilities.Measured {
		t.Errorf("capabilities should be all false, got %+v", info.Capabilities)
	}
	if len(info.Limitations) == 0 {
		t.Error("limitations should be recorded in empty environment")
	}
	found := false
	for _, l := range info.Limitations {
		if strings.Contains(l, "Fase 9") || strings.Contains(l, "Fase 9") {
			found = true
		}
	}
	if !found {
		t.Errorf("limitation about measured/F9 missing: %v", info.Limitations)
	}
}

func TestProbeStorage_PartialInfo(t *testing.T) {
	// /proc/mounts legível mas statfs/sysfs indisponíveis — informação parcial,
	// nunca falha: total 0, rotational nil, limitations registradas.
	s := fakeStorageSourcesEmpty()
	s.readFile = func(path string) string {
		if path == "/proc/mounts" {
			return fakeMounts
		}
		return ""
	}

	info := probeStorage(s)

	if len(info.Mounts) != 3 {
		t.Fatalf("mounts = %d, want 3", len(info.Mounts))
	}
	for i, m := range info.Mounts {
		if m.TotalBytes != 0 || m.AvailableBytes != 0 {
			t.Errorf("mount %d: expected zero capacity (statfs indisponível), got %+v", i, m)
		}
	}
	if !info.Detected {
		t.Error("Detected should be true (mounts reportados)")
	}
	if info.Available || info.Usable {
		t.Error("Available/Usable should be false (nenhum statfs OK)")
	}
	if !info.Capabilities.Reported {
		t.Error("Reported should be true")
	}
	if info.Capabilities.Visible || info.Capabilities.Usable {
		t.Error("Visible/Usable should be false")
	}
	if info.Capabilities.Measured {
		t.Error("Measured must be false")
	}
	found := false
	for _, l := range info.Limitations {
		if strings.Contains(l, "statfs indisponível") {
			found = true
		}
	}
	if !found {
		t.Errorf("limitation about statfs missing: %v", info.Limitations)
	}
	if info.Provenance.Confidence != "medium" {
		t.Errorf("confidence = %q, want medium (reportado, não mensurável)", info.Provenance.Confidence)
	}
}

func TestProbeStorage_KnowledgeDBFocus(t *testing.T) {
	// O fake usa literal de mount POSIX ("/work") — semântica não aplicável
	// no Windows (ver TestProbeStorage_FakeFull).
	if runtime.GOOS == "windows" {
		t.Skip("teste de fake valida semântica de mounts POSIX (literal '/work') — não aplicável no Windows")
	}
	// O cofre vive em /work/.cosca/knowledge.db → deve apontar para o mount
	// que cobre /work (longest-prefix).
	s := fakeStorageSourcesEmpty()
	s.readFile = func(path string) string {
		if path == "/proc/mounts" {
			return "/dev/sdb1 /work ext4 rw,relatime 0 0\n"
		}
		return ""
	}
	s.statfs = func(path string) (statfsInfo, bool) {
		if path == "/work" {
			return statfsInfo{TotalBytes: 1 << 40, FreeBytes: 1 << 40, AvailableBytes: 1 << 40}, true
		}
		return statfsInfo{}, false
	}

	info := probeStorage(s)
	if info.KnowledgeDBMount == nil || info.KnowledgeDBMount.MountPoint != "/work" {
		t.Errorf("knowledge.db mount = %+v, want /work", info.KnowledgeDBMount)
	}
	if info.WorkspaceMount == nil || info.WorkspaceMount.MountPoint != "/work" {
		t.Errorf("workspace mount = %+v, want /work", info.WorkspaceMount)
	}
}

// =============================================================================
// Parsers
// =============================================================================

func TestParseMounts(t *testing.T) {
	out := `/dev/nvme0n1p2 / ext4 rw,relatime 0 0
/dev/sda1 /mnt/data ext4 rw,relatime 0 0
tmpfs /run tmpfs rw,nosuid,nodev 0 0
mapper /var/run\040with\040space xfs rw 0 0
garbage-line
`
	mounts := parseMounts(out)
	if len(mounts) != 4 {
		t.Fatalf("mounts = %d, want 4 (linha inválida ignorada)", len(mounts))
	}
	first := mounts[0]
	if first.Device != "/dev/nvme0n1p2" || first.MountPoint != "/" || first.FSType != "ext4" || first.Options != "rw,relatime" {
		t.Errorf("first = %+v", first)
	}
	escaped := mounts[3]
	if escaped.MountPoint != "/var/run with space" {
		t.Errorf("escaped mount point = %q, want '/var/run with space'", escaped.MountPoint)
	}
	if got := parseMounts(""); len(got) != 0 {
		t.Errorf("empty input should yield 0 mounts, got %d", len(got))
	}
}

func TestParseRotational(t *testing.T) {
	if rot, ok := parseRotational("0\n"); !ok || rot == nil || *rot {
		t.Errorf("rotational '0' = %v/%v, want false,true", rot, ok)
	}
	if rot, ok := parseRotational("1"); !ok || rot == nil || !*rot {
		t.Errorf("rotational '1' = %v/%v, want true,true", rot, ok)
	}
	if rot, ok := parseRotational(""); ok || rot != nil {
		t.Errorf("rotational '' = %v/%v, want nil,false", rot, ok)
	}
	if rot, ok := parseRotational("junk"); ok || rot != nil {
		t.Errorf("rotational junk = %v/%v, want nil,false", rot, ok)
	}
}

func TestParseNrRequests(t *testing.T) {
	if got := parseNrRequests("128\n"); got != 128 {
		t.Errorf("nr_requests = %d, want 128", got)
	}
	if got := parseNrRequests(""); got != 0 {
		t.Errorf("nr_requests empty = %d, want 0", got)
	}
	if got := parseNrRequests("junk"); got != 0 {
		t.Errorf("nr_requests junk = %d, want 0", got)
	}
}

func TestBlockDevFromDevice(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"/dev/sda", "sda"},
		{"/dev/sda1", "sda"},
		{"/dev/nvme0n1", "nvme0n1"},
		{"/dev/nvme0n1p2", "nvme0n1"},
		{"/dev/mmcblk0", "mmcblk0"},
		{"/dev/mmcblk0p1", "mmcblk0"},
		{"/dev/vda2", "vda"},
		{"tmpfs", ""},
		{"none", ""},
		{"/dev/mapper/vg-root", "vg-root"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := blockDevFromDevice(tt.in); got != tt.want {
			t.Errorf("blockDevFromDevice(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestProbeBlockClass_NVMeVsHDD(t *testing.T) {
	s := fakeStorageSourcesEmpty()
	s.readFile = func(path string) string {
		switch path {
		case "/sys/block/nvme0n1/queue/rotational":
			return "0\n"
		case "/sys/block/nvme0n1/queue/nr_requests":
			return "256\n"
		case "/sys/block/sda/queue/rotational":
			return "1\n"
		case "/sys/block/sda/queue/nr_requests":
			return "64\n"
		}
		return ""
	}

	typ, rot, nr := probeBlockClass(s, "/dev/nvme0n1p2", "ext4")
	if typ != StorageNVMe {
		t.Errorf("nvme type = %q, want nvme", typ)
	}
	if rot == nil || *rot {
		t.Errorf("nvme rotational = %v, want false", rot)
	}
	if nr != 256 {
		t.Errorf("nvme nr_requests = %d, want 256", nr)
	}

	typ, rot, nr = probeBlockClass(s, "/dev/sda1", "ext4")
	if typ != StorageHDD {
		t.Errorf("sda type = %q, want hdd", typ)
	}
	if rot == nil || !*rot {
		t.Errorf("sda rotational = %v, want true", rot)
	}

	// tmpfs: tipo tmpfs, sem meio físico.
	typ, rot, nr = probeBlockClass(s, "tmpfs", "tmpfs")
	if typ != StorageTmpfs || rot != nil || nr != 0 {
		t.Errorf("tmpfs class = %q/%v/%d, want tmpfs/nil/0", typ, rot, nr)
	}

	// sem sysfs (jaula): unknown + rotational nil + nr 0.
	empty := fakeStorageSourcesEmpty()
	typ, rot, nr = probeBlockClass(empty, "/dev/sda1", "ext4")
	if typ != StorageUnknown || rot != nil || nr != 0 {
		t.Errorf("no-sysfs class = %q/%v/%d, want unknown/nil/0", typ, rot, nr)
	}
}

func TestMountForPath(t *testing.T) {
	// mountForPath opera com literais de mounts POSIX ("/", "/work").
	// No Windows o filepath.Clean converte os separadores para "\" e quebra o
	// prefix-match — a semântica de mounts POSIX não se aplica a esta plataforma.
	if runtime.GOOS == "windows" {
		t.Skip("teste valida semântica de mounts POSIX (literal '/') — não aplicável no Windows")
	}
	mounts := []MountInfo{
		{MountPoint: "/"},
		{MountPoint: "/work"},
		{MountPoint: "/work/projects"},
	}
	if m := mountForPath(mounts, "/work/projects/cosca"); m == nil || m.MountPoint != "/work/projects" {
		t.Errorf("deep path → %+v, want /work/projects (longest-prefix)", m)
	}
	if m := mountForPath(mounts, "/work"); m == nil || m.MountPoint != "/work" {
		t.Errorf("exact path → %+v, want /work", m)
	}
	if m := mountForPath(mounts, "/etc"); m == nil || m.MountPoint != "/" {
		t.Errorf("root fallback → %+v, want /", m)
	}
	if m := mountForPath(nil, "/work"); m != nil {
		t.Errorf("no mounts → want nil, got %+v", m)
	}
	if m := mountForPath(mounts, ""); m != nil {
		t.Errorf("empty path → want nil, got %+v", m)
	}
}

// =============================================================================
// Serialization
// =============================================================================

func TestStorageInfo_JSONSerialization(t *testing.T) {
	rot := false
	info := StorageInfo{
		Mounts: []MountInfo{{
			Device:         "/dev/nvme0n1p2",
			MountPoint:     "/",
			FSType:         "ext4",
			Options:        "rw,relatime",
			TotalBytes:     1 << 40,
			UsedBytes:      300 << 30,
			AvailableBytes: 200 << 30,
			Rotational:     &rot,
			NrRequests:     128,
			DeviceType:     StorageNVMe,
		}},
		DeviceNodes:     []string{"/dev/nvme0n1"},
		Environment:     EnvJail,
		Detected:        true,
		Available:       true,
		Usable:          true,
		Capabilities:    StorageCapabilities{Reported: true, Visible: true, Usable: true, Measured: false, WhyNot: "Fase 9"},
		Provenance:      StorageProvenance{Source: "proc-mounts,statfs,sysfs", Confidence: "high"},
		Limitations:     []string{"benchmark é Fase 9"},
	}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)
	for _, want := range []string{
		`"mount_point":"/"`, `"fstype":"ext4"`, `"device_type":"nvme"`,
		`"total_bytes":1099511627776`, `"rotational":false`, `"nr_requests":128`,
		`"device_nodes":["/dev/nvme0n1"]`, `"environment":"jail"`,
		`"detected":true`, `"available":true`, `"usable":true`,
		`"capabilities"`, `"reported":true`, `"measured":false`,
		`"provenance"`, `"confidence":"high"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("JSON missing %q: %s", want, s)
		}
	}
}

func TestStorageInfo_JSONZeroValue(t *testing.T) {
	info := StorageInfo{}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal zero value: %v", err)
	}
	s := string(data)
	for _, want := range []string{`"detected":false`, `"available":false`, `"usable":false`} {
		if !strings.Contains(s, want) {
			t.Errorf("zero JSON missing %q: %s", want, s)
		}
	}
	// Mounts vazio e rotational nil (desconhecido) são omitidos pelo omitempty.
	// (structs de capabilities/provenance serializam mesmo no zero-value —
	// mesmo comportamento das demais fases, ex: GPUInfo.)
	for _, skip := range []string{`"mounts"`, `"rotational"`, `"device_type"`} {
		if strings.Contains(s, skip) {
			t.Errorf("zero JSON should omit %q: %s", skip, s)
		}
	}
}

func TestMountInfo_RotationalNilOmitted(t *testing.T) {
	m := MountInfo{MountPoint: "/", TotalBytes: 0}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "rotational") {
		t.Errorf("nil rotational should be omitted: %s", string(data))
	}
}
