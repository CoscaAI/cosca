package compute

import (
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// =============================================================================
// GPU Probe
// =============================================================================

// GPUVendor identifies the GPU vendor family.
type GPUVendor string

const (
	GPUAmd    GPUVendor = "amd"
	GPUNvidia GPUVendor = "nvidia"
	GPUIntel  GPUVendor = "intel"
	GPUApple  GPUVendor = "apple"
	GPUNone   GPUVendor = "none"
)

// GPUInfo is a read-only snapshot of the detected GPU compute capacity.
// Unlike detectGPU (discovery), it detects CAPABILITY — driver, API surface
// and compute target — not just a brand name.
type GPUInfo struct {
	Vendor      GPUVendor `json:"vendor"`
	Model       string    `json:"model,omitempty"`
	VRAMGB      int       `json:"vram_gb,omitempty"`
	Driver      string    `json:"driver,omitempty"` // "rocm-smi" | "nvidia-smi" | "vulkan" | ""
	HasROCm     bool      `json:"has_rocm"`
	HasVulkan   bool      `json:"has_vulkan"`
	HasCUDA     bool      `json:"has_cuda"`
	Compute     string    `json:"compute,omitempty"`      // "gfx1031" | "sm_80" | ""
	ROCmVersion string    `json:"rocm_version,omitempty"` // e.g. "7.2.4"
	// HasVAAPI indica aceleração de vídeo (AMD VA-API / Intel QuickSync) —
	// usada pelo FFmpeg para transcode/preview sem tocar a CPU (§17/§28).
	HasVAAPI bool `json:"has_vaapi"`
	// HasOpenCL indica runtime OpenCL disponível.
	HasOpenCL bool `json:"has_opencl"`
	// ComputeUnits é o número de CUs (AMD) / SM (NVIDIA) / EU (Intel).
	ComputeUnits int `json:"compute_units,omitempty"`
	// MaxClockMHz é o clock máximo da GPU.
	MaxClockMHz int `json:"max_clock_mhz,omitempty"`

	// =============================================================================
	// Fase 5 — GPU como AMBIENTE COMPUTACIONAL (6 dimensões do Professor, L318).
	// Campos ADITIVOS — backward compatible com o JSON existente (omitempty).
	// =============================================================================

	// Resources: bandwidth de memória em GB/s. 0 = desconhecido (rocm-smi não
	// expõe; medição real exige microbenchmark — fase posterior).
	BandwidthGBs int64 `json:"bandwidth_gbs,omitempty"`

	// Runtime: versões observáveis das APIs (vulkaninfo / clinfo).
	VulkanVersion string `json:"vulkan_version,omitempty"`
	OpenCLVersion string `json:"opencl_version,omitempty"`

	// Visibility (dimensão 4): a GPU é visível no ambiente ATUAL (host/jail/
	// container)? Device nodes via stat (leitura de existência — KERNEL ACCESS
	// POLICY L312: nunca abrir/escrever).
	Visibility GPUVisibility `json:"visibility,omitempty"`

	// Capabilities (dimensão 5): as 3 camadas do Professor — reported →
	// actually_visible → experimentally_verified (a última é fase posterior).
	Capabilities GPUCapabilities `json:"capabilities,omitempty"`

	// Provenance (dimensão 6): fonte, timestamp, evidência e confiança de cada
	// fato (cadeia epistemológica detected ≠ available ≠ usable, L317).
	Provenance GPUProvenance `json:"provenance,omitempty"`

	// Limitations registra o que não foi determinável sem privilégio/no jail.
	Limitations []string `json:"limitations,omitempty"`
}

// GPUVisibility (dimensão VISIBILITY, Fase 5) — a GPU é visível no ambiente
// computacional ATUAL? Somente stat de device nodes (leitura de existência),
// nunca abrir/escrever (KERNEL ACCESS POLICY, L312).
type GPUVisibility struct {
	// Visible: pelo menos um device node de GPU presente (stat ok) NESTE ambiente.
	Visible bool `json:"visible"`
	// DeviceNodes: device nodes presentes observados (ex: /dev/kfd, /dev/dri/renderD128).
	DeviceNodes []string `json:"device_nodes,omitempty"`
	// Environment: classificação do ambiente atual (do ProbeEnvironment —
	// jail/container/bare-metal/...), o contexto que explica a visibilidade.
	Environment string `json:"environment,omitempty"`
	// Refs: por que a classificação foi dada (evidência textual curta).
	Refs []string `json:"refs,omitempty"`
}

// GPUCapabilities (dimensão CAPABILITIES, Fase 5) — as 3 camadas do Professor:
//   - Reported:            o hardware/OS DECLARA a GPU (sysfs/lspci/rocm-smi/rocminfo);
//   - ActuallyVisible:     o processo consegue VER a GPU neste ambiente (device nodes);
//   - ExperimentallyVerified: verificado por EXPERIMENTO (ex: compute shader mínimo) —
//     SEMPRE false nesta fase (sem benchmark/experimento, L318), explicado em WhyNot.
type GPUCapabilities struct {
	Reported               bool   `json:"reported"`
	ActuallyVisible        bool   `json:"actually_visible"`
	ExperimentallyVerified bool   `json:"experimentally_verified"`
	VerifiedAt             string `json:"verified_at,omitempty"`
	WhyNot                 string `json:"why_not,omitempty"`
}

// GPUProvenance (dimensão PROVENANCE, Fase 5) — a origem de cada fato.
type GPUProvenance struct {
	// Source: fontes observadas (ex: "rocminfo/rocm-smi,sysfs,device-nodes(stat)").
	Source string `json:"source,omitempty"`
	// Timestamp: quando a fotografia foi tirada.
	Timestamp time.Time `json:"timestamp,omitempty"`
	// Evidence: dado bruto relevante (truncado).
	Evidence string `json:"evidence,omitempty"`
	// Confidence: high | medium | low conforme a cadeia epistemológica.
	Confidence string `json:"confidence,omitempty"`
}

// gpuSources abstracts the OS-level read-only probes so probeGPU can be
// exercised against a fake environment in tests. Production uses defaultSources.
type gpuSources struct {
	runCommand func(name string, args ...string) string
	hasCommand func(name string) bool
	readFile   func(path string) string
	stat       func(path string) bool
	listDir    func(path string) []string
	readVendor func() GPUVendor
	// environment classifica o ambiente ATUAL (jail/container/bare-metal/...)
	// a partir do ProbeEnvironment — usado na dimensão VISIBILITY (Fase 5).
	environment func() string
}

// ProbeGPU detects the machine GPU capacity. It is strictly read-only
// (no driver install, no sudo): every source is best-effort and failures are
// skipped, never fatal. If nothing is found it returns GPUInfo{Vendor: GPUNone}.
func ProbeGPU() GPUInfo {
	return probeGPU(defaultSources())
}

func probeGPU(s gpuSources) GPUInfo {
	started := time.Now()
	info := GPUInfo{Vendor: GPUNone}

	if runtime.GOOS == "darwin" {
		probeApple(s, &info)
		probeF5Dimensions(s, &info, started)
		return info
	}

	// 1. Vendor hint from the kernel DRM subsystem (/sys/class/drm/card*/device/vendor).
	if v := s.readVendor(); v != "" {
		info.Vendor = v
	}

	// 2. Vendor-specific deep probes (model / VRAM / compute / ROCm / CUDA).
	switch info.Vendor {
	case GPUAmd:
		probeAMD(s, &info)
	case GPUNvidia:
		probeNVIDIA(s, &info)
	case GPUIntel:
		probeIntel(s, &info)
	}

	// 3. If the sysfs hint was missing, discover the vendor via CLI tools.
	if info.Vendor == GPUNone {
		switch {
		case s.hasCommand("nvidia-smi"):
			info.Vendor = GPUNvidia
			probeNVIDIA(s, &info)
		case rocmBin(s, "rocm-smi") != "" || rocmBin(s, "rocminfo") != "":
			info.Vendor = GPUAmd
			probeAMD(s, &info)
		}
	}

	// 4. Cross-vendor API surface (Vulkan ICD manifests / vulkaninfo).
	probeVulkan(s, &info)

	// 5. Aceleração de vídeo (VA-API) e compute genérico (OpenCL).
	probeVAAPI(s, &info)
	probeOpenCL(s, &info)

	// 6. Model fallback from the PCI bus (lspci).
	if info.Model == "" {
		probeLspci(s, &info)
	}

	// Defensive: an empty vendor always collapses to the explicit none value.
	if info.Vendor == "" {
		info.Vendor = GPUNone
	}

	// 7. Fase 5 — as 6 dimensões da GPU como ambiente computacional (L318).
	probeF5Dimensions(s, &info, started)

	return info
}

// probeF5Dimensions popula as 3 novas dimensões do Professor (Visibility,
// Capabilities, Provenance) e registra as limitações de Resources/Runtime
// (bandwidth não observável; verificação experimental adiada). Best-effort:
// nunca falha, parcial → Limitations.
func probeF5Dimensions(s gpuSources, info *GPUInfo, started time.Time) {
	probeVisibility(s, info)
	probeCapabilities(s, info)
	probeProvenance(s, info, started)

	// Resources: bandwidth de memória não é exposto por rocm-smi/nvidia-smi em
	// user space — medição real exige microbenchmark (fase posterior). Registrado.
	if info.Vendor != GPUNone && info.BandwidthGBs == 0 {
		info.Limitations = append(info.Limitations,
			"bandwidth de memória não exposta pelas ferramentas em user space (rocm-smi/nvidia-smi) — medição real é fase posterior (microbenchmark)")
	}
	// Epistemologia (L317): GPU declarada por lspci/Vulkan mas com vendor não
	// resolvível neste ambiente (sysfs DRM e ferramentas de vendor não expostos)
	// — reportada ≠ identificável ≠ usável.
	if info.Vendor == GPUNone && info.Model != "" {
		info.Limitations = append(info.Limitations,
			"vendor da GPU não identificável neste ambiente (sysfs DRM e ferramentas ROCm/CUDA não expostos na jaula) — GPU declarada por lspci/Vulkan apenas")
	}
}

// =============================================================================
// Default (real) sources
// =============================================================================

func defaultSources() gpuSources {
	return gpuSources{
		runCommand: runCommand,
		hasCommand: func(name string) bool {
			_, err := exec.LookPath(name)
			return err == nil
		},
		readFile: func(path string) string {
			data, err := os.ReadFile(path)
			if err != nil {
				return ""
			}
			return strings.TrimSpace(string(data))
		},
		stat: func(path string) bool {
			_, err := os.Stat(path)
			return err == nil
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
		readVendor: sysfsVendor,
		environment: func() string {
			return string(probeEnvironment(defaultEnvSources()).Type)
		},
	}
}

// runCommand executes a read-only command and returns trimmed stdout.
// A non-zero exit or an absent binary yields "" — never an error.
func runCommand(name string, args ...string) string {
	if name == "" {
		return ""
	}
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// sysfsVendor maps the DRM PCI vendor id (e.g. 0x1002) to a vendor.
func sysfsVendor() GPUVendor {
	entries, err := os.ReadDir("/sys/class/drm")
	if err != nil {
		return ""
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "card") {
			continue
		}
		data, err := os.ReadFile("/sys/class/drm/" + name + "/device/vendor")
		if err != nil {
			continue
		}
		switch strings.TrimSpace(string(data)) {
		case "0x1002": // Advanced Micro Devices
			return GPUAmd
		case "0x10de": // NVIDIA
			return GPUNvidia
		case "0x8086": // Intel
			return GPUIntel
		}
	}
	return ""
}

// =============================================================================
// Vendor probes
// =============================================================================

// rocmBin resolves an ROCm tool, preferring PATH and then /opt/rocm*/bin.
func rocmBin(s gpuSources, tool string) string {
	if s.hasCommand(tool) {
		return tool
	}
	for _, dir := range s.listDir("/opt") {
		if !strings.HasPrefix(dir, "rocm") {
			continue
		}
		p := "/opt/" + dir + "/bin/" + tool
		if s.stat(p) {
			return p
		}
	}
	return ""
}

func probeAMD(s gpuSources, info *GPUInfo) {
	info.Vendor = GPUAmd

	// Marketing name + gfx ISA target from rocminfo (read-only).
	if ri := rocmBin(s, "rocminfo"); ri != "" {
		if out := s.runCommand(ri); out != "" {
			model, gfx := parseRocminfo(out)
			if info.Model == "" {
				info.Model = model
			}
			if info.Compute == "" {
				info.Compute = gfx
			}
			info.HasROCm = true
			// Compute units + max clock (detalhe do §17) quando presentes.
			if cu := cusFromRocminfo(out); cu > 0 && info.ComputeUnits == 0 {
				info.ComputeUnits = cu
			}
			if clk := clockFromRocminfo(out); clk > 0 && info.MaxClockMHz == 0 {
				info.MaxClockMHz = clk
			}
		}
	}

	// VRAM + gfx ISA target from rocm-smi (read-only).
	if smi := rocmBin(s, "rocm-smi"); smi != "" {
		info.Driver = "rocm-smi"
		if out := s.runCommand(smi, "--showmeminfo", "vram"); out != "" {
			if b := vramBytesFromRocmSMI(out); b > 0 {
				info.VRAMGB = bytesToGB(b)
			}
		}
		if out := s.runCommand(smi, "--showproductname"); out != "" {
			if g := gfxFromRocmSMI(out); info.Compute == "" && g != "" {
				info.Compute = g
			}
		}
	}

	// ROCm availability via the KFD kernel device and /opt/rocm* installs.
	if s.stat("/dev/kfd") {
		info.HasROCm = true
	}
	if rocmInstallPresent(s) {
		info.HasROCm = true
	}
	if v := detectROCmVersion(s); v != "" {
		info.ROCmVersion = v
	}
}

func probeNVIDIA(s gpuSources, info *GPUInfo) {
	info.Vendor = GPUNvidia
	if !s.hasCommand("nvidia-smi") {
		return
	}
	info.HasCUDA = true
	info.Driver = "nvidia-smi"

	if out := s.runCommand("nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader"); out != "" {
		parts := strings.SplitN(out, ",", 2)
		if len(parts) > 0 {
			info.Model = strings.TrimSpace(parts[0])
		}
		if len(parts) > 1 {
			info.VRAMGB = parseNvidiaVRAM(parts[1])
		}
	}
	if cc := s.runCommand("nvidia-smi", "--query-gpu=compute_cap", "--format=csv,noheader"); cc != "" {
		info.Compute = nvidiaComputeCap(cc)
	}
}

func probeIntel(s gpuSources, info *GPUInfo) {
	info.Vendor = GPUIntel
}

func probeApple(s gpuSources, info *GPUInfo) {
	info.Vendor = GPUApple
	if out := s.runCommand("sysctl", "-n", "machdep.cpu.brand_string"); out != "" {
		info.Model = out
	}
}

// =============================================================================
// Fase 5 — dimensões: VISIBILITY / CAPABILITIES / PROVENANCE (L318)
// =============================================================================

// probeVisibility (dimensão VISIBILITY) detecta se a GPU é visível no ambiente
// ATUAL (host vs jail vs container). Somente stat de device nodes (leitura de
// existência) — NUNCA abrir/escrever (KERNEL ACCESS POLICY, L312). Best-effort:
// ausência de nodes vira Refs/Limitations, nunca falha.
func probeVisibility(s gpuSources, info *GPUInfo) {
	v := &info.Visibility
	if s.environment != nil {
		v.Environment = s.environment()
	}

	var nodes []string
	if s.stat("/dev/kfd") {
		nodes = append(nodes, "/dev/kfd")
		v.Refs = append(v.Refs, "/dev/kfd presente (AMD KFD — kernel fusion driver) neste ambiente")
	}
	if s.stat("/dev/nvidia0") {
		nodes = append(nodes, "/dev/nvidia0")
		v.Refs = append(v.Refs, "/dev/nvidia0 presente (NVIDIA control device) neste ambiente")
	}
	for _, name := range s.listDir("/dev/dri") {
		if !strings.HasPrefix(name, "renderD") {
			continue
		}
		p := "/dev/dri/" + name
		if s.stat(p) {
			nodes = append(nodes, p)
			v.Refs = append(v.Refs, p+" presente (DRM render node) neste ambiente")
		}
	}
	sort.Strings(nodes)
	v.DeviceNodes = nodes
	v.Visible = len(nodes) > 0
	if !v.Visible {
		v.Refs = append(v.Refs,
			"nenhum device node de GPU presente neste ambiente (stat de /dev/kfd, /dev/nvidia0 e /dev/dri/renderD* falhou)")
	}
}

// probeCapabilities (dimensão CAPABILITIES) aplica as 3 camadas do Professor:
// Reported (o hardware/OS declara) → ActuallyVisible (o processo enxerga neste
// ambiente) → ExperimentallyVerified (verificado por experimento). A última é
// SEMPRE false nesta fase — nenhum benchmark/experimento é executado (L318).
func probeCapabilities(s gpuSources, info *GPUInfo) {
	c := &info.Capabilities
	// Reported: o hardware/OS DECLARA a GPU — via sysfs (vendor) ou via lspci
	// (model encontrado em linha VGA/3D). Um vendor "none" com model presente
	// significa: GPU existe, mas a identidade de vendor não é resolvível aqui.
	c.Reported = info.Vendor != GPUNone || info.Model != ""
	c.ActuallyVisible = info.Visibility.Visible
	c.ExperimentallyVerified = false
	c.VerifiedAt = ""

	reasons := []string{"verificação experimental (compute shader mínimo / microbenchmark) é fase posterior — não executada nesta rodada"}
	switch {
	case c.Reported && !c.ActuallyVisible:
		reasons = append(reasons, "GPU reportada pelo hardware/OS mas sem device nodes visíveis neste ambiente (jail/container pode restringir visibilidade)")
		if info.Vendor == GPUNone {
			reasons = append(reasons, "vendor não identificável neste ambiente (sysfs DRM e ferramentas de vendor não expostos) — GPU declarada por lspci/Vulkan apenas")
		}
	case !c.Reported:
		reasons = append(reasons, "nenhuma GPU reportada (GPUNone)")
	}
	c.WhyNot = strings.Join(reasons, "; ")
}

// probeProvenance (dimensão PROVENANCE) registra a origem de cada fato: fonte,
// timestamp, evidência bruta (truncada) e confiança conforme a cadeia
// epistemológica detected ≠ available ≠ usable (L317). Nunca assume — se o ROCm
// não é verificável no ambiente, available cai e a confiança reduz.
func probeProvenance(s gpuSources, info *GPUInfo, started time.Time) {
	p := &info.Provenance
	p.Timestamp = started

	var sources []string
	switch {
	case info.HasROCm || info.Driver == "rocm-smi":
		sources = append(sources, "rocminfo/rocm-smi")
	case info.HasCUDA:
		sources = append(sources, "nvidia-smi")
	}
	if info.HasVulkan {
		sources = append(sources, "vulkaninfo")
	}
	if info.HasOpenCL {
		sources = append(sources, "clinfo")
	}
	if info.HasVAAPI {
		sources = append(sources, "vainfo")
	}
	if info.Vendor != GPUNone {
		sources = append(sources, "sysfs(/sys/class/drm)")
	}
	if len(info.Visibility.DeviceNodes) > 0 {
		sources = append(sources, "device-nodes(stat)")
	}
	if info.Model != "" && !info.HasROCm && !info.HasCUDA {
		sources = append(sources, "lspci")
	}
	if len(sources) == 0 {
		sources = append(sources, "nenhuma fonte observada (GPU não detectada)")
	}
	p.Source = strings.Join(sources, ",")

	p.Evidence = gpuEvidence(s, info)

	// Confiança pela cadeia epistemológica: só "high" com device nodes visíveis
	// E runtime confirmado por ferramenta. Reportado (vendor ou model) sem
	// verificação no ambiente → medium. Nada reportado → low.
	switch {
	case info.Vendor != GPUNone && info.Visibility.Visible && (info.HasROCm || info.HasCUDA):
		p.Confidence = "high"
	case info.Vendor != GPUNone || info.Model != "":
		p.Confidence = "medium"
	default:
		p.Confidence = "low"
	}
}

// gpuEvidence captura o dado bruto mais relevante (truncado em 200 runas) para
// a dimensão PROVENANCE. Read-only, best-effort — "" se nada disponível.
func gpuEvidence(s gpuSources, info *GPUInfo) string {
	const maxLen = 200
	var out string
	switch {
	case info.HasCUDA:
		out = s.runCommand("nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader")
	case info.HasROCm || info.Driver == "rocm-smi":
		out = s.runCommand(rocmBin(s, "rocm-smi"), "--showproductname")
	default:
		out = lspciEvidence(s.runCommand("lspci", "-mm"))
	}
	if out == "" {
		return ""
	}
	if len(out) > maxLen {
		return out[:maxLen] + "..."
	}
	return out
}

// lspciEvidence devolve a linha VGA/3D do output de `lspci -mm` — a evidência
// mais relevante de que uma GPU está reportada pelo hardware/OS. "" se ausente.
func lspciEvidence(out string) string {
	for _, line := range strings.Split(out, "\n") {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "vga") || strings.Contains(lower, "3d") {
			return line
		}
	}
	return ""
}

// =============================================================================
// Cross-vendor capability probes
// =============================================================================

func probeVulkan(s gpuSources, info *GPUInfo) {
	if s.hasCommand("vulkaninfo") {
		if out := s.runCommand("vulkaninfo", "--summary"); out != "" {
			info.HasVulkan = true
			if v := vulkanVersionFromVulkaninfo(out); v != "" && info.VulkanVersion == "" {
				info.VulkanVersion = v
			}
		}
	}
	if !info.HasVulkan {
		for _, dir := range []string{"/usr/share/vulkan/icd.d", "/etc/vulkan/icd.d"} {
			if len(s.listDir(dir)) > 0 {
				info.HasVulkan = true
				break
			}
		}
	}
	if info.HasVulkan && info.Driver == "" {
		info.Driver = "vulkan"
	}
}

// probeVAAPI checks VA-API video acceleration via vainfo or the Mesa DRI
// driver directory (AMD radeonsi / Intel). Used by FFmpeg for hardware
// transcode/preview without burning CPU (§17/§28).
func probeVAAPI(s gpuSources, info *GPUInfo) {
	if s.hasCommand("vainfo") {
		if out := s.runCommand("vainfo"); strings.Contains(out, "Driver version") {
			info.HasVAAPI = true
			return
		}
	}
	// Fallback: Mesa DRI drivers presence (radeonsi/amdgpu or intel).
	for _, dir := range []string{"/usr/lib/x86_64-linux-gnu/dri", "/usr/lib/dri", "/usr/lib64/dri"} {
		for _, f := range s.listDir(dir) {
			fl := strings.ToLower(f)
			if strings.Contains(fl, "radeonsi") || strings.Contains(fl, "amdgpu") ||
				strings.Contains(fl, "i965") || strings.Contains(fl, "iris") {
				info.HasVAAPI = true
				return
			}
		}
	}
}

// probeOpenCL checks OpenCL via clinfo or a libOpenCL runtime + ICD vendor.
func probeOpenCL(s gpuSources, info *GPUInfo) {
	if s.hasCommand("clinfo") {
		if out := s.runCommand("clinfo", "-l"); out != "" && strings.Contains(out, "Platform") {
			info.HasOpenCL = true
			if v := openclVersionFromClinfo(s.runCommand("clinfo")); v != "" {
				info.OpenCLVersion = v
			}
			return
		}
	}
	for _, dir := range []string{"/etc/OpenCL/vendors", "/etc/OpenCL/vendors.d"} {
		if len(s.listDir(dir)) > 0 {
			info.HasOpenCL = true
			return
		}
	}
}

// probeLspci fills the model from the PCI device name as a last resort.
func probeLspci(s gpuSources, info *GPUInfo) {
	out := s.runCommand("lspci", "-mm")
	if out == "" {
		return
	}
	for _, line := range strings.Split(out, "\n") {
		lower := strings.ToLower(line)
		if !strings.Contains(lower, "vga") && !strings.Contains(lower, "3d") {
			continue
		}
		parts := strings.Split(line, `"`)
		for i := len(parts) - 1; i >= 0; i-- {
			candidate := strings.TrimSpace(parts[i])
			cl := strings.ToLower(candidate)
			if candidate == "" ||
				strings.Contains(cl, "vga") ||
				strings.Contains(cl, "compatible") ||
				strings.Contains(cl, "advanced micro") {
				continue
			}
			info.Model = candidate
			return
		}
	}
}

// =============================================================================
// Parsers (pure, unit-testable)
// =============================================================================

// parseRocminfo extracts the GPU marketing name and gfx ISA target from
// rocminfo output, skipping the CPU agent block.
func parseRocminfo(out string) (model, gfx string) {
	var lastName string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Name:"):
			lastName = strings.TrimSpace(strings.TrimPrefix(line, "Name:"))
			if gfx == "" && (strings.HasPrefix(lastName, "gfx") || strings.HasPrefix(lastName, "amdgcn")) {
				gfx = lastName
			}
		case strings.HasPrefix(line, "Marketing Name:"):
			m := strings.TrimSpace(strings.TrimPrefix(line, "Marketing Name:"))
			if m == "" || m == lastName {
				continue
			}
			ml := strings.ToLower(m)
			if strings.Contains(ml, "ryzen") || strings.Contains(ml, "processor") ||
				strings.Contains(ml, "core") || strings.Contains(ml, "cpu") {
				continue
			}
			if model == "" {
				model = m
			}
		}
	}
	return model, gfx
}

// vramBytesFromRocmSMI parses "VRAM Total Memory (B)" from rocm-smi output.
func vramBytesFromRocmSMI(out string) int64 {
	const key = "VRAM Total Memory (B):"
	for _, line := range strings.Split(out, "\n") {
		if idx := strings.Index(line, key); idx >= 0 {
			rest := strings.TrimSpace(line[idx+len(key):])
			if n, err := strconv.ParseInt(rest, 10, 64); err == nil {
				return n
			}
		}
	}
	return 0
}

// cusFromRocminfo extrai os Compute Units da seção da GPU (gfx1030), pulando
// a seção do CPU (Agent 1 vem primeiro no rocminfo). Retorna 0 se não achar.
func cusFromRocminfo(out string) int {
	return intValueAfterKey(gpuSection(out), "Compute Unit:")
}

// clockFromRocminfo extrai o Max Clock da seção da GPU (não o do CPU).
func clockFromRocminfo(out string) int {
	return intValueAfterKey(gpuSection(out), "Max Clock Freq. (MHz):")
}

// gpuSection devolve a parte do rocminfo a partir da seção "Device Type: GPU".
// O rocminfo lista o CPU (Agent 1) antes da GPU (Agent 2) — esta função isola
// a seção da GPU para os parsers não misturarem valores.
func gpuSection(out string) string {
	idx := -1
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Device Type:") && strings.Contains(line, "GPU") {
			idx = strings.Index(out, line)
			break
		}
	}
	if idx < 0 {
		return out
	}
	return out[idx:]
}

// intValueAfterKey procura "key <n>" (número na mesma linha após o separador)
// e devolve o primeiro inteiro após a chave.
func intValueAfterKey(out, key string) int {
	for _, line := range strings.Split(out, "\n") {
		if idx := strings.Index(line, key); idx >= 0 {
			rest := strings.TrimSpace(line[idx+len(key):])
			for _, field := range strings.Fields(rest) {
				if n, err := strconv.Atoi(field); err == nil {
					return n
				}
			}
		}
	}
	return 0
}

// gfxFromRocmSMI parses "GFX Version" from rocm-smi --showproductname output.
func gfxFromRocmSMI(out string) string {
	const key = "GFX Version:"
	for _, line := range strings.Split(out, "\n") {
		if idx := strings.Index(line, key); idx >= 0 {
			return strings.TrimSpace(line[idx+len(key):])
		}
	}
	return ""
}

// parseNvidiaVRAM converts "12288 MiB" (or "12 GiB") to a GB count.
func parseNvidiaVRAM(s string) int {
	fields := strings.Fields(strings.TrimSpace(s))
	if len(fields) < 2 {
		return 0
	}
	n, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	switch strings.ToLower(fields[1]) {
	case "mib":
		return int((n + 1024/2) / 1024)
	case "gib":
		return int(n + 0.5)
	default:
		return int(n)
	}
}

// nvidiaComputeCap turns "8.0" into "sm_80".
func nvidiaComputeCap(s string) string {
	return "sm_" + strings.ReplaceAll(strings.TrimSpace(s), ".", "")
}

// bytesToGB rounds a byte count up to the nearest GB.
func bytesToGB(b int64) int {
	return int((b + (1<<30)/2) / (1 << 30))
}

// rocmInstallPresent reports whether an /opt/rocm* install directory exists.
func rocmInstallPresent(s gpuSources) bool {
	for _, dir := range s.listDir("/opt") {
		if strings.HasPrefix(dir, "rocm") {
			return true
		}
	}
	return false
}

// detectROCmVersion reads the ROCm version from /opt/rocm/.info/version,
// falling back to the /opt/rocm-<version> directory name.
func detectROCmVersion(s gpuSources) string {
	if v := s.readFile("/opt/rocm/.info/version"); v != "" {
		return v
	}
	for _, dir := range s.listDir("/opt") {
		if strings.HasPrefix(dir, "rocm-") {
			return strings.TrimPrefix(dir, "rocm-")
		}
	}
	return ""
}

// vulkanVersionFromVulkaninfo extrai a versão da API Vulkan do output de
// `vulkaninfo --summary` — "Vulkan Instance Version: X" ou a linha
// "apiVersion  = X" do dispositivo. "" se ausente.
func vulkanVersionFromVulkaninfo(out string) string {
	for _, line := range strings.Split(out, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "Vulkan Instance Version:") {
			return strings.TrimSpace(strings.TrimPrefix(t, "Vulkan Instance Version:"))
		}
		if strings.HasPrefix(t, "apiVersion") {
			if idx := strings.Index(t, "="); idx >= 0 {
				return strings.TrimSpace(t[idx+1:])
			}
		}
	}
	return ""
}

// openclVersionFromClinfo extrai a versão do runtime OpenCL do output completo
// de `clinfo` (linha "Platform Version:  OpenCL 3.0 ..."). "" se ausente.
func openclVersionFromClinfo(out string) string {
	const key = "Platform Version:"
	for _, line := range strings.Split(out, "\n") {
		if idx := strings.Index(line, key); idx >= 0 {
			return strings.TrimSpace(line[idx+len(key):])
		}
	}
	return ""
}
