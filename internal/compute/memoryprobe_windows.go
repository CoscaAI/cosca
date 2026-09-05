//go:build windows

package compute

// defaultRlimitAS não tem contraparte observável no Windows: não existe
// RLIMIT_AS (nem getrlimit) em Win32. Devolve (0, false) para o
// resolveEffective cair para o fallback visible — o restante de probeMemory
// já trata isso como best-effort (limitação registrada em Limitations).
func defaultRlimitAS() (uint64, bool) {
	return 0, false
}
