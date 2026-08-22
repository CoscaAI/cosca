package estimator

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// changeTypeBaseMinutes é a base heurística de tempo (em minutos) por tipo de
// alteração, derivada do histórico real do Cosca (sessões registradas no
// TRUST_REGISTRY e nos impact-reports).
var changeTypeBaseMinutes = map[string]float64{
	"feature":  8,
	"fix":      5,
	"security": 10,
	"refactor": 6,
	"docs":     2,
	"test":     4,
}

// EstimateTime estima o tempo de execução em minutos usando a heurística
// documentada: base por tipo de alteração + files×0.3min + tests×0.02min.
// O resultado é arredondado para cima, com mínimo de 1 minuto.
// Tipos desconhecidos usam a base neutra de 5 minutos.
func EstimateTime(files, tests int, changeType string) int {
	base, ok := changeTypeBaseMinutes[changeType]
	if !ok {
		base = 5
	}
	minutes := base + float64(files)*0.3 + float64(tests)*0.02
	rounded := int(math.Ceil(minutes))
	if rounded < 1 {
		return 1
	}
	return rounded
}

// criticalPackageNames são os pacotes críticos que tornam a alteração de risco
// ALTO: núcleo do kernel e superfície de segurança.
var criticalPackageNames = []string{
	"auth",
	"jail",
	"kernel",
	"security",
	"emergency",
	"serve",
}

// mediumPathSegments são segmentos de caminho que tornam a alteração de risco
// MÉDIO: a API pública e o chat interno.
var mediumPathSegments = []string{
	"/api/",
	"/internal/chat/",
}

// AssessRisk classifica o risco da alteração:
//
//	ALTO  — se houver migrações, OU tocar pacotes críticos (auth, jail, kernel,
//	        security, emergency, serve), OU files > 25.
//	MÉDIO — se files > 10, OU tests > 200, OU tocar api/ ou internal/chat/.
//	BAIXO — caso contrário.
func AssessRisk(files []string, tests, migrations int, changeType string) string {
	for _, f := range files {
		if touchesCriticalPackage(f) {
			return "alto"
		}
	}
	if migrations > 0 {
		return "alto"
	}
	if len(files) > 25 {
		return "alto"
	}
	if len(files) > 10 {
		return "médio"
	}
	if tests > 200 {
		return "médio"
	}
	for _, f := range files {
		if pathContainsAnySegment(f, mediumPathSegments) {
			return "médio"
		}
	}
	return "baixo"
}

// touchesCriticalPackage detecta se um arquivo pertence a um pacote crítico.
// Casa por diretório ("internal/auth/..."), por nome de arquivo ("jail.go",
// "emergency.go") ou por prefixo de arquivo ("security_policy.go",
// "kernel_util.go"). O prefixo exige "_" logo após o nome do pacote para não
// marcar falsamente "server.go" como "serve".
func touchesCriticalPackage(path string) bool {
	slashed := "/" + strings.Trim(filepath.ToSlash(path), "/")
	base := strings.ToLower(filepath.Base(path))
	for _, kw := range criticalPackageNames {
		if strings.Contains(slashed, "/"+kw+"/") {
			return true
		}
		if base == kw+".go" || strings.HasPrefix(base, kw+"_") {
			return true
		}
	}
	return false
}

// pathContainsAnySegment normaliza o caminho para slash e verifica se ele
// contém algum dos segmentos (ex: "/api/"). O prefixo "/" garante que um
// arquivo na raiz (ex: "api.go") não seja falsamente classificado.
func pathContainsAnySegment(path string, segments []string) bool {
	slashed := "/" + strings.Trim(filepath.ToSlash(path), "/")
	for _, seg := range segments {
		if strings.Contains(slashed, seg) {
			return true
		}
	}
	return false
}

// ComputeConfidence calcula a confiança percentual (0-100) a partir do risco
// da alteração e da confiança histórica do agente (0.00-1.00):
//
//	baixo → 90 + min(8,  agentConfidence×10)  → 90-98
//	médio → 80 + min(10, agentConfidence×10)  → 80-90
//	alto  → 65 + min(15, agentConfidence×15)  → 65-80
func ComputeConfidence(risk string, agentConfidence float64) int {
	switch risk {
	case "baixo":
		return 90 + int(math.Min(8, agentConfidence*10))
	case "médio":
		return 80 + int(math.Min(10, agentConfidence*10))
	case "alto":
		return 65 + int(math.Min(15, agentConfidence*15))
	default:
		return 80 + int(math.Min(10, agentConfidence*10))
	}
}

// CheckRollback verifica se o projeto é um repositório git (.git/) e se existe
// ao menos um commit (HEAD resolvível). Quando disponível, retorna o comando
// de rollback "disponível (git revert <hash>)"; caso contrário, "não disponível".
func CheckRollback(projectDir string) (bool, string) {
	gitDir := filepath.Join(projectDir, ".git")
	if fi, err := os.Stat(gitDir); err != nil || !fi.IsDir() {
		return false, "não disponível"
	}

	if hash, ok := headHashViaGit(projectDir); ok {
		return true, fmt.Sprintf("disponível (git revert %s)", hash)
	}
	if hash, ok := headHashViaRefs(gitDir); ok {
		return true, fmt.Sprintf("disponível (git revert %s)", hash)
	}
	return false, "não disponível"
}

// headHashViaGit resolve o hash do HEAD via "git rev-parse HEAD". Retorna
// false se o binário git não estiver disponível, não for um repo ou não houver
// commits.
func headHashViaGit(projectDir string) (string, bool) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = projectDir
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	hash := strings.TrimSpace(string(out))
	if looksLikeGitHash(hash) {
		return hash, true
	}
	return "", false
}

// headHashViaRefs é o fallback sem depender do binário git: lê .git/HEAD e
// resolve a ref (diretamente em .git/refs/... ou via packed-refs). Também
// suporta HEAD destacado (hash cru).
func headHashViaRefs(gitDir string) (string, bool) {
	headData, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return "", false
	}
	head := strings.TrimSpace(string(headData))
	if !strings.HasPrefix(head, "ref: ") {
		if looksLikeGitHash(head) {
			return head, true
		}
		return "", false
	}

	ref := strings.TrimSpace(strings.TrimPrefix(head, "ref: "))
	if refData, err := os.ReadFile(filepath.Join(gitDir, filepath.FromSlash(ref))); err == nil {
		hash := strings.TrimSpace(string(refData))
		if looksLikeGitHash(hash) {
			return hash, true
		}
	}
	if data, err := os.ReadFile(filepath.Join(gitDir, "packed-refs")); err == nil {
		for _, l := range strings.Split(string(data), "\n") {
			parts := strings.Fields(l)
			if len(parts) == 2 && parts[1] == ref && looksLikeGitHash(parts[0]) {
				return parts[0], true
			}
		}
	}
	return "", false
}

// looksLikeGitHash aceita hashes curtos (7+) e completos (40) em hexadecimal.
func looksLikeGitHash(s string) bool {
	if len(s) < 7 || len(s) > 40 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}
