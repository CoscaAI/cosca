package compute

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// =============================================================================
// Storage Probe — Fase 6 (discovery)
// =============================================================================
//
// A camada que responde: "Quais recursos de armazenamento o processo consegue
// OBSERVAR e UTILIZAR neste ambiente?" (ordem do Don + Professor, L319). Só
// informação observável em user space, SEM privilégio e SEM tocar o
// filesystem (KERNEL ACCESS POLICY — L312):
//   - /proc/mounts: lista de filesystems montados (device, mount point,
//     fstype, options);
//   - syscall.Statfs no mount point: capacidade total/usada/disponível;
//   - /sys/block/<dev>/queue/rotational: "0" = SSD/NVMe, "1" = HDD (leitura);
//     /sys/block/<dev>/queue/nr_requests: fila de I/O do scheduler;
//   - /dev/sd*, /dev/nvme*, /dev/mmc*: device nodes VISÍVEIS (listDir —
//     existência, nunca abrir/escrever).
//
// A cadeia epistemológica do Professor é separada em quatro camadas (L319):
//   - REPORTED: o OS/hardware DECLARA o storage (mounts em /proc/mounts,
//     device nodes em /dev);
//   - VISIBLE:  o processo observa o storage NESTE ambiente (statfs OK no
//     mount point — na jaula bwrap alguns mounts/sysfs não existem);
//   - USABLE:   o storage está utilizável agora (capacidade disponível > 0);
//   - MEASURED: desempenho MEDIDO (benchmark) — SEMPRE false nesta fase;
//     benchmark de storage é a Fase 9, nunca executado aqui (L319).
//
// Toda fonte é best-effort: informação parcial é registrada (zero-value +
// Limitations), nunca um erro que derrube a função. Nada é montado, alterado
// ou formatado — só observação.

// DeviceType classifica o meio físico do storage quando observável.
type DeviceType string

const (
	StorageUnknown DeviceType = "unknown"
	StorageNVMe    DeviceType = "nvme"
	StorageSSD     DeviceType = "ssd"
	StorageHDD     DeviceType = "hdd"
	StorageTmpfs   DeviceType = "tmpfs" // RAM-backed — relevante para o auto-jail
)

// MountInfo é a fotografia read-only de um filesystem montado (Fase 6).
// Unidades em BYTES (0 = desconhecido). JSON serializável.
type MountInfo struct {
	Device     string `json:"device,omitempty"` // ex: /dev/nvme0n1p2 ("" para pseudofs: tmpfs, proc, ...)
	MountPoint string `json:"mount_point"`
	FSType     string `json:"fstype,omitempty"`
	Options    string `json:"options,omitempty"`

	// Capacidade (de syscall.Statfs no mount point — user space, sem privilégio).
	TotalBytes     uint64 `json:"total_bytes,omitempty"`
	UsedBytes      uint64 `json:"used_bytes,omitempty"`
	AvailableBytes uint64 `json:"available_bytes,omitempty"` // Bavail — disponível ao processo não-root

	// Rotational: nil = meio físico desconhecido; false = SSD/NVMe; true = HDD
	// (de /sys/block/<dev>/queue/rotational — leitura).
	Rotational *bool `json:"rotational,omitempty"`
	// NrRequests: tamanho da fila de I/O do scheduler (/sys/block/<dev>/queue/nr_requests). 0 = desconhecido.
	NrRequests int `json:"nr_requests,omitempty"`
	// DeviceType: meio físico classificado (nvme/ssd/hdd/tmpfs/unknown).
	DeviceType DeviceType `json:"device_type,omitempty"`
}

// StorageCapabilities é a cadeia do Professor (L319) — reported ≠ visible ≠
// usable ≠ measured — espelhando as 3 camadas da Fase 5 (GPU).
type StorageCapabilities struct {
	Reported bool `json:"reported"` // OS declara o storage (mounts / device nodes)
	Visible  bool `json:"visible"`  // observável NESTE ambiente (statfs OK)
	Usable   bool `json:"usable"`   // utilizável agora (capacidade disponível > 0)
	Measured bool `json:"measured"` // desempenho MEDIDO — SEMPRE false nesta fase (F9)
	WhyNot   string `json:"why_not,omitempty"`
}

// StorageProvenance registra a origem de cada fato (Fase 5 style).
type StorageProvenance struct {
	Source     string    `json:"source,omitempty"`
	Timestamp  time.Time `json:"timestamp,omitempty"`
	Evidence   string    `json:"evidence,omitempty"`
	Confidence string    `json:"confidence,omitempty"`
}

// StorageInfo é a fotografia read-only de descoberta de armazenamento (Fase 6).
// JSON serializável; unidades em BYTES.
type StorageInfo struct {
	// Mounts: filesystems montados observados (em /proc/mounts), cada um com
	// capacidade e meio físico quando determináveis.
	Mounts []MountInfo `json:"mounts,omitempty"`

	// DeviceNodes: device nodes de block visíveis em /dev (sd*/nvme*/mmc*) —
	// SÓ existência (listDir), nunca abertos/escritos (KERNEL ACCESS POLICY).
	DeviceNodes []string `json:"device_nodes,omitempty"`

	// Environment: contexto de execução (reuso do ProbeEnvironment, Fase 4) —
	// explica o que é visível na jaula vs host.
	Environment EnvironmentType `json:"environment,omitempty"`

	// Focos do Cosca (o que importa para a casa):
	KnowledgeDBMount *MountInfo `json:"knowledge_db_mount,omitempty"` // o filesystem do cofre (knowledge.db)
	WorkspaceMount   *MountInfo `json:"workspace_mount,omitempty"`    // o filesystem raiz do workspace
	TmpMount         *MountInfo `json:"tmp_mount,omitempty"`          // /tmp (tmpfs = RAM, relevante ao auto-jail)

	// Camadas detected/available/usable (padrão das Fases 1-5).
	Detected  bool `json:"detected"`
	Available bool `json:"available"`
	Usable    bool `json:"usable"`

	// Capabilities: a cadeia reported/visible/usable/measured (L319).
	Capabilities StorageCapabilities `json:"capabilities,omitempty"`

	// Provenance: fonte, timestamp, evidência e confiança (Fase 5 style).
	Provenance StorageProvenance `json:"provenance,omitempty"`

	// Limitations registra o que não foi determinável em user space (e por quê).
	Limitations []string `json:"limitations,omitempty"`
}

// statfsInfo é a projeção read-only dos campos de syscall.Statfs_t que o probe
// usa — tudo convertido para BYTES. Injetável para testes.
type statfsInfo struct {
	TotalBytes     uint64
	FreeBytes      uint64
	AvailableBytes uint64
}

// storageSources abstrai as fontes read-only para o probeStorage poder ser
// exercitado contra um ambiente fake nos testes. Produção usa
// defaultStorageSources.
type storageSources struct {
	readFile func(path string) string
	listDir  func(path string) []string
	stat     func(path string) bool
	// statfs devolve a capacidade de um path em bytes (total/free/available) e
	// false quando o mount point não é statfs-ável NESTE ambiente.
	statfs func(path string) (statfsInfo, bool)
	// environment classifica o ambiente ATUAL (jail/container/bare-metal/...)
	// a partir do ProbeEnvironment — o contexto da visibilidade (Fase 4).
	environment func() EnvironmentInfo
	// getwd devolve o diretório de trabalho atual (raiz do workspace).
	getwd func() (string, bool)
	// knowledgeDB devolve o caminho absoluto do knowledge.db (o cofre) se
	// determinável; "" se não.
	knowledgeDB func() string
}

// ProbeStorage descobre os recursos de armazenamento observáveis/utilizáveis.
// Estritamente read-only e best-effort: nenhuma fonte falha a função como um
// todo — informação parcial é registrada no modelo (zero-value + Limitations).
func ProbeStorage() StorageInfo {
	return probeStorage(defaultStorageSources())
}

func probeStorage(s storageSources) StorageInfo {
	started := time.Now()
	info := StorageInfo{}

	// Contexto de execução (Fase 4) — explica o que é visível vs o host.
	if s.environment != nil {
		env := s.environment()
		info.Environment = env.Type
		if env.InJail {
			info.Limitations = append(info.Limitations,
				"visibilidade de storage restrita pela jaula (bwrap): mounts e /sys/block refletem o sandbox, não o host completo")
		}
	}

	// 1. Filesystems montados (/proc/mounts) + capacidade (statfs) + meio físico (sysfs).
	mounts := parseMounts(s.readFile("/proc/mounts"))
	info.Mounts = make([]MountInfo, 0, len(mounts))
	var statfsOK, usableCount int
	unclassified := make(map[string]bool) // block devs sem rotational observável (dedupe)
	for _, m := range mounts {
		mount := m
		if sf, ok := s.statfs(mount.MountPoint); ok {
			mount.TotalBytes = sf.TotalBytes
			if sf.TotalBytes >= sf.FreeBytes {
				mount.UsedBytes = sf.TotalBytes - sf.FreeBytes
			}
			mount.AvailableBytes = sf.AvailableBytes
			statfsOK++
			if mount.AvailableBytes > 0 {
				usableCount++
			}
		}
		mount.DeviceType, mount.Rotational, mount.NrRequests = probeBlockClass(s, mount.Device, mount.FSType)
		if mount.DeviceType == StorageUnknown && blockDevFromDevice(mount.Device) != "" {
			unclassified[blockDevFromDevice(mount.Device)] = true
		}
		info.Mounts = append(info.Mounts, mount)
	}

	// 2. Device nodes de block visíveis em /dev (existência apenas).
	info.DeviceNodes = probeStorageDeviceNodes(s)

	// 3. Focos do Cosca: o cofre (knowledge.db), o workspace e /tmp.
	if kb := s.knowledgeDB(); kb != "" {
		info.KnowledgeDBMount = mountForPath(info.Mounts, kb)
		if info.KnowledgeDBMount == nil {
			info.Limitations = append(info.Limitations,
				"filesystem do knowledge.db (o cofre) não resolvível sobre os mounts observados")
		}
	} else {
		info.Limitations = append(info.Limitations,
			"filesystem do knowledge.db (o cofre) não determinável em user space")
	}
	if wd, ok := s.getwd(); ok {
		info.WorkspaceMount = mountForPath(info.Mounts, wd)
	} else {
		info.Limitations = append(info.Limitations,
			"filesystem raiz do workspace não determinável (getcwd indisponível)")
	}
	info.TmpMount = mountForPath(info.Mounts, "/tmp")

	// 4. Camadas detected/available/usable (padrão Fases 1-5).
	info.Detected = len(info.Mounts) > 0
	info.Available = info.Detected && statfsOK > 0
	info.Usable = info.Available && usableCount > 0

	// 5. Cadeia do Professor (L319): reported ≠ visible ≠ usable ≠ measured.
	cap := &info.Capabilities
	cap.Reported = info.Detected || len(info.DeviceNodes) > 0
	cap.Visible = statfsOK > 0
	cap.Usable = usableCount > 0
	cap.Measured = false
	var whyNot []string
	whyNot = append(whyNot, "benchmark de armazenamento (measured) é a Fase 9 — não executado nesta rodada")
	if !cap.Reported {
		whyNot = append(whyNot, "nenhum mount nem device node de block observado")
	} else if !cap.Visible {
		whyNot = append(whyNot, "mounts reportados mas nenhum statfs observável neste ambiente (jaula restringe)")
	} else if !cap.Usable {
		whyNot = append(whyNot, "capacidade reportada mas sem espaço disponível observável")
	}
	cap.WhyNot = strings.Join(whyNot, "; ")

	// 6. Limitações registradas — nunca contornadas (KERNEL ACCESS POLICY).
	if !info.Detected {
		info.Limitations = append(info.Limitations,
			"nenhum mount observável em /proc/mounts (não legível neste ambiente)")
	} else if statfsOK < len(info.Mounts) {
		info.Limitations = append(info.Limitations,
			fmt.Sprintf("%d mount(s) sem capacidade observável (statfs indisponível para o mount point neste ambiente)",
				len(info.Mounts)-statfsOK))
	}
	if len(unclassified) > 0 {
		devs := make([]string, 0, len(unclassified))
		for d := range unclassified {
			devs = append(devs, d)
		}
		sort.Strings(devs)
		limit := len(devs)
		if limit > 3 {
			limit = 3
		}
		info.Limitations = append(info.Limitations,
			"tipo de meio físico (rotational) não observável em user space para: "+strings.Join(devs[:limit], ", "))
	}
	info.Limitations = append(info.Limitations,
		"capacidade de I/O e latência de armazenamento não mensuráveis sem benchmark (measured = Fase 9 — não executado nesta rodada)")

	// 7. Provenance (Fase 5 style).
	prov := &info.Provenance
	prov.Timestamp = started
	prov.Source = "proc-mounts(/proc/mounts),statfs(syscall),sysfs(/sys/block),/dev(listDir),ProbeEnvironment"
	prov.Evidence = fmt.Sprintf("mounts=%d statfs_ok=%d device_nodes=%d",
		len(info.Mounts), statfsOK, len(info.DeviceNodes))
	switch {
	case info.Detected && statfsOK > 0 && len(info.DeviceNodes) > 0:
		prov.Confidence = "high"
	case info.Detected:
		prov.Confidence = "medium"
	default:
		prov.Confidence = "low"
	}

	return info
}

// =============================================================================
// Default (real) sources
// =============================================================================

func defaultStorageSources() storageSources {
	return storageSources{
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
		stat: func(path string) bool {
			_, err := os.Stat(path)
			return err == nil
		},
		// statfs: fonte platform-specific (syscall.Statfs no Linux;
		// GetDiskFreeSpaceEx no Windows — ver storageprobe_linux.go /
		// storageprobe_windows.go).
		statfs: defaultStatfs,
		environment: func() EnvironmentInfo { return ProbeEnvironment() },
		getwd: func() (string, bool) {
			wd, err := os.Getwd()
			return wd, err == nil
		},
		knowledgeDB: resolveKnowledgeDBPath,
	}
}

// resolveKnowledgeDBPath determina o caminho absoluto do knowledge.db (o cofre).
// Ordem: COSCA_PROJECT_DIR (se definida) → walk-up a partir do cwd procurando o
// diretório ".cosca" (mesma heurística do filesystem.CoscaProjectDir, sem
// acoplar o compute ao filesystem) → fallback ".cosca/knowledge.db" sob o cwd.
// Best-effort: devolve "" apenas se nem cwd nem a variável forem resolvíveis.
func resolveKnowledgeDBPath() string {
	if env := os.Getenv("COSCA_PROJECT_DIR"); env != "" {
		return filepath.Join(env, "knowledge.db")
	}
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	dir := wd
	for {
		if info, err := os.Stat(filepath.Join(dir, ".cosca")); err == nil && info.IsDir() {
			return filepath.Join(dir, ".cosca", "knowledge.db")
		}
		for _, marker := range []string{"go.mod", "package.json", ".git", "Cargo.toml", "pyproject.toml"} {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return filepath.Join(dir, ".cosca", "knowledge.db")
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return filepath.Join(wd, ".cosca", "knowledge.db")
		}
		dir = parent
	}
}

// =============================================================================
// Parsers (puros, unit-testáveis)
// =============================================================================

// parseMounts lê /proc/mounts (user space puro). Formato por linha:
// "<device> <mountpoint> <fstype> <options> <dump> <pass>". Linhas malformadas
// são ignoradas — a função nunca falha.
func parseMounts(out string) []MountInfo {
	var mounts []MountInfo
	for _, line := range strings.Split(out, "\n") {
		if m, ok := parseMountLine(line); ok {
			mounts = append(mounts, m)
		}
	}
	return mounts
}

// parseMountLine decodifica uma linha de /proc/mounts, desescapando o mount
// point (espaços/tabs/newlines são escape-octal \040 \011 \012).
func parseMountLine(line string) (MountInfo, bool) {
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return MountInfo{}, false
	}
	mp := strings.ReplaceAll(fields[1], `\040`, " ")
	mp = strings.ReplaceAll(mp, `\011`, "\t")
	mp = strings.ReplaceAll(mp, `\012`, "\n")
	mp = strings.ReplaceAll(mp, `\134`, `\`)
	return MountInfo{
		Device:     fields[0],
		MountPoint: mp,
		FSType:     fields[2],
		Options:    fields[3],
	}, true
}

// parseRotational lê /sys/block/<dev>/queue/rotational: "0" = SSD/NVMe,
// "1" = HDD. Devolve (nil, false) quando não observável em user space.
func parseRotational(raw string) (*bool, bool) {
	n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return nil, false
	}
	b := n == 1
	return &b, true
}

// parseNrRequests lê /sys/block/<dev>/queue/nr_requests (fila de I/O do
// scheduler). 0 quando não observável.
func parseNrRequests(raw string) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return n
}

// probeBlockClass classifica o meio físico de um mount quando observável em
// user space (/sys/block/<dev>/queue/*). NUNCA abre device nodes — só lê
// sysfs. tmpfs é RAM (relevante ao auto-jail). Devolve (deviceType,
// rotational *bool — nil = desconhecido, nrRequests).
func probeBlockClass(s storageSources, device, fstype string) (DeviceType, *bool, int) {
	if fstype == "tmpfs" {
		return StorageTmpfs, nil, 0
	}
	dev := blockDevFromDevice(device)
	if dev == "" {
		return StorageUnknown, nil, 0
	}
	rot, ok := parseRotational(s.readFile("/sys/block/" + dev + "/queue/rotational"))
	nr := parseNrRequests(s.readFile("/sys/block/" + dev + "/queue/nr_requests"))
	typ := StorageUnknown
	switch {
	case strings.HasPrefix(dev, "nvme"):
		typ = StorageNVMe
	case ok && rot != nil && *rot:
		typ = StorageHDD
	case ok && rot != nil && !*rot:
		typ = StorageSSD
	}
	return typ, rot, nr
}

// blockDevFromDevice extrai o nome do block device (base) de um device de
// mount (ex: "/dev/sda1" → "sda", "/dev/nvme0n1p2" → "nvme0n1",
// "/dev/mmcblk0p1" → "mmcblk0"). Retorna "" para dispositivos não-block
// (tmpfs, proc, overlay, ...).
func blockDevFromDevice(device string) string {
	if !strings.HasPrefix(device, "/dev/") {
		return ""
	}
	base := filepath.Base(device)
	switch {
	case strings.HasPrefix(base, "nvme"):
		// nvme0n1 [p<N>] — strip apenas do sufixo "p<dígitos>".
		if i := strings.LastIndex(base, "p"); i > 0 && isDigits(base[i+1:]) {
			return base[:i]
		}
		return base
	case strings.HasPrefix(base, "mmcblk"):
		// mmcblk0 [p<N>] — strip apenas do sufixo "p<dígitos>".
		if i := strings.LastIndex(base, "p"); i > 0 && isDigits(base[i+1:]) {
			return base[:i]
		}
		return base
	case strings.HasPrefix(base, "sd"), strings.HasPrefix(base, "vd"),
		strings.HasPrefix(base, "hd"), strings.HasPrefix(base, "xvd"):
		// sda1 / vdX2 / hda1 / xvda1 → strip dígitos finais.
		return stripTrailingDigits(base)
	default:
		return stripTrailingDigits(base)
	}
}

// stripTrailingDigits remove os dígitos do final de um nome (sufixo de
// partição). "sda1" → "sda"; "sda" → "sda"; "123" → "".
func stripTrailingDigits(name string) string {
	i := len(name)
	for i > 0 && name[i-1] >= '0' && name[i-1] <= '9' {
		i--
	}
	return name[:i]
}

// isDigits reporta se s é não-vazio e formado apenas por dígitos.
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// probeStorageDeviceNodes lista os device nodes de block visíveis em /dev
// (sd*/nvme*/mmc*) — SÓ existência (listDir), nunca abrir/escrever
// (KERNEL ACCESS POLICY, L312).
func probeStorageDeviceNodes(s storageSources) []string {
	var nodes []string
	for _, name := range s.listDir("/dev") {
		for _, prefix := range []string{"sd", "nvme", "mmc"} {
			if strings.HasPrefix(name, prefix) {
				nodes = append(nodes, "/dev/"+name)
				break
			}
		}
	}
	sort.Strings(nodes)
	return nodes
}

// mountForPath devolve o mount que cobre um path absoluto (longest-prefix
// match). nil se nenhum mount observado cobrir o path.
func mountForPath(mounts []MountInfo, path string) *MountInfo {
	if path == "" || len(mounts) == 0 {
		return nil
	}
	abs := path
	if !filepath.IsAbs(abs) {
		abs = filepath.Clean(abs)
	}
	best, bestLen := -1, -1
	for i := range mounts {
		mp := mounts[i].MountPoint
		if mp == "" {
			continue
		}
		// O mount raiz ("/") cobre todo path absoluto; os demais cobrem por
		// longest-prefix exato (abs == mp ou abs dentro de mp).
		if abs == mp || (mp == "/" && strings.HasPrefix(abs, "/")) || strings.HasPrefix(abs, mp+"/") {
			if len(mp) > bestLen {
				bestLen = len(mp)
				best = i
			}
		}
	}
	if best < 0 {
		return nil
	}
	m := mounts[best]
	return &m
}
