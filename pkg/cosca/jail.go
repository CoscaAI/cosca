/*
Package cosca provides the auto-jail mechanism.

O proprio binario cosca e a jaula. Nao existe script externo.

Fluxo:
 1. main() detecta que COSCA_JAILED nao esta setado
 2. Le o proprio binario (/proc/self/exe)
 3. Cria copia executavel em RAM (memfd_create ou /tmp)
 4. Reexecuta via Bubblewrap com COSCA_JAILED=1
 5. Dentro da jaula: CLI roda normalmente
 6. Quando termina: copia executavel e apagada da RAM

FAIL-CLOSED (seguranca — correcao apos red team):

	A jaula NUNCA falha em silencio. Se a jaula nao puder ser estabelecida
	(bwrap ausente, /proc/self/exe ilegivel, falha ao criar a copia em RAM,
	ou o proprio bwrap falhar ao subir a sandbox), o processo:
	  - imprime um SECURITY WARNING alto em stderr + grava no security log;
	  - SEM o opt-in explicito COSCA_ALLOW_NO_ROOT=1, o chamador (ReexecInJail)
	    NEGOA a execucao (exit 1) — fail-closed de verdade. Nada de agente/
	    workload rodar sem sandbox apenas "com um aviso".
	  - COM COSCA_ALLOW_NO_ROOT=1 (opt-in explícito do operador, NUNCA default)
	    o processo segue sem sandbox, mas ainda assim emite o WEARNING — um
	    opt-in nunca vira um fallback silencioso.
	Rodar como ROOT e recusado SEMPRE: bwrap como root nao cria user
	namespace, portanto a jaula seria anulavel (root real dentro da bolha).

NOTA DE PORTABILIDADE: a implementacao bwrap (Linux-only) vive em
jail_linux.go. Em outras plataformas (ex: Windows) nao existe bubblewrap;
jail_windows.go implementa o mesmo contrato fail-closed: jailAvailable()
retorna indisponivel e ReexecInJail() cai no jailFallback (SECURITY WARNING
+ opt-in explicito COSCA_ALLOW_NO_ROOT=1).
*/
package cosca

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/config"
)

const (
	// EnvJailed indica se ja estamos dentro da jaula.
	EnvJailed = "COSCA_JAILED"

	// EnvJailPID armazena o PID do processo pai.
	// Util para o filho identificar o pai se precisar de comunicacao.
	EnvJailPID = "COSCA_JAIL_PID"

	// EnvAllowNoRoot e o opt-in explicito para rodar SEM sandbox quando a
	// jaula nao puder ser estabelecida.
	//   - sem a env:  o fallback imprime um SECURITY WARNING alto (stderr+log)
	//   - com "1":     o fallback roda silencioso (operador aceitou o risco)
	EnvAllowNoRoot = "COSCA_ALLOW_NO_ROOT"

	// EnvSecurityLog define o caminho do security log. Default:
	// ~/.config/cosca/security.log
	EnvSecurityLog = "COSCA_SECURITY_LOG"

	// EnvForceJail sobrescreve a deteccao automatica de container e FORCA
	// a tentativa de jail (para testes e ambientes que suportam bwrap
	// dentro de container com --privileged). Use APENAS se souber o que
	// esta fazendo.
	EnvForceJail = "COSCA_FORCE_JAIL"

	// ApparmorUsernsSysctlPath e o caminho do sysctl do kernel que controla a
	// restricao de user namespaces nao privilegiados (AppArmor 4.x / Ubuntu
	// 24.04). Quando =1, o bwrap falha com "setting up uid map: Permission
	// denied" mesmo sem CAP_SYS_ADMIN.
	ApparmorUsernsSysctlPath = "/proc/sys/kernel/apparmor_restrict_unprivileged_userns"

	// SecurityWarningNoJail e o alerta emitido quando a jaula nao pode ser
	// estabelecida e o processo roda sem sandbox. Texto honesto: SEM opt-in o
	// processo e NEGADO (exit 1) — o warning nunca significa "continua rodando".
	SecurityWarningNoJail = "SECURITY WARNING: jail unavailable, running WITHOUT sandbox. Set COSCA_ALLOW_NO_ROOT=1 to accept this risk (OPT-IN); without it the process is DENIED (exit 1). DO NOT run untrusted agents in this mode."

	// JailSecretsFile e o caminho FIXO dentro da jaula onde os segredos
	// COSCA_* (JWT/metrics etc.) sao entregues. O workspace e o "/" da jaula,
	// entao o arquivo estagiado em <workspace>/.cosca/jail-secrets.env antes
	// do launch aparece em /.cosca/jail-secrets.env dentro da bolha e e
	// removido pelo pai assim que o bwrap termina. Segredos NUNCA cruzam a
	// fronteira via --setenv (visiveis no argv por ps/journald).
	JailSecretsFile = "/.cosca/jail-secrets.env"

	// jailBootInfoFD e o fd do canal de prova de boot DENTRO do bwrap
	// (--info-fd): ExtraFiles[1] → fd 4 do filho. O bwrap escreve a info da
	// sandbox nesse fd assim que a montagem termina, ANTES de executar o
	// processo interno. O pai lê esse canal para distinguir "a jaula subiu e
	// o comando interno falhou" de "a jaula nem montou" (BUGFIX L218 — ver
	// jailBooted e ReexecInJail).
	//
	// VERDADE CRÍTICA (BUGFIX L218, 2ª rodada): o fd não pode ser 3 —
	// ExtraFiles[0] (o próprio binário memfd) ocupa o fd 3 do filho e o
	// caminho /proc/self/fd/3 precisa apontar para o BINÁRIO. Se o canal
	// ocupasse o fd 3, o bwrap tentaria exec do pipe e TODO comando preso
	// morreria com exit 1 sem diagnóstico.
	jailBootInfoFD = "4"

	// jailBinaryPathInChild e o caminho estável do binário DENTRO do bwrap:
	// ExtraFiles[0] → fd 3 do filho, sempre. O fd do memfd no PAI pode ser
	// qualquer número (Go aloca 3, 4, 5... conforme fds abertos antes) — o
	// caminho /proc/self/fd/N do pai NÃO vale no filho quando ExtraFiles
	// realoca os fds. Este literal é a única referência confiável.
	jailBinaryPathInChild = "/proc/self/fd/3"
)

// LoadJailSecrets carrega os segredos COSCA_* do arquivo montado dentro da
// jaula e aplica-os ao ambiente do processo via os.Setenv. Deve ser chamado
// no main() imediatamente apos entrar na jaula (COSCA_JAILED=1) e ANTES de
// qualquer leitura de config/segredo. No-op fora da jaula ou se o arquivo
// nao existir — segredos nao montados nao sao inventados.
func LoadJailSecrets() {
	if os.Getenv(EnvJailed) != "1" {
		return
	}
	data, err := os.ReadFile(JailSecretsFile)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if key, value, ok := strings.Cut(line, "="); ok {
			os.Setenv(key, value)
		}
	}
}

// isJailSecretEnv identifica variaveis COSCA_* que carregam segredo e nao
// podem ser propagadas via --setenv (o valor apareceria no argv do bwrap,
// visivel por ps aux / systemctl status / journald). Heuristica: nome contem
// SECRET, KEY, TOKEN, PASSWORD ou PASSPHRASE.
func isJailSecretEnv(key string) bool {
	upper := strings.ToUpper(key)
	for _, marker := range []string{"SECRET", "KEY", "TOKEN", "PASSWORD", "PASSPHRASE"} {
		if strings.Contains(upper, marker) {
			return true
		}
	}
	return false
}

// writeJailSecretsFile estagia os segredos COSCA_* em
// <workspace>/.cosca/jail-secrets.env (0600) — dentro do workspace, que e o
// root da jaula, entao o conteudo fica visivel para o processo preso sem
// nenhum bind adicional e sem exposição no argv. Retorna o caminho ("" se nao
// ha segredos) para o chamador remover no cleanup.
func writeJailSecretsFile(workspaceDir string) (string, error) {
	var secretItems []string
	for _, item := range jailPropagatedEnv() {
		key, _, ok := strings.Cut(item, "=")
		if ok && isJailSecretEnv(key) {
			secretItems = append(secretItems, item)
		}
	}
	if len(secretItems) == 0 {
		return "", nil
	}
	sort.Strings(secretItems)
	dir := filepath.Join(workspaceDir, ".cosca")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create .cosca dir for jail secrets: %w", err)
	}
	path := filepath.Join(dir, "jail-secrets.env")
	content := strings.Join(secretItems, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write jail secrets file: %w", err)
	}
	return path, nil
}

// InsideJail retorna true se o processo ja esta executando dentro da jaula.
func InsideJail() bool {
	return os.Getenv(EnvJailed) == "1"
}

// IsRunningInContainer detecta se o processo esta rodando dentro de um
// container (Docker, containerd, Podman, Kubernetes, LXC/LXD).
//
// Usa 3 checks (qualquer um positivo = container):
//  1. Arquivo /.dockerenv existe — Docker (classico)
//  2. /proc/1/cgroup contem "docker", "containerd", "kubepods" ou "/lxc/"
//     — container runtime (funciona com cgroups v1 e v2)
//  3. Variavel de ambiente "container" esta setada — Podman, systemd-nspawn
//
// Se COSCA_FORCE_JAIL=1, retorna false — o operador quer forcar o jail
// mesmo em container (requer --privileged + bwrap instalado).
//
// Deteccao e best-effort: um falso negativo (nao detecta container) so
// faz o jail TENTAR rodar — o bwrap vai falhar e cair no fallback de
// qualquer forma. Um falso positivo (detecta container onde nao tem)
// pula o jail desnecessariamente — o container (inexistente) nao isola
// nada, mas o COSCA_ALLOW_NO_ROOT serve de rede de seguranca.
func IsRunningInContainer() bool {
	// Operador pode forcar jail mesmo em container.
	if os.Getenv(EnvForceJail) == "1" {
		return false
	}

	// Check 1: /.dockerenv (Docker classico).
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// Check 2: /proc/1/cgroup (cgroups v1 e v2).
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		lower := strings.ToLower(string(data))
		for _, marker := range []string{"docker", "containerd", "kubepods", "/lxc/"} {
			if strings.Contains(lower, marker) {
				return true
			}
		}
	}

	// Check 3: env var "container" (Podman, systemd-nspawn).
	if os.Getenv("container") != "" {
		return true
	}

	return false
}

// jailFallback e a funcao que decide o que acontece quando a jaula nao pode
// ser estabelecida. NUNCA roda direto em silencio. Retorna TRUE apenas com o
// opt-in explicito; caso contrario, o chamador (ReexecInJail) NEGOA a execucao.
//   - default (sem COSCA_ALLOW_NO_ROOT): SECURITY WARNING alto em stderr +
//     gravado no security log e RETORNO FALSE → o processo nao segue (exit 1
//     no chamador). Fail-closed de verdade: agente/workload nao roda sem
//     sandbox apenas "com um aviso".
//   - COSCA_ALLOW_NO_ROOT=1: opt-in explicito — retorna TRUE (permite seguir
//     sem sandbox), mas SEMPRE emite o alerta. Um opt-in nunca vira um
//     fallback silencioso.
//
// IMPORTANTE (postura): este opt-in e de responsabilidade EXCLUSIVA do
// operador. Nenhum launcher/script instalado deve injeta-lo por default —
// faz-lo transformaria o escape-hatch em postura default e anularia o
// fail-closed. Ver deploy/*.ps1, cosca-service.ps1 e cosca-serve.bat.
func jailFallback(reason string) bool {
	recordSecurityAlert(reason)
	// An explicit development opt-in is the only permitted direct-execution
	// mode. It is still noisy: an opt-in must never become a silent fallback.
	printSecurityWarning(reason)
	return os.Getenv(EnvAllowNoRoot) == "1"
}

// printSecurityWarning imprime o alerta de seguranca em stderr.
func printSecurityWarning(reason string) {
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "  [COSCA] %s\n", SecurityWarningNoJail)
	fmt.Fprintf(os.Stderr, "  [COSCA] Reason: %s\n", reason)
	fmt.Fprintf(os.Stderr, "  [COSCA] Without COSCA_ALLOW_NO_ROOT=1 this run is DENIED (exit 1). Set it ONLY to explicitly accept the risk of running WITHOUT sandbox (opt-in); it never makes execution safe.\n\n")
}

// recordSecurityAlert grava o alerta no security log (best-effort, nunca
// falha o chamador). Local: $COSCA_SECURITY_LOG ou ~/.config/cosca/security.log.
func recordSecurityAlert(reason string) {
	path := os.Getenv(EnvSecurityLog)
	if path == "" {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, ".config", "cosca", "security.log")
		}
	}
	if path == "" {
		return
	}
	line := fmt.Sprintf("%s [SECURITY] %s reason=%q\n", time.Now().Format(time.RFC3339), SecurityWarningNoJail, reason)
	if f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600); err == nil {
		defer f.Close()
		f.WriteString(line)
	}
}

// jailBooted decide se a jaula foi estabelecida DE VERDADE: o bwrap escreve a
// info da sandbox no fd registrado com --info-fd assim que a montagem
// termina, ANTES de executar o processo interno (o próprio bwrap garante a
// ordem). Dados no canal → a jaula subiu e o processo interno rodou; EOF sem
// dados → o bwrap morreu antes de completar a montagem (falha genuína).
// Recebe um io.Reader para testabilidade (os/exec real usa o pipe os.Pipe).
func jailBooted(r io.Reader) bool {
	buf := make([]byte, 1)
	n, err := r.Read(buf)
	if err != nil {
		return false
	}
	return n > 0
}

// jailExitCodeFromRun extrai o exit code de um erro de cmd.Run(). A jaula
// subiu e o comando interno falhou: o código é o DO COMANDO (ExitError). Para
// erros não-ExitError (ex: processo morto por sinal, ExitCode() = -1) ou
// erros genéricos, devolve 1 — nunca 0, para o chamador nunca tratar uma
// falha como sucesso.
func jailExitCodeFromRun(err error) int {
	if exitErr, ok := err.(*exec.ExitError); ok {
		if code := exitErr.ExitCode(); code > 0 {
			return code
		}
	}
	return 1
}

// jailInjectInfoFD insere --info-fd FD imediatamente ANTES do caminho do
// binário na lista de args do bwrap (opções do bwrap precisam vir antes do
// comando; buildJailArgs anexa jailPath + args originais no fim). Procura a
// ÚLTIMA ocorrência de jailPath — o próprio caminho do binário, nunca um
// argumento do usuário que coincida por acaso.
func jailInjectInfoFD(args []string, jailPath, fd string) []string {
	idx := len(args)
	for i := len(args) - 1; i >= 0; i-- {
		if args[i] == jailPath {
			idx = i
			break
		}
	}
	out := make([]string, 0, len(args)+2)
	out = append(out, args[:idx]...)
	out = append(out, "--info-fd", fd)
	out = append(out, args[idx:]...)
	return out
}

func jailEnvironment() []string {
	allowed := jailAllowedEnv()
	var result []string
	for _, item := range os.Environ() {
		key, _, ok := strings.Cut(item, "=")
		if ok && allowed[key] {
			result = append(result, item)
		}
	}
	return result
}

// jailAllowedEnv devolve o conjunto de variáveis de ambiente do host que o
// processo pai entrega ao bwrap. Regra central: só configuração própria do
// Cosca (prefixo COSCA_*) + mínimo de localização/runtime. Credenciais de
// terceiros (OPENAI_API_KEY, ANTHROPIC_API_KEY...) nunca cruzam a fronteira
// da jaula — o pai carrega o que precisa, a jaula não herda chaves do host.
func jailAllowedEnv() map[string]bool {
	allowed := map[string]bool{
		"PATH": true, "HOME": true, "LANG": true, "LC_ALL": true, "TERM": true,
		"COLORTERM": true, "TERM_PROGRAM": true,
		"NO_COLOR": true, "CLICOLOR": true, "CLICOLOR_FORCE": true,
		"TMPDIR": true, "TZ": true, EnvJailed: true, EnvJailPID: true,
		// Toolchain Go: o pai fornece caches/proxy ao bwrap; a jaula recebe os
		// valores REMAPEADOS (caminhos da jaula) via --setenv em buildJailArgs.
		"GOMODCACHE": true, "GOCACHE": true, "GOPATH": true, "GOPROXY": true,
	}
	for _, item := range os.Environ() {
		key, _, ok := strings.Cut(item, "=")
		if ok && strings.HasPrefix(key, "COSCA_") {
			allowed[key] = true
		}
	}
	return allowed
}

// jailPropagatedEnv devolve os pares chave=valor do host que devem ser
// RESTAURADOS dentro da jaula via --setenv (o --clearenv varre tudo).
// Apenas variáveis COSCA_* — a configuração do próprio engine (ex:
// COSCA_JWT_SECRET, COSCA_METRICS_SECRET, COSCA_PROVIDER, COSCA_LOG_LEVEL).
// EnvJailed/EnvJailPID são fixados explicitamente pelo jail e não entram;
// PATH e HOME também não entram — o jail fixa os dois por design.
// As variáveis do toolchain Go (GOMODCACHE/GOCACHE/GOPATH/GOPROXY) também não
// entram aqui: os valores do host apontam para caminhos que NÃO existem dentro
// da jaula (HOME=/). Elas são remapeadas para os caminhos canônicos da jaula
// em jailGoToolchainEnv(), chamado por buildJailArgs.
func jailPropagatedEnv() []string {
	var result []string
	for _, item := range os.Environ() {
		key, _, ok := strings.Cut(item, "=")
		if !ok || !strings.HasPrefix(key, "COSCA_") || key == EnvJailed || key == EnvJailPID {
			continue
		}
		result = append(result, item)
	}
	return result
}

// validateJailWorkspace prevents a jail from turning the host filesystem root
// (including a symlink to it) into the writable workspace mount.
func validateJailWorkspace(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("cannot resolve workspace: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return fmt.Errorf("cannot resolve workspace: %w", err)
	}
	if resolved == string(filepath.Separator) {
		return fmt.Errorf("refusing filesystem root as workspace")
	}
	if filepath.Clean(resolved) != filepath.Clean(abs) {
		return fmt.Errorf("refusing symlinked workspace")
	}
	return nil
}

func sanitizeJailDiagnostic(s string) string {
	for _, item := range os.Environ() {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) == 2 && len(parts[1]) >= 4 {
			s = strings.ReplaceAll(s, parts[1], "[REDACTED]")
		}
	}
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' || (r >= 0x20 && r != 0x7f) {
			return r
		}
		return ' '
	}, s)
	s = strings.TrimSpace(s)
	if len(s) > 1024 {
		s = s[:1024] + "..."
	}
	return s
}

// removeEmptyUp remove um mountpoint vazio (arquivo ou diretório) e sobe
// removendo diretórios vazios até achar um que ainda tenha conteúdo.
func removeEmptyUp(p string) {
	for {
		info, err := os.Lstat(p)
		if err != nil {
			return
		}
		if info.IsDir() {
			if entries, err := os.ReadDir(p); err != nil || len(entries) > 0 {
				return
			}
		} else if info.Size() != 0 {
			return
		}
		if err := os.Remove(p); err != nil {
			return
		}
		parent := filepath.Dir(p)
		if parent == p {
			return
		}
		p = parent
	}
}

// createTempFile cria um arquivo temporario em /tmp/ (tmpfs = RAM) e devolve
// o *os.File REABERTO (o original precisa ser fechado para o cleanup poder
// remover o arquivo; o fd reaberto é o que atravessa o exec do bwrap).
func createTempFile(self []byte) (*os.File, func(), error) {
	tmpDir := os.TempDir()
	if tmpDir == "" {
		tmpDir = "/tmp"
	}

	f, err := os.CreateTemp(tmpDir, "cosca-jail-*")
	if err != nil {
		return nil, nil, fmt.Errorf("create temp: %w", err)
	}
	path := f.Name()

	if _, err := f.Write(self); err != nil {
		f.Close()
		os.Remove(path)
		return nil, nil, fmt.Errorf("temp write: %w", err)
	}

	if err := f.Chmod(0500); err != nil {
		f.Close()
		os.Remove(path)
		return nil, nil, fmt.Errorf("temp chmod: %w", err)
	}

	if err := f.Close(); err != nil {
		os.Remove(path)
		return nil, nil, fmt.Errorf("temp close: %w", err)
	}

	file, err := os.Open(path)
	if err != nil {
		os.Remove(path)
		return nil, nil, fmt.Errorf("reopen temp: %w", err)
	}
	cleanup := func() {
		file.Close()
		os.Remove(path)
	}

	return file, cleanup, nil
}

// loadConstraints reads the workspace constraints file (.cosca/constraints.yaml).
// Returns permissive defaults if the file doesn't exist. FAIL-CLOSED: if the
// file exists but is unreadable/corrupt, returns restrictive defaults
// (everything blocked, read-only) — the jail must CLOSE when the policy breaks,
// never open.
func loadConstraints(workspaceDir string) *config.Constraints {
	c, err := config.LoadConstraints(workspaceDir)
	if err != nil {
		recordSecurityAlert(fmt.Sprintf("constraints file unreadable/corrupt — FAIL-CLOSED, restrictive policy applied: %v", err))
		return config.RestrictiveConstraints()
	}
	return c
}
