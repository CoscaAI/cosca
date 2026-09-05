package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/compute"
)

// NewHardwareCommand creates the `cosca hardware` command tree.
// It probes machine capacity: CPU, RAM, GPU and CPU/memory topology (NUMA).
func NewHardwareCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hardware [probe|cpu|memory|topology|environment|gpu|storage]",
		Short: "Probe machine hardware capacity",
		Long: `Probe machine hardware capacity — CPU, RAM and GPU compute capability.

The GPU probe is read-only (no sudo, no driver install) and reports not just
the GPU brand but its capacity: driver, API surface (ROCm/Vulkan/CUDA) and
compute target (gfx1031, sm_80, ...).

The CPU probe is read-only (user space): vendor, family/model/stepping,
physical vs logical cores, packages, frequencies, caches and SIMD/ISA features.

The Memory probe is read-only (user space): the physical/visible/available/
effective chain, page size and swap.

The Topology probe is read-only (user space): NUMA nodes (CPUs, memory,
distances), CPU→node mapping, packages/sockets and per-CPU topology (core id,
thread siblings). Nenhuma afinidade/política NUMA é modificada — apenas observada.

The Environment probe is read-only (user space): the execution context —
bare-metal, virtualized, container or inside the Cosca jail (bwrap). Reports
whether /sys is mounted, the cgroup version/controllers and the process
effective cpuset. Nunca tenta escapar da jaula nem inferir o host.

The GPU environment probe (Fase 5 do Hardware & Performance Brain) models the
GPU as a COMPUTATIONAL ENVIRONMENT across 6 dimensions: identity, resources,
runtime (APIs), visibility (device nodes visíveis no ambiente atual — somente
stat, nunca abrir/escrever), capabilities (reported → actually_visible →
experimentally_verified) and provenance (source/timestamp/evidence/confidence).

The Storage probe is read-only (user space): mounted filesystems (from
/proc/mounts), per-filesystem capacity (statfs — total/used/available),
physical media classification (NVMe/SSD/HDD via /sys/block rotational) and
visible block device nodes (/dev/sd*, /dev/nvme*, /dev/mmc* — existence only).
The capability chain (reported → visible → usable → measured) follows the
Professor's law: measured stays false — storage benchmark is Fase 9.

Subcommands:
  probe       - Print the full hardware table (CPU / RAM / GPU / topology)
  cpu         - Print the detailed CPU discovery (features, caches, frequencies)
  memory      - Print the detailed memory discovery (physical/visible/available/effective)
  topology    - Print the detailed CPU/memory topology (NUMA nodes, distances, packages)
  environment - Print the execution environment (bare-metal/VM/container/jail)
  gpu         - Print the GPU computational environment (6 dimensions: identity/
                resources/runtime/visibility/capabilities/provenance)
  storage     - Print the storage discovery (mounts, capacity, NVMe/SSD/HDD)`,
		Example: `  cosca hardware probe       Print CPU / RAM / GPU capacity
  cosca hardware cpu        Print detailed CPU discovery
  cosca hardware memory     Print detailed memory discovery
  cosca hardware topology   Print detailed NUMA/topology discovery
  cosca hardware environment Print execution environment
  cosca hardware gpu        Print GPU as a computational environment (6 dimensions)
  cosca hardware storage    Print storage discovery (mounts / capacity / media type)
  cosca hardware probe --json  Machine-readable output`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && args[0] != "probe" && args[0] != "cpu" && args[0] != "memory" && args[0] != "topology" && args[0] != "environment" && args[0] != "gpu" && args[0] != "storage" {
				return fmt.Errorf("unknown subcommand: %s (valid: probe, cpu, memory, topology, environment, gpu, storage)", args[0])
			}
			return runHardwareProbe(cmd)
		},
	}
	cmd.AddCommand(NewHardwareProbeCommand(), NewHardwareCPUCommand(), NewHardwareMemoryCommand(), NewHardwareTopologyCommand(), NewHardwareEnvironmentCommand(), NewHardwareGPUCommand(), NewHardwareStorageCommand())
	return cmd
}

// NewHardwareProbeCommand creates the `cosca hardware probe` subcommand.
func NewHardwareProbeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "probe",
		Short: "Print CPU, RAM and GPU capacity",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runHardwareProbe(cmd)
		},
	}
}

// NewHardwareCPUCommand creates the `cosca hardware cpu` subcommand — a
// read-only user-space CPU discovery (Fase 1 do Hardware & Performance Brain).
func NewHardwareCPUCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "cpu",
		Short: "Print detailed CPU discovery (vendor, cores, features, caches)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runHardwareCPU(cmd)
		},
	}
}

func runHardwareCPU(cmd *cobra.Command) error {
	formatter := GetFormatter(cmd)
	info := compute.ProbeCPU()

	if IsJSONOutput(cmd) {
		return printJSON(cmd, info)
	}

	formatter.Header("CPU Discovery")
	formatter.KeyValue("Vendor", string(info.Vendor))
	if info.ModelName != "" {
		formatter.KeyValue("Model", info.ModelName)
	}
	if info.Family != "" {
		formatter.KeyValue("Family", info.Family)
	}
	if info.Model != "" {
		formatter.KeyValue("Model ID", info.Model)
	}
	if info.Stepping != "" {
		formatter.KeyValue("Stepping", info.Stepping)
	}
	formatter.KeyValue("Architecture", info.Arch)
	formatter.KeyValue("Logical CPUs", strconv.Itoa(info.LogicalCPUs))
	if info.PhysicalCores > 0 {
		formatter.KeyValue("Physical Cores", strconv.Itoa(info.PhysicalCores))
	}
	if info.Packages > 0 {
		formatter.KeyValue("Packages", strconv.Itoa(info.Packages))
	}
	formatter.KeyValue("Detected", yesNo(info.Detected))
	formatter.KeyValue("Available", yesNo(info.Available))
	formatter.KeyValue("Usable", yesNo(info.Usable))

	if f := info.Frequencies; f.BaseMHz > 0 || f.MaxMHz > 0 || f.CurrentMHz > 0 {
		formatter.Header("Frequencies (MHz)")
		if f.BaseMHz > 0 {
			formatter.KeyValue("Base", strconv.FormatInt(f.BaseMHz, 10))
		}
		if f.MaxMHz > 0 {
			formatter.KeyValue("Max", strconv.FormatInt(f.MaxMHz, 10))
		}
		if f.CurrentMHz > 0 {
			formatter.KeyValue("Current", strconv.FormatInt(f.CurrentMHz, 10))
		}
	}

	if len(info.Caches) > 0 {
		formatter.Header("Caches")
		rows := make([][]string, 0, len(info.Caches))
		for _, c := range info.Caches {
			rows = append(rows, []string{
				fmt.Sprintf("L%d", c.Level),
				fmt.Sprintf("%d KB", c.SizeKB),
				c.Type,
			})
		}
		formatter.Table([]string{"Level", "Size", "Type"}, rows)
	}

	formatter.Header("Features (ISA)")
	var feats []string
	for f := range info.Features {
		feats = append(feats, f)
	}
	sort.Strings(feats)
	if len(feats) == 0 {
		formatter.KeyValue("Flags", "(não determináveis em user space)")
	} else {
		formatter.KeyValue("Flags", strings.Join(feats, " "))
	}

	if len(info.Limitations) > 0 {
		formatter.Header("Limitations")
		for _, l := range info.Limitations {
			formatter.Bullet(l)
		}
	}
	return nil
}

// NewHardwareMemoryCommand creates the `cosca hardware memory` subcommand — a
// read-only user-space memory discovery (Fase 2 do Hardware & Performance Brain).
func NewHardwareMemoryCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "memory",
		Short: "Print detailed memory discovery (physical/visible/available/effective, swap)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runHardwareMemory(cmd)
		},
	}
}

func runHardwareMemory(cmd *cobra.Command) error {
	formatter := GetFormatter(cmd)
	info := compute.ProbeMemory()

	if IsJSONOutput(cmd) {
		return printJSON(cmd, info)
	}

	formatter.Header("Memory Discovery")
	formatter.KeyValue("Physical", formatBytes(info.PhysicalBytes))
	formatter.KeyValue("Visible", formatBytes(info.VisibleBytes))
	formatter.KeyValue("Available", formatBytes(info.AvailableBytes))
	formatter.KeyValue("Effective", formatBytes(info.EffectiveBytes))
	formatter.KeyValue("Page Size", fmt.Sprintf("%d B", info.PageSize))
	formatter.KeyValue("Swap Total", formatBytes(info.SwapTotalBytes))
	formatter.KeyValue("Swap Available", formatBytes(info.SwapAvailableBytes))
	formatter.KeyValue("Detected", yesNo(info.Detected))
	formatter.KeyValue("Available", yesNo(info.Available))
	formatter.KeyValue("Usable", yesNo(info.Usable))

	if len(info.Limitations) > 0 {
		formatter.Header("Limitations")
		for _, l := range info.Limitations {
			formatter.Bullet(l)
		}
	}
	return nil
}

// NewHardwareTopologyCommand creates the `cosca hardware topology` subcommand —
// a read-only user-space CPU/memory topology and NUMA discovery (Fase 3 do
// Hardware & Performance Brain). Nunca modifica afinidade/NUMA — apenas observa.
func NewHardwareTopologyCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "topology",
		Short: "Print detailed CPU/memory topology (NUMA nodes, distances, packages)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runHardwareTopology(cmd)
		},
	}
}

func runHardwareTopology(cmd *cobra.Command) error {
	formatter := GetFormatter(cmd)
	info := compute.ProbeTopology()

	if IsJSONOutput(cmd) {
		return printJSON(cmd, info)
	}

	formatter.Header("Topology Discovery")
	formatter.KeyValue("NUMA Nodes", strconv.Itoa(len(info.Nodes)))
	formatter.KeyValue("Packages", strconv.Itoa(info.Packages))
	if len(info.CoresPerPackage) > 0 {
		var parts []string
		for _, pkg := range sortedIntKeys(info.CoresPerPackage) {
			parts = append(parts, fmt.Sprintf("pkg %d: %d cores", pkg, info.CoresPerPackage[pkg]))
		}
		formatter.KeyValue("Cores per Package", strings.Join(parts, ", "))
	}
	if info.Synthetic {
		formatter.KeyValue("Synthetic", "yes (sysfs indisponível — topologia assumida)")
	}
	formatter.KeyValue("Detected", yesNo(info.Detected))
	formatter.KeyValue("Available", yesNo(info.Available))
	formatter.KeyValue("Usable", yesNo(info.Usable))

	for _, n := range info.Nodes {
		formatter.Header(fmt.Sprintf("NUMA Node %d", n.ID))
		if len(n.CPUs) > 0 {
			formatter.KeyValue("CPUs", formatCPUList(n.CPUs))
		}
		if n.MemTotal > 0 {
			formatter.KeyValue("Mem Total", formatBytes(n.MemTotal))
		}
		if n.MemFree > 0 {
			formatter.KeyValue("Mem Free", formatBytes(n.MemFree))
		}
		if len(n.Distance) > 0 {
			var dparts []string
			for _, to := range sortedIntKeys(n.Distance) {
				dparts = append(dparts, fmt.Sprintf("node%d=%d", to, n.Distance[to]))
			}
			formatter.KeyValue("Distances", strings.Join(dparts, " "))
		}
	}

	if len(info.CPUTopology) > 0 {
		formatter.Header("Per-CPU Topology")
		rows := make([][]string, 0, len(info.CPUTopology))
		for _, t := range info.CPUTopology {
			node := info.CPUToNode[t.ID]
			rows = append(rows, []string{
				strconv.Itoa(t.ID),
				strconv.Itoa(node),
				strconv.Itoa(t.PackageID),
				strconv.Itoa(t.CoreID),
				formatCPUList(t.ThreadSiblings),
			})
		}
		formatter.Table([]string{"CPU", "Node", "Package", "Core", "Threads"}, rows)
	}

	if len(info.Limitations) > 0 {
		formatter.Header("Limitations")
		for _, l := range info.Limitations {
			formatter.Bullet(l)
		}
	}
	return nil
}

// NewHardwareEnvironmentCommand creates the `cosca hardware environment`
// subcommand — the execution context (bare-metal/VM/container/jail).
func NewHardwareEnvironmentCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "environment",
		Short: "Print the execution environment (bare-metal/VM/container/jail)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runHardwareEnvironment(cmd)
		},
	}
}

func runHardwareEnvironment(cmd *cobra.Command) error {
	formatter := GetFormatter(cmd)
	info := compute.ProbeEnvironment()

	if IsJSONOutput(cmd) {
		return printJSON(cmd, info)
	}

	formatter.Header("Environment Discovery")
	formatter.KeyValue("Type", string(info.Type))
	if len(info.Types) > 0 {
		var parts []string
		for _, t := range info.Types {
			parts = append(parts, string(t))
		}
		formatter.KeyValue("Detected As", strings.Join(parts, ", "))
	}
	formatter.KeyValue("In Jail", yesNo(info.InJail))
	formatter.KeyValue("In Container", yesNo(info.InContainer))
	formatter.KeyValue("In VM", yesNo(info.InVM))
	formatter.KeyValue("Sysfs Mounted", yesNo(info.SysfsMounted))
	if info.CgroupVersion != "" {
		formatter.KeyValue("Cgroup Version", info.CgroupVersion)
	}
	if info.CgroupControllers != "" {
		formatter.KeyValue("Cgroup Controllers", info.CgroupControllers)
	}
	if info.EffectiveCPUs != "" {
		formatter.KeyValue("Effective CPUs (cpuset)", info.EffectiveCPUs)
	}
	formatter.KeyValue("GOOS", info.GOOS)
	formatter.KeyValue("GOARCH", info.GOARCH)
	formatter.KeyValue("Detected", yesNo(info.Detected))
	formatter.KeyValue("Available", yesNo(info.Available))
	formatter.KeyValue("Usable", yesNo(info.Usable))

	if len(info.Limitations) > 0 {
		formatter.Header("Limitations")
		for _, l := range info.Limitations {
			formatter.Bullet(l)
		}
	}
	return nil
}

// NewHardwareGPUCommand creates the `cosca hardware gpu` subcommand — the GPU
// as a COMPUTATIONAL ENVIRONMENT (Fase 5 do Hardware & Performance Brain, L318):
// identity, resources, runtime, visibility, capabilities and provenance.
func NewHardwareGPUCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "gpu",
		Short: "Print the GPU computational environment (6 dimensions)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runHardwareGPU(cmd)
		},
	}
}

func runHardwareGPU(cmd *cobra.Command) error {
	formatter := GetFormatter(cmd)
	info := compute.ProbeGPU()

	if IsJSONOutput(cmd) {
		return printJSON(cmd, info)
	}

	formatter.Header("GPU Computational Environment (Fase 5)")

	// 1. IDENTITY (vendor/device/architecture/driver).
	formatter.KeyValue("Vendor", string(info.Vendor))
	if info.Model != "" {
		formatter.KeyValue("Model", info.Model)
	}
	if info.Compute != "" {
		formatter.KeyValue("Compute Target", info.Compute)
	}
	if info.Driver != "" {
		formatter.KeyValue("Driver", info.Driver)
	}

	// 2. RESOURCES (VRAM / compute units / clocks / bandwidth).
	formatter.Header("Resources")
	if info.VRAMGB > 0 {
		formatter.KeyValue("VRAM", fmt.Sprintf("%d GB", info.VRAMGB))
	}
	if info.ComputeUnits > 0 {
		formatter.KeyValue("Compute Units", strconv.Itoa(info.ComputeUnits))
	}
	if info.MaxClockMHz > 0 {
		formatter.KeyValue("Max Clock", fmt.Sprintf("%d MHz", info.MaxClockMHz))
	}
	if info.BandwidthGBs > 0 {
		formatter.KeyValue("Bandwidth", fmt.Sprintf("%d GB/s", info.BandwidthGBs))
	} else if info.Vendor != compute.GPUNone {
		formatter.KeyValue("Bandwidth", "(não observável em user space — microbenchmark é fase posterior)")
	}

	// 3. RUNTIME (APIs disponíveis + versões observáveis).
	formatter.Header("Runtime (APIs)")
	formatter.KeyValue("ROCm", yesNo(info.HasROCm))
	if info.ROCmVersion != "" {
		formatter.KeyValue("ROCm Version", info.ROCmVersion)
	}
	formatter.KeyValue("Vulkan", yesNo(info.HasVulkan))
	if info.VulkanVersion != "" {
		formatter.KeyValue("Vulkan Version", info.VulkanVersion)
	}
	formatter.KeyValue("OpenCL", yesNo(info.HasOpenCL))
	if info.OpenCLVersion != "" {
		formatter.KeyValue("OpenCL Version", info.OpenCLVersion)
	}
	formatter.KeyValue("CUDA", yesNo(info.HasCUDA))
	formatter.KeyValue("VAAPI", yesNo(info.HasVAAPI))

	// 4. VISIBILITY (visível no ambiente ATUAL — device nodes via stat).
	formatter.Header("Visibility (ambiente atual)")
	formatter.KeyValue("Visible", yesNo(info.Visibility.Visible))
	if info.Visibility.Environment != "" {
		formatter.KeyValue("Environment", info.Visibility.Environment)
	}
	if len(info.Visibility.DeviceNodes) > 0 {
		formatter.KeyValue("Device Nodes", strings.Join(info.Visibility.DeviceNodes, " "))
	}
	if len(info.Visibility.Refs) > 0 {
		for _, r := range info.Visibility.Refs {
			formatter.Bullet(r)
		}
	}

	// 5. CAPABILITIES (as 3 camadas do Professor).
	formatter.Header("Capabilities (3 camadas)")
	formatter.KeyValue("Reported", yesNo(info.Capabilities.Reported))
	formatter.KeyValue("Actually Visible", yesNo(info.Capabilities.ActuallyVisible))
	formatter.KeyValue("Experimentally Verified", yesNo(info.Capabilities.ExperimentallyVerified))
	if info.Capabilities.WhyNot != "" {
		formatter.Bullet(info.Capabilities.WhyNot)
	}

	// 6. PROVENANCE (fonte / timestamp / evidência / confiança).
	formatter.Header("Provenance")
	if info.Provenance.Source != "" {
		formatter.KeyValue("Source", info.Provenance.Source)
	}
	if !info.Provenance.Timestamp.IsZero() {
		formatter.KeyValue("Timestamp", info.Provenance.Timestamp.UTC().Format(time.RFC3339))
	}
	if info.Provenance.Evidence != "" {
		formatter.KeyValue("Evidence", truncate(info.Provenance.Evidence, 120))
	}
	if info.Provenance.Confidence != "" {
		formatter.KeyValue("Confidence", info.Provenance.Confidence)
	}

	if len(info.Limitations) > 0 {
		formatter.Header("Limitations")
		for _, l := range info.Limitations {
			formatter.Bullet(l)
		}
	}
	return nil
}

// NewHardwareStorageCommand creates the `cosca hardware storage` subcommand —
// a read-only user-space storage discovery (Fase 6 do Hardware & Performance
// Brain): mounted filesystems, per-filesystem capacity, NVMe/SSD/HDD media
// classification and visible block device nodes. Nunca monta/altera nada.
func NewHardwareStorageCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "storage",
		Short: "Print storage discovery (mounts, capacity, NVMe/SSD/HDD)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runHardwareStorage(cmd)
		},
	}
}

func runHardwareStorage(cmd *cobra.Command) error {
	formatter := GetFormatter(cmd)
	info := compute.ProbeStorage()

	if IsJSONOutput(cmd) {
		return printJSON(cmd, info)
	}

	formatter.Header("Storage Discovery (Fase 6)")
	formatter.KeyValue("Environment", string(info.Environment))
	formatter.KeyValue("Detected", yesNo(info.Detected))
	formatter.KeyValue("Available", yesNo(info.Available))
	formatter.KeyValue("Usable", yesNo(info.Usable))

	formatter.Header("Capabilities (cadeia do Professor, L319)")
	formatter.KeyValue("Reported", yesNo(info.Capabilities.Reported))
	formatter.KeyValue("Visible", yesNo(info.Capabilities.Visible))
	formatter.KeyValue("Usable", yesNo(info.Capabilities.Usable))
	formatter.KeyValue("Measured", yesNo(info.Capabilities.Measured))
	if info.Capabilities.WhyNot != "" {
		formatter.Bullet(info.Capabilities.WhyNot)
	}

	formatter.Header("Focos do Cosca")
	if m := info.KnowledgeDBMount; m != nil {
		formatter.KeyValue("Knowledge.db (o cofre)", mountSummary(m))
	} else {
		formatter.KeyValue("Knowledge.db (o cofre)", "(não resolvível neste ambiente)")
	}
	if m := info.WorkspaceMount; m != nil {
		formatter.KeyValue("Workspace", mountSummary(m))
	} else {
		formatter.KeyValue("Workspace", "(não resolvível)")
	}
	if m := info.TmpMount; m != nil {
		formatter.KeyValue("/tmp", mountSummary(m))
	} else {
		formatter.KeyValue("/tmp", "(não resolvível)")
	}

	if len(info.Mounts) > 0 {
		formatter.Header("Filesystems Montados")
		rows := make([][]string, 0, len(info.Mounts))
		for _, m := range info.Mounts {
			media := string(m.DeviceType)
			if m.Rotational != nil && media != "nvme" {
				if *m.Rotational {
					media += " (hdd)"
				} else {
					media += " (ssd)"
				}
			}
			if m.Rotational == nil && media == "unknown" {
				media = "-"
			}
			rows = append(rows, []string{
				m.Device,
				m.MountPoint,
				m.FSType,
				formatBytes(int64(m.TotalBytes)),
				formatBytes(int64(m.UsedBytes)),
				formatBytes(int64(m.AvailableBytes)),
				media,
			})
		}
		formatter.Table([]string{"Device", "Mount", "FS", "Total", "Used", "Available", "Type"}, rows)
	}

	if len(info.DeviceNodes) > 0 {
		formatter.Header("Device Nodes Visíveis (/dev)")
		formatter.KeyValue("Block Devices", strings.Join(info.DeviceNodes, " "))
	}

	formatter.Header("Provenance")
	if info.Provenance.Source != "" {
		formatter.KeyValue("Source", info.Provenance.Source)
	}
	if !info.Provenance.Timestamp.IsZero() {
		formatter.KeyValue("Timestamp", info.Provenance.Timestamp.UTC().Format(time.RFC3339))
	}
	if info.Provenance.Evidence != "" {
		formatter.KeyValue("Evidence", info.Provenance.Evidence)
	}
	if info.Provenance.Confidence != "" {
		formatter.KeyValue("Confidence", info.Provenance.Confidence)
	}

	if len(info.Limitations) > 0 {
		formatter.Header("Limitations")
		for _, l := range info.Limitations {
			formatter.Bullet(l)
		}
	}
	return nil
}

// mountSummary renderiza um MountInfo em uma linha curta para os focos do
// Cosca (ex: "/dev/nvme0n1p2 (ext4) total 512.3 GB livre 240.1 GB — nvme").
func mountSummary(m *compute.MountInfo) string {
	media := string(m.DeviceType)
	if m.Rotational != nil && media != "nvme" && media != "tmpfs" {
		if *m.Rotational {
			media = "hdd"
		} else {
			media = "ssd"
		}
	}
	var b strings.Builder
	b.WriteString(m.MountPoint)
	b.WriteString(" (")
	b.WriteString(m.FSType)
	b.WriteString(") total ")
	b.WriteString(formatBytes(int64(m.TotalBytes)))
	b.WriteString(" livre ")
	b.WriteString(formatBytes(int64(m.AvailableBytes)))
	if media != "" && media != "unknown" {
		b.WriteString(" — ")
		b.WriteString(media)
	}
	return b.String()
}

func runHardwareProbe(cmd *cobra.Command) error {
	formatter := GetFormatter(cmd)
	snap := compute.ProbeHardware()

	if IsJSONOutput(cmd) {
		return printJSON(cmd, snap)
	}

	formatter.Header("CPU")
	formatter.KeyValue("Cores", strconv.Itoa(snap.LogicalCores))
	if cpu := snap.CPU; cpu.Vendor != "" {
		formatter.KeyValue("Vendor", string(cpu.Vendor))
		if cpu.ModelName != "" {
			formatter.KeyValue("Model", cpu.ModelName)
		}
		if cpu.PhysicalCores > 0 {
			formatter.KeyValue("Physical Cores", strconv.Itoa(cpu.PhysicalCores))
		}
		if f := cpu.Frequencies; f.CurrentMHz > 0 {
			formatter.KeyValue("Clock", fmt.Sprintf("%d MHz", f.CurrentMHz))
		}
	}
	formatter.KeyValue("Load 1m", fmt.Sprintf("%.2f", snap.Load1))
	formatter.KeyValue("Load 5m", fmt.Sprintf("%.2f", snap.Load5))
	formatter.KeyValue("Load 15m", fmt.Sprintf("%.2f", snap.Load15))
	formatter.KeyValue("Usage", fmt.Sprintf("%.1f%%", snap.CPUUsage))

	formatter.Header("RAM")
	formatter.KeyValue("Total", fmt.Sprintf("%.1f GB", float64(snap.TotalRAM)/(1<<30)))
	formatter.KeyValue("Available", fmt.Sprintf("%.1f GB", float64(snap.AvailableRAM)/(1<<30)))
	formatter.KeyValue("Used", fmt.Sprintf("%.1f GB", float64(snap.UsedRAM)/(1<<30)))
	formatter.KeyValue("Usage", fmt.Sprintf("%.1f%%", snap.MemoryUsage))
	if snap.Memory.EffectiveBytes > 0 {
		formatter.KeyValue("Effective", formatBytes(snap.Memory.EffectiveBytes))
	}

	formatter.Header("GPU")
	gpu := snap.GPU
	formatter.KeyValue("Vendor", string(gpu.Vendor))
	if gpu.Model != "" {
		formatter.KeyValue("Model", gpu.Model)
	}
	if gpu.VRAMGB > 0 {
		formatter.KeyValue("VRAM", fmt.Sprintf("%d GB", gpu.VRAMGB))
	}
	if gpu.Driver != "" {
		formatter.KeyValue("Driver", gpu.Driver)
	}
	formatter.KeyValue("ROCm", yesNo(gpu.HasROCm))
	formatter.KeyValue("Vulkan", yesNo(gpu.HasVulkan))
	formatter.KeyValue("CUDA", yesNo(gpu.HasCUDA))
	if gpu.Compute != "" {
		formatter.KeyValue("Compute", gpu.Compute)
	}
	if gpu.ROCmVersion != "" {
		formatter.KeyValue("ROCm Version", gpu.ROCmVersion)
	}

	return nil
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// formatCPUList renders a CPU id slice as a compact comma list ("0,8").
func formatCPUList(cpus []int) string {
	parts := make([]string, 0, len(cpus))
	for _, c := range cpus {
		parts = append(parts, strconv.Itoa(c))
	}
	return strings.Join(parts, ",")
}

// sortedIntKeys returns the keys of an int→T map in ascending order (used to
// render NUMA distances and per-package core counts deterministically).
func sortedIntKeys[T any](m map[int]T) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}
