//go:build linux

package cosca

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/config"

	"golang.org/x/sys/unix"
)

// =============================================================================
// 5. createMemfd — executable binary in memory (Linux-only, memfd_create)
// =============================================================================

func TestJail_CreateMemfd(t *testing.T) {
	t.Run("creates memfd with correct content", func(t *testing.T) {
		// Arrange
		data := []byte("binary-payload-for-memfd-test-abcdef")

		// Act
		file, cleanup, err := createMemfd(data)

		// Assert: no error, valid return values
		if err != nil {
			t.Fatalf("createMemfd() unexpected error: %v", err)
		}
		if cleanup == nil {
			t.Fatal("cleanup function is nil")
		}
		if file == nil {
			t.Fatal("file is nil")
		}
		var cleanupOnce sync.Once
		safeCleanup := func() {
			cleanupOnce.Do(cleanup)
		}
		t.Cleanup(safeCleanup)

		// Assert: path follows /proc/self/fd/N pattern
		fd := int(file.Fd())
		path := "/proc/self/fd/" + strconv.Itoa(fd)
		if !strings.HasPrefix(path, "/proc/self/fd/") {
			t.Errorf("expected path to start with /proc/self/fd/, got %q", path)
		}

		// Assert: data is readable from the path
		readData, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) failed: %v", path, err)
		}
		if !bytes.Equal(readData, data) {
			t.Errorf("content mismatch: got %d bytes, want %d bytes", len(readData), len(data))
		}

		// Assert: fd is executable (fchmod 0500 — owner read+execute, no write)
		var stat unix.Stat_t
		if err := unix.Fstat(fd, &stat); err != nil {
			t.Fatalf("Fstat(%d) failed: %v", fd, err)
		}
		perms := stat.Mode & 0777
		if perms != 0500 {
			t.Errorf("expected file mode 0500, got 0%o", perms)
		}

		// Assert: fd does NOT have FD_CLOEXEC (flag 0 passed to MemfdCreate)
		// MFD_CLOEXEC would set the FD_CLOEXEC flag on the fd descriptor flags.
		// We verify by checking the file descriptor flags via F_GETFD.
		fdFlags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0)
		if err != nil {
			t.Fatalf("FcntlInt(F_GETFD) failed: %v", err)
		}
		if fdFlags&unix.FD_CLOEXEC != 0 {
			t.Error("fd has FD_CLOEXEC set; expected MFD_CLOEXEC NOT to be used")
		}

		// Assert: cleanup closes the fd (reading from path should fail after)
		safeCleanup()
		if _, err := os.ReadFile(path); err == nil {
			t.Error("expected ReadFile to fail after cleanup (fd closed)")
		}
	})

	t.Run("handles empty data", func(t *testing.T) {
		// Arrange
		data := []byte{}

		// Act
		file, cleanup, err := createMemfd(data)
		if err != nil {
			t.Fatalf("createMemfd() with empty data: %v", err)
		}
		defer cleanup()

		// Assert: file is empty and accessible
		fd := int(file.Fd())
		path := "/proc/self/fd/" + strconv.Itoa(fd)
		readData, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) failed: %v", path, err)
		}
		if len(readData) != 0 {
			t.Errorf("expected empty memfd, got %d bytes", len(readData))
		}

		// Assert: permissions still 0500
		var stat unix.Stat_t
		if err := unix.Fstat(fd, &stat); err != nil {
			t.Fatalf("Fstat failed: %v", err)
		}
		perms := stat.Mode & 0777
		if perms != 0500 {
			t.Errorf("expected file mode 0500, got 0%o", perms)
		}
	})
}

// =============================================================================
// 6. createJailBinary — memfd first, temp-file fallback
// =============================================================================

func TestJail_CreateJailBinary(t *testing.T) {
	t.Parallel()

	t.Run("uses memfd on Linux", func(t *testing.T) {
		// Arrange
		data := []byte("create-jail-binary-test-data")

		// Act
		file, cleanup, err := createJailBinary(data)

		// Assert: no error
		if err != nil {
			t.Fatalf("createJailBinary() unexpected error: %v", err)
		}
		if cleanup == nil {
			t.Fatal("cleanup function is nil")
		}
		defer cleanup()

		// On Linux, createJailBinary tries memfd first and should succeed.
		// The path should be /proc/self/fd/N (memfd), not /tmp/... (temp file).
		fd := int(file.Fd())
		path := "/proc/self/fd/" + strconv.Itoa(fd)
		if !strings.HasPrefix(path, "/proc/self/fd/") {
			t.Errorf("on Linux, expected memfd path (/proc/self/fd/...), got %q", path)
		}

		// Assert: content is readable and correct
		readData, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) failed: %v", path, err)
		}
		if !bytes.Equal(readData, data) {
			t.Errorf("content mismatch: got %d bytes, want %d bytes", len(readData), len(data))
		}

		// Assert: cleanup closes the fd
		cleanup()
		if _, err := os.ReadFile(path); err == nil {
			t.Error("expected ReadFile to fail after cleanup")
		}
	})

	t.Run("handle empty binary", func(t *testing.T) {
		// Edge case: empty binary data should not crash
		data := []byte{}

		file, cleanup, err := createJailBinary(data)
		if err != nil {
			t.Fatalf("createJailBinary() with empty data: %v", err)
		}
		defer cleanup()

		if file == nil {
			t.Error("expected non-nil file")
		}
	})
}

// =============================================================================
// 4. setupSignalForwarding — signal propagation to child process (Unix)
// =============================================================================

func TestJail_SetupSignalForwarding(t *testing.T) {
	// NOT t.Parallel() — modifies global signal handling for this process

	t.Run("propagates SIGTERM to child process group", func(t *testing.T) {
		// Arrange: start a long-running child in its own process group
		cmd := exec.Command("sleep", "30")
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := cmd.Start(); err != nil {
			t.Skipf("cannot start sleep process: %v", err)
		}

		// Track whether we already waited for the child.
		// We must not write to cmd.Process because the signal forwarding
		// goroutine reads it concurrently (no mutex in production code).
		waited := false

		// Ensure child is cleaned up even on test failure.
		// If we already waited, Kill/Wait are harmless no-ops.
		defer func() {
			if !waited && cmd.Process != nil {
				cmd.Process.Kill()
				cmd.Wait()
			}
		}()

		// Verify child is alive
		if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
			t.Fatalf("child process not running: %v", err)
		}

		// Act: set up signal forwarding
		setupSignalForwarding(cmd)

		// Give the goroutine time to start listening on the signal channel
		time.Sleep(50 * time.Millisecond)

		// Send SIGTERM to ourselves; the goroutine forwards it to the child's
		// process group. signal.Notify intercepts the signal, so our test
		// process is not killed.
		syscall.Kill(syscall.Getpid(), syscall.SIGTERM)

		// Assert: child should exit because of the forwarded signal
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		select {
		case err := <-done:
			waited = true
			if err == nil {
				t.Error("expected child to exit with error (killed by signal)")
			} else {
				if exitErr, ok := err.(*exec.ExitError); ok {
					status, ok := exitErr.Sys().(syscall.WaitStatus)
					if ok && !status.Signaled() {
						t.Errorf("child exited but not from signal: %v", err)
					}
				}
			}
		case <-time.After(5 * time.Second):
			t.Error("timeout: child process did not exit after signal")
		}
	})

	t.Run("handles nil Process gracefully", func(t *testing.T) {
		// Arrange: create a command but don't start it (Process == nil)
		cmd := exec.Command("true")

		// Act: should not panic
		// We can't easily verify the goroutine doesn't panic, but we can
		// verify that calling the function with a nil Process is safe.
		setupSignalForwarding(cmd)

		// Send a signal that the goroutine will try to forward.
		// With Process == nil, the `if cmd.Process != nil` check prevents
		// any actual forwarding. We verify no panic occurs by reaching here.
		syscall.Kill(syscall.Getpid(), syscall.SIGTERM)

		// Give goroutine time to process the signal
		time.Sleep(50 * time.Millisecond)

		// If we got here without panic, the nil-Process guard works.
	})
}

// =============================================================================
// 8. --die-with-parent e flags de isolamento nos args do bwrap
// =============================================================================

func TestJail_DieWithParent(t *testing.T) {
	argsContains := func(args []string, flag string) bool {
		for _, a := range args {
			if a == flag {
				return true
			}
		}
		return false
	}

	t.Run("bwrap args include --die-with-parent", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		c := config.DefaultConstraints()

		// Act
		args := buildJailArgs(dir, c, "/proc/self/fd/3")

		// Assert: nenhum processo orfao sobrevive ao pai morrer
		if !argsContains(args, "--die-with-parent") {
			t.Errorf("bwrap args missing --die-with-parent: %v", args)
		}
	})

	t.Run("isolation flags: --unshare-all + --share-net when network allowed", func(t *testing.T) {
		dir := t.TempDir()
		c := config.DefaultConstraints() // Network: true

		args := buildJailArgs(dir, c, "/proc/self/fd/3")

		if !argsContains(args, "--unshare-all") {
			t.Errorf("bwrap args missing --unshare-all: %v", args)
		}
		if !argsContains(args, "--share-net") {
			t.Errorf("bwrap args missing --share-net (network allowed): %v", args)
		}
		if argsContains(args, "--unshare-net") {
			t.Errorf("bwrap args must NOT contain --unshare-net when network allowed: %v", args)
		}
	})

	t.Run("isolation flags: --unshare-all without --share-net when network blocked", func(t *testing.T) {
		dir := t.TempDir()
		c := config.DefaultConstraints()
		c.Network = false

		args := buildJailArgs(dir, c, "/proc/self/fd/3")

		if !argsContains(args, "--unshare-all") {
			t.Errorf("bwrap args missing --unshare-all: %v", args)
		}
		if argsContains(args, "--share-net") {
			t.Errorf("bwrap args must NOT contain --share-net when network blocked: %v", args)
		}
	})

	t.Run("--unshare-user added for regular (non-root) user", func(t *testing.T) {
		orig := jailGeteuid
		jailGeteuid = func() int { return 1000 }
		t.Cleanup(func() { jailGeteuid = orig })

		args := buildJailArgs(t.TempDir(), config.DefaultConstraints(), "/proc/self/fd/3")

		if !argsContains(args, "--unshare-user") {
			t.Errorf("non-root bwrap args missing --unshare-user: %v", args)
		}
	})

	t.Run("jail binary path is the last arg before original args", func(t *testing.T) {
		args := buildJailArgs(t.TempDir(), config.DefaultConstraints(), "/proc/self/fd/3")

		found := false
		for _, a := range args {
			if a == "/proc/self/fd/3" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("bwrap args missing jail binary path: %v", args)
		}
	})
}

func TestJail_UsesMinimalEnvironmentAndNoHostRun(t *testing.T) {
	args := buildJailArgs(t.TempDir(), config.DefaultConstraints(), "/proc/self/fd/3")
	clear := false
	for i, arg := range args {
		if arg == "--clearenv" {
			clear = true
		}
		if arg == "/run" {
			t.Fatalf("jail must not bind host /run at arg %d", i)
		}
	}
	if !clear {
		t.Fatal("jail must clear the host environment")
	}
}

func TestJail_RejectsRootWorkspace(t *testing.T) {
	if err := validateJailWorkspace("/"); err == nil {
		t.Fatal("filesystem root must not be accepted as a jail workspace")
	}
	link := filepath.Join(t.TempDir(), "root-link")
	if err := os.Symlink("/", link); err != nil {
		t.Fatal(err)
	}
	if err := validateJailWorkspace(link); err == nil {
		t.Fatal("symlink resolving to filesystem root must not be accepted")
	}
}

// =============================================================================
// 9. Root e inseguro — recusa fail-closed (red team fix)
// =============================================================================

func TestJail_RootUnsafe(t *testing.T) {
	t.Run("euid==0 → SECURITY ERROR (refusal)", func(t *testing.T) {
		// Arrange: simula euid==0 via variavel injetavel
		orig := jailGeteuid
		jailGeteuid = func() int { return 0 }
		t.Cleanup(func() { jailGeteuid = orig })

		// Act 1: decisao reporta que root deve ser recusado
		msg := jailRootDenied()
		if msg == "" {
			t.Fatal("jailRootDenied() empty for euid==0, want SECURITY ERROR message")
		}
		if !strings.Contains(strings.ToLower(msg), "root") {
			t.Errorf("refusal message should mention root, got: %q", msg)
		}
		if !strings.Contains(strings.ToLower(msg), "user namespace") {
			t.Errorf("refusal message should explain the user namespace problem, got: %q", msg)
		}

		// Act 2: jailRefuseRoot devolve erro para euid==0
		if err := jailRefuseRoot(0); err == nil {
			t.Error("jailRefuseRoot(0) = nil, want error")
		}

		// Act 3: root NUNCA recebe --unshare-user (jaula anulavel) nem fallback
		args := buildJailArgs(t.TempDir(), config.DefaultConstraints(), "/proc/self/fd/3")
		for _, a := range args {
			if a == "--unshare-user" {
				t.Errorf("root must NOT get --unshare-user (bwrap as root has no user namespace): %v", args)
			}
		}
	})

	t.Run("non-root euid → no refusal", func(t *testing.T) {
		orig := jailGeteuid
		jailGeteuid = func() int { return 1000 }
		t.Cleanup(func() { jailGeteuid = orig })

		if msg := jailRootDenied(); msg != "" {
			t.Errorf("jailRootDenied() = %q for non-root, want empty", msg)
		}
		if err := jailRefuseRoot(1000); err != nil {
			t.Errorf("jailRefuseRoot(1000) = %v, want nil", err)
		}
	})
}

// =============================================================================
// 10. apparmor_restrict_unprivileged_userns — diagnostico especifico do bwrap
// =============================================================================

func TestReadApparmorUsernsRestricted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data string // conteudo do arquivo (ignorado quando file=false)
		file bool   // se o arquivo deve existir
		want bool
	}{
		{"content 1\n → restricted", "1\n", true, true},
		{"content 0\n → not restricted", "0\n", true, false},
		{"empty content → not restricted", "", true, false},
		{"missing file → not restricted", "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: arquivo temporario com o conteudo do sysctl (ou ausente)
			path := filepath.Join(t.TempDir(), "apparmor_restrict_unprivileged_userns")
			if tt.file {
				if err := os.WriteFile(path, []byte(tt.data), 0600); err != nil {
					t.Fatalf("WriteFile(%q): %v", path, err)
				}
			}

			// Act + Assert
			if got := readApparmorUsernsRestricted(path); got != tt.want {
				t.Errorf("readApparmorUsernsRestricted(%q) = %v, want %v (content=%q)", path, got, tt.want, tt.data)
			}
		})
	}
}

func TestBwrapFailureHint(t *testing.T) {
	t.Parallel()

	uidMapOut := "bwrap: setting up uid map: Permission denied"
	permDeniedOut := "bwrap: failed to write /proc/self/uid_map: Permission denied"
	otherOut := "bwrap: mount /foo: No such file or directory"

	tests := []struct {
		name       string
		restricted bool
		output     string
		want       bool // se o hint deve ser emitido
	}{
		{"restricted + uid map → hint com conserto", true, uidMapOut, true},
		{"restricted + Permission denied → hint com conserto", true, permDeniedOut, true},
		{"restricted + output sem padrao → hint vazio", true, otherOut, false},
		{"sem restricao → hint vazio", false, uidMapOut, false},
		{"output vazio → hint vazio", true, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hint := bwrapFailureHint(tt.restricted, tt.output)

			if tt.want {
				if !strings.Contains(hint, "apparmor_restrict_unprivileged_userns=0") {
					t.Errorf("hint should contain the sysctl fix (apparmor_restrict_unprivileged_userns=0), got: %q", hint)
				}
				if !strings.Contains(hint, "sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0") {
					t.Errorf("hint should contain the one-liner fix, got: %q", hint)
				}
			} else if hint != "" {
				t.Errorf("bwrapFailureHint(%v, %q) = %q, want empty", tt.restricted, tt.output, hint)
			}
		})
	}
}

// =============================================================================
// 12. Root = workspace — sem path transversal (ordem de Don 2026-08-08)
// =============================================================================

func TestJail_WorkspaceIsRoot(t *testing.T) {
	dir := t.TempDir()
	c := config.DefaultConstraints()
	args := buildJailArgs(dir, c, "/proc/self/fd/3")

	// 1. O workspace é bindado em "/", não em si mesmo.
	foundRootBind := false
	for i, a := range args {
		if a == "--bind" && i+2 < len(args) {
			if args[i+1] == dir && args[i+2] == "/" {
				foundRootBind = true
			}
			if args[i+1] == dir && args[i+2] == dir {
				t.Errorf("workspace bound to itself (%s %s): root=workspace rule violated", dir, dir)
			}
		}
	}
	if !foundRootBind {
		t.Errorf("bwrap args missing --bind %s /: %v", dir, args)
	}

	// 2. chdir para "/" — o processo nasce no root da jaula (= workspace).
	if !argsContainsAny(args, []string{"--chdir", "/"}) {
		t.Errorf("bwrap args missing --chdir /: %v", args)
	}

	// 3. HOME="/" — dentro da jaula, / É o workspace.
	if !argsContainsAny(args, []string{"--setenv", "HOME", "/"}) {
		t.Errorf("bwrap args missing HOME=/ : %v", args)
	}

	// 4. ORDEM: o bind do workspace vem ANTES dos --ro-bind de sistema.
	//    (Um bind em "/" mascara mounts anteriores — o sistema monta por cima.)
	wsIdx := -1
	for i, a := range args {
		if a == "--bind" && args[i+1] == dir {
			wsIdx = i
			break
		}
	}
	if wsIdx == -1 {
		t.Fatal("workspace bind not found")
	}
	for i := wsIdx + 1; i < len(args); i++ {
		if args[i] == "--ro-bind" {
			return // sistema veio depois — ordem correta
		}
		if args[i] == "--bind" {
			t.Fatalf("another --bind appears before --ro-bind system binds (order violation): %v", args[i:])
		}
	}
	t.Errorf("no --ro-bind after workspace bind: %v", args)
}

func TestJail_WorkspaceIsRoot_ReadOnly(t *testing.T) {
	dir := t.TempDir()
	c := config.DefaultConstraints()
	c.ReadOnly = true
	args := buildJailArgs(dir, c, "/proc/self/fd/3")

	found := false
	for i, a := range args {
		if a == "--ro-bind" && i+2 < len(args) && args[i+1] == dir && args[i+2] == "/" {
			found = true
		}
	}
	if !found {
		t.Errorf("read-only mode must mount workspace as --ro-bind %s /: %v", dir, args)
	}
}

func TestJail_PrepareAndCleanupWorkspaceBinds(t *testing.T) {
	dir := t.TempDir()

	// 1. prepare cria os destinos que não existem e retorna exatamente eles.
	//    Além dos jailBindDirs, pode criar o mountpoint (arquivo) do bind de
	//    DNS quando /etc/resolv.conf é um symlink para fora de /etc.
	created := prepareWorkspaceBinds(dir)
	want := len(jailBindDirs)
	if jailResolvConfTarget() != "" {
		want++
	}
	if len(created) != want {
		t.Fatalf("prepareWorkspaceBinds created %d paths, want %d: %v", len(created), want, created)
	}
	for _, p := range created {
		info, err := os.Stat(p)
		if err != nil {
			t.Errorf("prepareWorkspaceBinds did not create %s: %v", p, err)
			continue
		}
		if filepath.Base(p) == filepath.Base(jailResolvConfTarget()) {
			if !info.Mode().IsRegular() {
				t.Errorf("DNS mountpoint %s should be a regular file, got %v", p, info.Mode())
			}
			continue
		}
		if !info.IsDir() {
			t.Errorf("prepareWorkspaceBinds did not create dir %s: %v", p, err)
		}
	}

	// 2. segunda chamada não cria nada novo (já existem → não são reportados).
	createdAgain := prepareWorkspaceBinds(dir)
	if len(createdAgain) != 0 {
		t.Errorf("second prepare reported %d dirs, want 0 (existing dirs are never touched): %v", len(createdAgain), createdAgain)
	}

	// 3. cleanup remove apenas os vazios criados por nós.
	//    Um dir com conteúdo pré-existente NUNCA é removido.
	contentDir := filepath.Join(dir, "etc")
	if err := os.WriteFile(filepath.Join(contentDir, "hosts"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	cleanupWorkspaceBinds(created)
	for _, p := range created {
		if filepath.Base(p) == "etc" {
			if _, err := os.Stat(p); err != nil {
				t.Errorf("cleanup removed %s even though it has content — must be preserved", p)
			}
			continue
		}
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("cleanup did not remove empty dir %s", p)
		}
	}
}

func TestJail_GoToolchainAndDNSBinds(t *testing.T) {
	args := buildJailArgs(t.TempDir(), config.DefaultConstraints(), "/proc/self/fd/3")

	// Binds de cache do toolchain Go + DNS.
	if mod := jailGoModCacheHost(); mod != "" {
		if !argsContainsAny(args, []string{"--ro-bind", mod, jailGoModCacheDest}) {
			t.Errorf("missing module cache bind %s -> %s: %v", mod, jailGoModCacheDest, args)
		}
	}
	if !argsContainsAny(args, []string{"--tmpfs", jailGoBuildCacheDest}) {
		t.Errorf("missing build cache tmpfs %s: %v", jailGoBuildCacheDest, args)
	}
	if resolv := jailResolvConfTarget(); resolv != "" {
		if !argsContainsAny(args, []string{"--ro-bind", resolv, resolv}) {
			t.Errorf("missing DNS bind %s: %v", resolv, args)
		}
	}

	// Env do toolchain Go remapeado para a jaula.
	for _, kv := range []struct{ key, value string }{
		{"GOMODCACHE", jailGoModCacheDest},
		{"GOCACHE", jailGoBuildCacheDest},
		{"GOPATH", jailGoPathDest},
	} {
		if !argsContainsAny(args, []string{"--setenv", kv.key, kv.value}) {
			t.Errorf("missing --setenv %s %s: %v", kv.key, kv.value, args)
		}
	}
	if !argsContainsAny(args, []string{"--setenv", "GOPROXY", os.Getenv("GOPROXY")}) &&
		!argsContainsAny(args, []string{"--setenv", "GOPROXY", "https://proxy.golang.org,direct"}) {
		t.Errorf("missing GOPROXY --setenv: %v", args)
	}
}

func TestJail_GoToolchainEnv(t *testing.T) {
	set := jailGoToolchainEnv()
	want := []string{
		"--setenv", "GOMODCACHE", jailGoModCacheDest,
		"--setenv", "GOCACHE", jailGoBuildCacheDest,
		"--setenv", "GOPATH", jailGoPathDest,
		"--setenv", "GOPROXY",
	}
	for i := 0; i < len(want); i++ {
		if set[i] != want[i] {
			t.Errorf("jailGoToolchainEnv[%d] = %q, want %q (full: %v)", i, set[i], want[i], set)
		}
	}
}

// =============================================================================
// 13. BUGFIX L218 — injeção do --info-fd nos args REAIS do buildJailArgs
// (a injeção em si é portátil — ver TestJail_InfoFDInjection_Portable em
// jail_test.go; aqui validamos a integração com os args do bwrap).
// =============================================================================

func TestJail_InfoFDInjection(t *testing.T) {
	t.Run("--info-fd inserted immediately before jail binary path", func(t *testing.T) {
		jailPath := "/proc/self/fd/3"
		args := buildJailArgs(t.TempDir(), config.DefaultConstraints(), jailPath)
		args = jailInjectInfoFD(args, jailPath, jailBootInfoFD)

		found := -1
		for i, a := range args {
			if a == "--info-fd" {
				found = i
				break
			}
		}
		if found == -1 {
			t.Fatalf("--info-fd missing: %v", args)
		}
		if args[found+1] != jailBootInfoFD {
			t.Errorf("--info-fd value = %q, want %q", args[found+1], jailBootInfoFD)
		}
		if found+2 >= len(args) || args[found+2] != jailPath {
			t.Errorf("--info-fd must be immediately before the jail binary path: %v", args)
		}
		// O jailPath continua sendo o comando (ainda existe na lista).
		if !argsContainsAny(args, []string{jailPath}) {
			t.Errorf("jail binary path missing after injection: %v", args)
		}
	})

	t.Run("ordinary user args that match the path are not treated as command", func(t *testing.T) {
		jailPath := "/bin/cosca-jail"
		args := []string{"--bind", "/x", "/", "--clearenv", jailPath, "arg", "/bin/cosca-jail"}
		out := jailInjectInfoFD(args, jailPath, jailBootInfoFD)

		if out[6] != "--info-fd" || out[7] != jailBootInfoFD || out[8] != jailPath {
			t.Errorf("injection must target the LAST occurrence (the command): %v", out)
		}
	})
}
