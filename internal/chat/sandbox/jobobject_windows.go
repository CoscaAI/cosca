//go:build windows

package sandbox

// jobobject_windows.go — JOB OBJECT: DETECÇÃO do sandbox nativo Windows.
//
// O executor compartilhado (processutil.Run) já cria um Job Object com
// JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE para terminar a árvore inteira de um
// comando (ver internal/processutil/processutil_tree_windows.go). Este arquivo
// é o ponto de DETECÇÃO do backend nativo: jobObjectAvailable() decide se o
// sandbox do Windows está operacional — usado por nativeSandboxAvailable()
// (gate_windows.go) e IsAvailable() (gate.go).
//
// PRÓXIMO INCREMENTO (ADR-034 §3.2, fases seguintes):
//   - Limites de recurso no job: JOB_OBJECT_LIMIT_PROCESS_MEMORY (análogo do
//     RLIMIT_AS, default 6 GiB via COSCA_SANDBOX_MEMORY_MB) e
//     JOB_OBJECT_LIMIT_ACTIVE_PROCESS (anti fork-bomb, default 256 via
//     COSCA_SANDBOX_MAX_PROCESSES). Exigem um hook no processutil para aplicar
//     os limites no job que ele cria.
//   - AppContainer (isolamento FS/rede por SID): não exposto no x/sys v0.47.0
//     (CreateAppContainerProfile/SECURITY_CAPABILITIES ausentes) — exige
//     syscall manual ou upgrade da dependência.

import (
	"sync"

	"golang.org/x/sys/windows"
)

// jobObjectAvailableCache evita re-testar a API a cada chamada (uma vez por
// processo). É package-level para os testes conseguirem forçar indisponível.
var (
	jobObjectAvailableMu     sync.Mutex
	jobObjectAvailableCache *bool
)

// jobObjectAvailable reporta se o SO consegue criar um Job Object — o backend
// nativo do sandbox Windows. Best-effort e cacheado: cria e fecha um job de
// teste na primeira chamada.
func jobObjectAvailable() bool {
	jobObjectAvailableMu.Lock()
	defer jobObjectAvailableMu.Unlock()
	if jobObjectAvailableCache != nil {
		return *jobObjectAvailableCache
	}
	ok := false
	if job, err := windows.CreateJobObject(nil, nil); err == nil {
		ok = true
		_ = windows.CloseHandle(job)
	}
	jobObjectAvailableCache = &ok
	return ok
}
