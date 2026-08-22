//go:build linux

/*
Implementação Linux do auto-jail via Bubblewrap. Todo o comportamento
bwrap-specific vive aqui; jail.go (portátil) contém o contrato fail-closed
comum e jail_windows.go a variante sem bwrap (fallback com opt-in explícito).
*/

package cosca

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/CoscaAI/cosca/internal/config"

	"golang.org/x/sys/unix"
)

// jailGeteuid e injetavel para testes (TestJail_RootUnsafe simula euid==0).
var jailGeteuid = os.Geteuid

// jailAvailable decide se a jaula PODE ser estabelecida no ambiente atual.
// E o ponto unico de decisao fail-closed dos pre-flight checks:
//   - bwrap no PATH?                       (vuln 1: fail-open se ausente)
//   - /proc/self/exe legivel?              (vuln 2: fail-open se ilegivel)
//
// Retorna (true, "") se tudo pronto; (false, motivo) caso contrario.
func jailAvailable() (bool, string) {
	_, err := validatedBwrapPath()
	if err != nil {
		return false, err.Error()
	}
	/* /proc/self/exe is checked below; bwrap validation is deliberately repeated
	   at the actual exec boundary to reduce PATH replacement races. */
	return jailSelfExecutableAvailable()
}

func validatedBwrapPath() (string, error) {
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		return "", fmt.Errorf("bubblewrap (bwrap) not found in PATH")
	}
	// Never execute a launcher selected from an ambiguous PATH entry. This also
	// makes diagnostics deterministic when PATH is modified by an agent.
	bwrap, err = filepath.Abs(bwrap)
	if err != nil || !filepath.IsAbs(bwrap) {
		return "", fmt.Errorf("bubblewrap path is not absolute")
	}
	bwrap, err = filepath.EvalSymlinks(bwrap)
	if err != nil {
		return "", fmt.Errorf("bubblewrap path cannot be resolved")
	}
	info, err := os.Stat(bwrap)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0111 == 0 || info.Mode()&0022 != 0 {
		return "", fmt.Errorf("bubblewrap path is not a trusted executable")
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || (st.Uid != uint32(os.Geteuid()) && st.Uid != 0) {
		return "", fmt.Errorf("bubblewrap owner is not trusted")
	}
	return bwrap, nil
}

func jailSelfExecutableAvailable() (bool, string) {
	self, err := os.Open("/proc/self/exe")
	if err != nil {
		return false, fmt.Sprintf("cannot read /proc/self/exe: %v", err)
	}
	self.Close()
	return true, ""
}

// jailRefuseRoot devolve um erro fatal se o processo estiver rodando como
// root (euid == 0). Root + bwrap NAO cria user namespace — o "root dentro
// da jaula" e root real no host, logo a jaula e anulavel. Nao existe opt-in
// para este caso: e sempre recusado.
func jailRefuseRoot(euid int) error {
	if euid != 0 {
		return nil
	}
	return fmt.Errorf("refusing to run as root: bwrap as root does not create a user namespace, so the sandbox would be annullable (real root inside the jail); run as a regular user (no sudo). sudoers-cosca-jail is DEPRECATED and must NOT be installed in production")
}

// jailRootDenied retorna a mensagem fatal se o processo atual estiver
// rodando como root, ou "" se pode prosseguir. Ponto de decisao testavel.
func jailRootDenied() string {
	if err := jailRefuseRoot(jailGeteuid()); err != nil {
		return err.Error()
	}
	return ""
}

// readApparmorUsernsRestricted le o sysctl
// kernel.apparmor_restrict_unprivileged_userns (pura, testavel com arquivos
// temporarios). Retorna true apenas se o conteudo trimado for "1"; arquivo
// inexistente ou ilegivel retorna false.
func readApparmorUsernsRestricted(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == "1"
}

// apparmorUsernsRestricted detecta se o kernel bloqueia user namespaces nao
// privilegiados (AppArmor 4.x + Ubuntu 24.04) — causa raiz do bwrap
// "setting up uid map: Permission denied". Arquivo ausente (kernel sem o
// sysctl) retorna false.
func apparmorUsernsRestricted() bool {
	return readApparmorUsernsRestricted(ApparmorUsernsSysctlPath)
}

// bwrapFailureHint decide se a falha do bwrap merece o diagnostico especifico
// do sysctl kernel.apparmor_restrict_unprivileged_userns. Pura e testavel sem
// rodar bwrap: retorna "" quando nao aplicavel (sem restricao OU o output
// capturado do bwrap nao contem "uid map"/"permission denied", para nao
// poluir outras falhas). O bwrap escreve a mensagem de erro no stderr (que o
// ReexecInJail captura com io.MultiWriter), entao a analise opera sobre esse
// texto — o error retornado pelo cmd.Run() e apenas "exit status N".
func bwrapFailureHint(restricted bool, bwrapOutput string) string {
	if !restricted {
		return ""
	}
	msg := strings.ToLower(bwrapOutput)
	if !strings.Contains(msg, "uid map") && !strings.Contains(msg, "permission denied") {
		return ""
	}
	return "[COSCA] DETECTED: kernel.apparmor_restrict_unprivileged_userns=1 blocks user namespaces (bwrap cannot set up uid_map).\n" +
		"[COSCA] Fix (as root): sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0\n" +
		"[COSCA] Persist across reboots: echo 'kernel.apparmor_restrict_unprivileged_userns=0' | sudo tee /etc/sysctl.d/99-cosca-userns.conf && sudo sysctl --system"
}

// ReexecInJail cria uma copia executavel do binario em RAM e reexecuta
// via Bubblewrap. So deve ser chamada se InsideJail() == false.
//
// MODO NO-ROOT (fallback): se a jaula nao puder ser estabelecida — binario
// ausente, user namespaces bloqueados (ex: "setting up uid map: Permission
// denied"), falha ao criar a copia em RAM — a decisao e fail-closed:
// NUNCA roda direto em silencio. Por default imprime um SECURITY WARNING
// alto e segue sem sandbox (tradeoff aceito pelo Don neste ambiente de dev);
// com COSCA_ALLOW_NO_ROOT=1 roda silencioso (opt-in explicito).
// Rodar como ROOT e recusado sempre (exit 1).
//
// DISTINCAO FALHA DE JAULA x FALHA DE COMANDO (BUGFIX L218): o exit != 0 do
// processo INTERNO (command failure legitimo, ex: config ausente) NAO e
// falha de sandbox. O canal --info-fd (jailBooted) prova que a jaula subiu e
// o processo rodou; nesse caso o exit code do comando e propagado e o
// fallback sem-jaula e IMPOSSIVEL de disparar por erro do comando interno.
func ReexecInJail() {
	// --- 0. Container: o isolamento ja existe — jail e redundante ----------
	// Docker/containerd/Podman/K8s ja proveem namespaces. Tentar bwrap
	// dentro de container sem --privileged falha (user namespaces proibidos
	// pelo perfil seccomp default). Em vez de warning + fallback, pulamos
	// silenciosamente: o container e a jaula.
	//
	// COSCA_FORCE_JAIL=1 sobrescreve (para ambientes que rodam bwrap
	// dentro de container com --privileged).
	if IsRunningInContainer() {
		return
	}

	// --- 1. Recusa rodar como root (fail-closed, sem opt-in) --------------
	// Root + bwrap = sem user namespace = jail anulavel. Nao existe jaula
	// segura para root com bwrap; recusamos antes de qualquer fallback.
	if msg := jailRootDenied(); msg != "" {
		fmt.Fprintf(os.Stderr, "\n  [COSCA] SECURITY ERROR: %s\n\n", msg)
		os.Exit(1)
	}

	// --- 2. Pre-flight: a jaula pode ser estabelecida? ---------------------
	if ok, reason := jailAvailable(); !ok {
		if !jailFallback(reason) {
			os.Exit(1)
		}
		return
	}

	// --- 3. Le o proprio binario --------------------------------------------
	self, err := os.ReadFile("/proc/self/exe")
	if err != nil {
		if !jailFallback(fmt.Sprintf("cannot read /proc/self/exe: %v", err)) {
			os.Exit(1)
		}
		return
	}

	// --- 4. Cria copia executavel em RAM ------------------------------------
	// O *os.File devolvido vira ExtraFiles[0] do bwrap → fd 3 do filho; o
	// caminho dentro da jaula é o literal estável jailBinaryPathInChild
	// (/proc/self/fd/3). O fd original do pai pode ser QUALQUER número — Go
	// aloca conforme fds já abertos (epoll, signalfd...) — e o bwrap realoca
	// os fds herdados em 3+i, então o caminho /proc/self/fd/<fdDoPai> NUNCA
	// deve ser usado (apontaria para o canal de info ou nada).
	jailFile, cleanup, err := createJailBinary(self)
	if err != nil {
		if !jailFallback(fmt.Sprintf("failed to create in-memory executable copy: %v", err)) {
			os.Exit(1)
		}
		return
	}
	defer cleanup()
	jailPath := jailBinaryPathInChild

	// --- 5. Prepara jaula com escopo de workspace --------------------------
	// A jaula so enxerga o diretorio do projeto + o minimo do sistema.
	// Nada de /tmp do host, nada de /root/.config/cosca global.
	// Se o agente sair do workspace, nao encontra nada.
	workspaceDir, _ := os.Getwd()
	if err := validateJailWorkspace(workspaceDir); err != nil {
		if !jailFallback(err.Error()) {
			os.Exit(1)
		}
		return
	}

	// Le constraints do workspace (.cosca/constraints.yaml)
	constraints := loadConstraints(workspaceDir)

	// --- 5.1. Prepara destinos dos binds de sistema no root=workspace -------
	// O workspace é o "/" da jaula; os binds de sistema (/usr, /lib, /lib64,
	// /etc) montam por cima. O bwrap criaria esses diretórios sozinho dentro
	// do workspace (poluindo-o); criamos antes com controle e removemos no
	// cleanup se continuarem vazios.
	createdBinds := prepareWorkspaceBinds(workspaceDir)
	// NOTA: os.Exit NÃO executa defers — o cleanup dos destinos de bind precisa
	// ser chamado explicitamente em todos os caminhos de saída (exit), senão o
	// bwrap deixa diretórios/mountpoints vazios dentro do workspace.

	// Segredos COSCA_* (JWT/metrics) sao estagiados em .cosca/jail-secrets.env
	// (0600) em vez de --setenv: o valor NAO aparece no argv do bwrap (ps aux,
	// systemctl status, journald). O arquivo e removido no cleanup, em qualquer
	// caminho de saida.
	var secretsPath string
	exit := func(code int) {
		cleanupWorkspaceBinds(createdBinds)
		if secretsPath != "" {
			os.Remove(secretsPath)
		}
		os.Exit(code)
	}
	secretsPath, secretsErr := writeJailSecretsFile(workspaceDir)
	if secretsErr != nil {
		if !jailFallback(fmt.Sprintf("cannot stage jail secrets file: %v", secretsErr)) {
			exit(1)
		}
		cleanupWorkspaceBinds(createdBinds)
		return
	}
	if secretsPath != "" {
		defer os.Remove(secretsPath)
	}

	// --- 6. Prepara argumentos do bwrap --------------------------------------
	args := buildJailArgs(workspaceDir, constraints, jailPath)

	// --- 7. Fork/exec: roda bwrap e espera ----------------------------------
	// Usamos exec.Command + Run() em vez de syscall.Exec direto para que o
	// processo PAI sobreviva, faca cleanup (defer cleanup() acima) e so entao
	// termine. O FILHO e o bwrap, que por sua vez executa o cosca na jaula.
	// Resolve and validate again at the point of execution. PATH can change
	// between preflight and launch.
	bwrapPath, lookErr := validatedBwrapPath()
	if lookErr != nil {
		if !jailFallback("bubblewrap disappeared before launch") {
			exit(1)
		}
		cleanupWorkspaceBinds(createdBinds)
		return
	}

	// --- 7.1. Canal de prova de boot da jaula (BUGFIX L218) -----------------
	// Sem este canal, um exit != 0 do COMANDO INTERNO era indistinguivel de
	// uma falha de montagem do bwrap: o pai printava o falso "jail
	// unavailable" e, com COSCA_ALLOW_NO_ROOT=1, reexecutava SEM sandbox.
	//
	// Solucao: um pipe; a ponta de escrita vira o ExtraFiles[1] do bwrap
	// (fd 4 do filho) registrado com --info-fd 4. O bwrap escreve a info da
	// sandbox nesse fd assim que termina de montar a jaula, ANTES de executar
	// o processo interno. Os/exec cuida de limpar CLOEXEC no fd herdado — o
	// bwrap leva o canal intacto.
	//
	// O fd do canal NUNCA pode ser 3: ExtraFiles[0] (o proprio binario memfd)
	// ocupa o fd 3 do filho e o caminho /proc/self/fd/3 precisa apontar para
	// o BINARIO — colisao de fd matava todo comando preso com exit 1 mudo
	// (descoberto na 2a rodada do BUGFIX L218, ver L219).
	infoR, infoW, pipeErr := os.Pipe()
	if pipeErr != nil {
		if !jailFallback(fmt.Sprintf("cannot create jail boot channel: %v", pipeErr)) {
			exit(1)
		}
		cleanupWorkspaceBinds(createdBinds)
		return
	}
	defer infoR.Close()
	args = jailInjectInfoFD(args, jailPath, jailBootInfoFD)

	cmd := exec.Command(bwrapPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	// bwrap escreve o motivo da falha no stderr (ex: "setting up uid map:
	// Permission denied"). Encaminhamos ao vivo para o terminal E capturamos
	// em buffer: o error retornado pelo cmd.Run() so carrega "exit status N",
	// entao o diagnostico especifico precisa analisar o texto capturado.
	var bwrapStderr bytes.Buffer
	cmd.Stderr = &bwrapStderr
	cmd.Env = jailEnvironment()
	// ExtraFiles[0] → fd 3 do filho = o binário (jailPath); ExtraFiles[1] →
	// fd 4 = o canal de info (--info-fd 4). A ORDEM é sagrada: o binário
	// precisa do fd 3 fixo (o caminho /proc/self/fd/3 depende disso).
	cmd.ExtraFiles = []*os.File{jailFile, infoW}

	// Propagar sinais do terminal para o grupo de processo do bwrap
	setupSignalForwarding(cmd)

	if err := cmd.Run(); err != nil {
		// A jaula subiu e o processo interno rodou — o bwrap escreveu a info
		// no canal antes de executa-lo. O exit != 0 é do COMANDO, não da
		// jaula (BUGFIX L218): propaga o exit code real, NUNCA trata como
		// falha de sandbox e NUNCA cai no fallback sem jaula. Sintoma clássico
		// do falso "jail unavailable": erro interno (ex: "No models found")
		// aparecia como "bwrap failed to establish the sandbox".
		infoW.Close() // bwrap já morreu — garante EOF se ele não escreveu nada
		if jailBooted(infoR) {
			cleanupWorkspaceBinds(createdBinds)
			if secretsPath != "" {
				os.Remove(secretsPath)
			}
			os.Exit(jailExitCodeFromRun(err))
		}
		// Fail-closed: o bwrap morreu ANTES de completar a montagem da
		// sandbox (ex: user namespaces bloqueados — "setting up uid map:
		// Permission denied"). NUNCA roda direto em silencio: alerta alto por
		// default, silencioso apenas com COSCA_ALLOW_NO_ROOT=1.
		//
		// Diagnostico especifico: se o kernel restringe user namespaces nao
		// privilegiados (AppArmor 4.x) e o erro e o de uid map, avisamos o
		// operador QUAL e o conserto (sysctl) ANTES do fallback generico.
		diagnostic := sanitizeJailDiagnostic(bwrapStderr.String())
		if hint := bwrapFailureHint(apparmorUsernsRestricted(), diagnostic); hint != "" {
			fmt.Fprintf(os.Stderr, "\n  %s\n\n", strings.ReplaceAll(hint, "\n", "\n  "))
		}
		if !jailFallback(fmt.Sprintf("bwrap failed to establish the sandbox: %s", diagnostic)) {
			exit(1)
		}
		cleanupWorkspaceBinds(createdBinds)
		return // fallback no-root
	}
	exit(0)
}

// Caminhos canônicos do toolchain Go dentro da jaula. HOME=/ na jaula, então
// GOMODCACHE/GOCACHE são fixados explicitamente (remap — nunca os caminhos do
// host) e os binds em buildJailArgs tornam os destinos reais dentro da jaula.
const (
	// jailGoModCacheDest é onde o cache de módulos do host aparece na jaula.
	jailGoModCacheDest = "/go/pkg/mod"
	// jailGoBuildCacheDest é o cache de build da jaula (tmpfs privado por jail).
	jailGoBuildCacheDest = "/go-build-cache"
	// jailGoPathDest é o GOPATH canônico da jaula (GOMODCACHE = GOPATH/pkg/mod).
	jailGoPathDest = "/go"
)

// jailGoModCacheHost devolve o cache de módulos do HOST para bindar na jaula.
// Prioridade: GOMODCACHE do host; fallback $HOME/go/pkg/mod. "" se não existe.
func jailGoModCacheHost() string {
	mod := os.Getenv("GOMODCACHE")
	if mod == "" {
		if home, err := os.UserHomeDir(); err == nil {
			mod = filepath.Join(home, "go", "pkg", "mod")
		}
	}
	if mod == "" {
		return ""
	}
	if info, err := os.Stat(mod); err != nil || !info.IsDir() {
		return ""
	}
	return mod
}

// jailResolvConfTarget resolve /etc/resolv.conf. Em sistemas com
// systemd-resolved ele é um symlink para /run/systemd/resolve/...; o bind de
// /etc (--ro-bind /etc /etc) carrega o symlink QUEBRADO dentro da jaula
// porque /run não é montado. Devolve o alvo real para ser bindado no próprio
// caminho — o symlink volta a resolver. "" se é um arquivo regular (o bind de
// /etc já cobre /etc/resolv.conf) ou inacessível.
func jailResolvConfTarget() string {
	resolved, err := filepath.EvalSymlinks("/etc/resolv.conf")
	if err != nil {
		return ""
	}
	if resolved == "/etc/resolv.conf" || strings.HasPrefix(resolved, "/etc/") {
		return ""
	}
	return resolved
}

// jailGoToolchainEnv devolve os pares --setenv do toolchain Go com os caminhos
// REMAPEADOS para a jaula. Não usa os caminhos do host: HOME=/ na jaula
// tornaria /home/cosca/go/pkg/mod inalcançável. GOTELEMETRY=off impede o go de
// gravar telemetria em /.config/go/telemetry dentro do workspace (poluição a
// cada jail).
func jailGoToolchainEnv() []string {
	set := []string{
		"--setenv", "GOMODCACHE", jailGoModCacheDest,
		"--setenv", "GOCACHE", jailGoBuildCacheDest,
		"--setenv", "GOPATH", jailGoPathDest,
	}
	goproxy := os.Getenv("GOPROXY")
	if goproxy == "" {
		goproxy = "https://proxy.golang.org,direct"
	}
	set = append(set, "--setenv", "GOPROXY", goproxy)
	set = append(set, "--setenv", "GOTELEMETRY", "off")
	return set
}

// buildJailArgs monta os argumentos do bwrap (exceto o argv0).
// Separada de ReexecInJail para testabilidade (--die-with-parent etc.).
//
// REGRA DE RAÍZ (root = workspace): o workspace É montado como "/" da jaula.
// Não existe /home, /var, /root nem qualquer caminho fora do workspace — o
// pai de "/" é "/" (cd .. não sai). Path transversal é impossível: não há
// nada para atravessar. Os binds de sistema (/usr, /lib, /lib64, /etc) são
// montados POR CIMA do root — a ORDEM importa: um bind em "/" mascara os
// mounts feitos antes, então o workspace entra primeiro e o sistema depois.
func buildJailArgs(workspaceDir string, c *config.Constraints, jailPath string) []string {
	args := []string{}

	// Workspace VIRA O ROOT da jaula — ro ou rw conforme constraints.
	// Deve vir ANTES dos binds de sistema (bind em "/" mascara mounts
	// anteriores; o sistema precisa montar por cima do workspace).
	if c.ReadOnly {
		args = append(args, "--ro-bind", workspaceDir, "/")
	} else {
		args = append(args, "--bind", workspaceDir, "/")
	}

	// Sistema minimo (ro) — montado por cima do root=workspace
	sysBinds := [][2]string{
		{"/usr", "/usr"},
		{"/lib", "/lib"},
		{"/lib64", "/lib64"},
		{"/etc", "/etc"},
	}
	for _, b := range sysBinds {
		args = append(args, "--ro-bind", b[0], b[1])
	}

	// Toolchain Go (montado por cima do root=workspace, como os binds acima).
	// Cache de módulos do host (ro): a jaula lê os módulos pré-baixados sem
	// poder modificá-los — injeção de dependência impossível. O download de
	// módulos NOVOS continua sendo responsabilidade do host.
	if modCache := jailGoModCacheHost(); modCache != "" {
		args = append(args, "--ro-bind", modCache, jailGoModCacheDest)
	}
	// Cache de build: o go SEMPRE grava artefatos de build no GOCACHE — um
	// --ro-bind do cache do host falharia ("read-only file system"), e um
	// --bind daria ao agente o poder de injetar artefatos reutilizados depois
	// pelo build do próprio host. Usamos um tmpfs privado por jail: nenhum
	// write chega ao cache do host e nada host é exposto à jaula.
	args = append(args, "--tmpfs", jailGoBuildCacheDest)
	// DNS: /etc/resolv.conf do host é symlink para /run/... (systemd-resolved);
	// o bind de /etc carrega o symlink quebrado. Bindamos o alvo real no mesmo
	// caminho para o symlink voltar a resolver dentro da jaula.
	if resolv := jailResolvConfTarget(); resolv != "" {
		args = append(args, "--ro-bind", resolv, resolv)
	}

	// /tmp isolado (tmpfs privado)
	args = append(args, "--tmpfs", "/tmp")

	// Dispositivos e proc
	// --dev DEST recebe UM argumento: monta um /dev novo em DEST. O antigo
	// "--dev /dev /dev" fazia o bwrap tratar o segundo /dev como programa a
	// executar → "bwrap: execvp /dev: Permission denied".
	args = append(args, "--dev", "/dev")
	args = append(args, "--proc", "/proc")

	// Namespaces de isolamento.
	//
	// --die-with-parent: se o PAI morrer (kill -9, crash), o bwrap — e tudo
	// dentro da jaula — e SIGKILLed junto. Nenhum processo orfao sobrevive
	// sem supervisao (correcao red team).
	args = append(args, "--die-with-parent")
	//
	// --unshare-all: unshare de todos os namespaces suportados (mount, ipc,
	// pid, uts, cgroup, net). Com Network=true usamos --share-net (combinavel
	// apenas com --unshare-all) para preservar a rede conforme a constraint.
	// O euid==0 e recusado ANTES de chegar aqui (root + bwrap nao cria user
	// namespace → jail anulavel); para usuario normal adicionamos
	// --unshare-user explicitamente (--unshare-all NAO inclui user namespace).
	args = append(args, "--unshare-all")
	if c.Network {
		args = append(args, "--share-net")
	}
	if jailGeteuid() != 0 {
		args = append(args, "--unshare-user")
	}
	args = append(args, "--hostname", "cosca-jail")
	args = append(args, "--chdir", "/")

	// Variaveis de ambiente
	// Nunca herda o ambiente do host: em particular API keys e credenciais.
	args = append(args, "--clearenv")
	args = append(args, "--setenv", EnvJailed, "1")
	args = append(args, "--setenv", EnvJailPID, strconv.Itoa(os.Getpid()))
	args = append(args, "--setenv", "HOME", "/")
	args = append(args, "--setenv", "PATH", "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")
	// COSCA_PROJECT_DIR aponta o workspace ORIGINAL do host (o root da jaula
	// é o workspace, então /home/cosca/... não existe lá dentro). Comandos que
	// recebem caminhos absolutos do host (ex.: graph export -o /abs/path)
	// reescrevem para o caminho relativo equivalente dentro do workspace.
	// Vem DEPOIS do --clearenv: um --setenv antes do clearenv é varrido junto
	// com o resto do ambiente (BUGFIX L335).
	args = append(args, "--setenv", "COSCA_PROJECT_DIR", workspaceDir)
	for _, key := range []string{
		"TERM", "COLORTERM", "TERM_PROGRAM",
		"NO_COLOR", "CLICOLOR", "CLICOLOR_FORCE",
	} {
		if value := os.Getenv(key); value != "" {
			args = append(args, "--setenv", key, value)
		}
	}

	// Toolchain Go: caminhos remapeados para a jaula (jailGoToolchainEnv).
	// Com HOME=/, sem esses --setenv o go resolveria GOMODCACHE=/go/pkg/mod e
	// GOCACHE=/.cache/go-build — caminhos que não existem dentro da jaula.
	args = append(args, jailGoToolchainEnv()...)

	// Propaga a configuração do Cosca (COSCA_*) para dentro da jaula.
	// Sem isso o daemon (cosca serve) perdia COSCA_JWT_SECRET no --clearenv
	// e se recusava a subir (fail-closed, "COSCA_JWT_SECRET not set"). O
	// restante do ambiente do host permanece descartado: a jaula nunca
	// herda chaves de terceiros (BUGFIX 2026-08-04).
	//
	// SEGREDOS (COSCA_JWT_SECRET, COSCA_METRICS_SECRET etc.) NÃO entram aqui:
	// o valor apareceria no argv do bwrap (visível por ps aux, systemctl
	// status e journald). Eles são entregues via .cosca/jail-secrets.env
	// (estagiado antes do launch, removido no cleanup) e carregados por
	// LoadJailSecrets() assim que a jaula sobe.
	for _, item := range jailPropagatedEnv() {
		key, value, _ := strings.Cut(item, "=")
		if isJailSecretEnv(key) {
			continue
		}
		args = append(args, "--setenv", key, value)
	}

	// Caminho do binario dentro da jaula + args originais
	args = append(args, jailPath)
	if len(os.Args) > 1 {
		args = append(args, os.Args[1:]...)
	}
	return args
}

// jailBindDirs são os destinos dos binds de sistema (montados por cima do
// root=workspace). O bwrap precisa que esses diretórios existam no destino;
// com o workspace como "/", os destinos ficam DENTRO do workspace. Criamos
// antes (para controle) e removemos depois se continuarem vazios — nunca
// tocamos em diretórios que já existiam no workspace antes do jail.
var jailBindDirs = []string{
	"usr", "lib", "lib64", "etc",
	// Toolchain Go + DNS: destinos dos binds adicionados em buildJailArgs.
	"go-build-cache", "go/pkg/mod", "run/systemd/resolve",
}

// prepareWorkspaceBinds garante que os destinos dos binds de sistema existam
// no workspace (o bwrap monta por cima deles dentro da jaula). Retorna os
// paths que foram criados por esta chamada — os únicos elegíveis para cleanup.
func prepareWorkspaceBinds(workspaceDir string) []string {
	var created []string
	for _, d := range jailBindDirs {
		p := filepath.Join(workspaceDir, d)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			if err := os.MkdirAll(p, 0o755); err == nil {
				created = append(created, p)
			}
		}
	}
	// O bind de DNS é um ARQUIVO (--ro-bind SRC SRC, onde SRC é o alvo real do
	// symlink /etc/resolv.conf). O bwrap monta por cima de um mountpoint no
	// workspace; pré-criamos o arquivo vazio para controle e para o cleanup
	// saber removê-lo depois.
	if resolv := jailResolvConfTarget(); resolv != "" {
		file := filepath.Join(workspaceDir, filepath.FromSlash(resolv))
		if _, err := os.Stat(file); os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(file), 0o755); err == nil {
				if f, err := os.OpenFile(file, os.O_CREATE, 0o644); err == nil {
					f.Close()
					created = append(created, file)
				}
			}
		}
	}
	return created
}

// cleanupWorkspaceBinds remove os mountpoints criados por prepareWorkspaceBinds.
// Enquanto a jaula roda, os binds os ocultam (vazios); após o bwrap morrer os
// mounts desmontam e voltam a ser visíveis. removeEmptyUp remove arquivos
// vazios (mountpoints de binds de arquivo, ex: stub-resolv.conf) e sobe
// removendo diretórios vazios até encontrar conteúdo pré-existente — que nunca
// é tocado (o remove falha silenciosamente e o conteúdo permanece intacto).
func cleanupWorkspaceBinds(created []string) {
	for _, p := range created {
		removeEmptyUp(p)
	}
}

// createJailBinary cria uma copia executavel do binario em RAM e devolve o
// *os.File correspondente (para ser entregue ao bwrap como ExtraFiles[0] →
// fd 3 do filho). Tenta memfd_create primeiro (100% RAM, zero disco). Se
// falhar (kernel antigo sem suporte), fallback para /tmp/ (tmpfs = RAM).
//
// Retorna: arquivo executável aberto, funcao de cleanup, erro.
func createJailBinary(self []byte) (*os.File, func(), error) {
	file, cleanup, err := createMemfd(self)
	if err == nil {
		return file, cleanup, nil
	}

	// Fallback: arquivo temporario em /tmp/ (tmpfs = RAM no Ubuntu)
	return createTempFile(self)
}

// createMemfd tenta criar um memfd com o binario.
// O fd e CLOEXEC-free para sobreviver ao exec do bwrap.
func createMemfd(self []byte) (*os.File, func(), error) {
	// Gera nome unico para o memfd
	randBytes := make([]byte, 8)
	if _, err := rand.Read(randBytes); err != nil {
		return nil, nil, err
	}
	name := "cosca-jail-" + hex.EncodeToString(randBytes)

	// Cria memfd com flag 0 (sem MFD_CLOEXEC — o fd precisa ser herdado)
	fd, err := unix.MemfdCreate(name, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("unix.MemfdCreate: %w", err)
	}

	// Escreve binario
	if _, err := unix.Write(fd, self); err != nil {
		unix.Close(fd)
		return nil, nil, fmt.Errorf("memfd write: %w", err)
	}

	// Torna executavel (owner read+execute)
	if err := unix.Fchmod(fd, 0500); err != nil {
		unix.Close(fd)
		return nil, nil, fmt.Errorf("memfd fchmod: %w", err)
	}

	// Cleanup is deliberately idempotent. Callers commonly defer cleanup while
	// also cleaning up on an assertion/error path; a second close could close a
	// different descriptor that was allocated after fd was released.
	//
	// CRITICAL: the returned *os.File must NOT be left to the GC finalizer.
	// os.NewFile registers the fd with the runtime; if the finalizer runs after
	// cleanup() already closed the fd, it will close whatever descriptor the OS
	// has since recycled (e.g. an HTTP socket of another goroutine), corrupting
	// unrelated I/O. SetFinalizer(f, nil) makes the manual cleanup the sole
	// owner of the fd, and os.File.Close() is safe to call twice (the second
	// call returns ErrClosed without touching the fd).
	f := os.NewFile(uintptr(fd), name)
	runtime.SetFinalizer(f, nil)
	var closeOnce sync.Once
	cleanup := func() {
		closeOnce.Do(func() { _ = f.Close() })
	}

	return f, cleanup, nil
}

// setupSignalForwarding propaga sinais do terminal para o grupo de processo do bwrap.
func setupSignalForwarding(cmd *exec.Cmd) {
	sigCh := make(chan os.Signal, 8)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		for sig := range sigCh {
			if cmd.Process != nil {
				pgid, err := syscall.Getpgid(cmd.Process.Pid)
				if err == nil {
					syscall.Kill(-pgid, sig.(syscall.Signal))
				} else {
					cmd.Process.Signal(sig)
				}
			}
		}
	}()
}
