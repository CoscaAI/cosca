package main

import (
	"crypto/subtle"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/cli"
	"github.com/CoscaAI/cosca/internal/integrity"
)

func main() {
	sign := flag.Bool("sign", false, "Scan embed dir, sign a new block with the machine-bound Ed25519 key (Don authority — requires the Don gate)")
	signAuto := flag.Bool("sign-auto", false, "Auto-sign using git anchor (immutability witness, no Don authority)")
	init := flag.Bool("init", false, "Generate machine-bound Ed25519 keypair + create genesis block")
	rekey := flag.Bool("rekey", false, "Mint a NEW machine-bound keypair (recovery/migration — no passphrase)")
	push := flag.Bool("push", false, "Push to remote — REQUIRES the Don gate + a SESSION credential (COSCA_GIT_TOKEN or GIT_ASKPASS)")
	watch := flag.Bool("watch", false, "Real-time integrity watchdog (monitors chain + keys + embed)")
	watchInterval := flag.Int("watch-interval", 30, "Polling interval in seconds for --watch fallback")
	root := flag.String("root", ".", "Cosca project root directory")

	flag.Parse()

	abs, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "🔴 invalid root: %v\n", err)
		os.Exit(1)
	}

	switch {
	case *watch:
		// Real-time integrity watchdog
		done := make(chan struct{})
		cfg := integrity.WatchConfig{
			Root:     abs,
			Interval: time.Duration(*watchInterval) * time.Second,
		}

		err := integrity.Watch(cfg, func(r integrity.WatchResult) {
			timestamp := r.Time.Format("15:04:05")
			if !r.Valid {
				fmt.Printf("[%s] %s\n", timestamp, r.Message)
				for _, e := range r.Errors {
					fmt.Printf("[%s]   ❌ %s\n", timestamp, e)
				}
				// Show forensic diffs — what the intruder actually changed
				for _, t := range r.Tampers {
					fmt.Printf("[%s]   📁 %s\n", timestamp, t.Path)
					for _, line := range strings.Split(t.Diff, "\n") {
						fmt.Printf("[%s]      %s\n", timestamp, line)
					}
				}
			} else {
				fmt.Printf("[%s] %s\n", timestamp, r.Message)
			}
		}, done)
		if err != nil {
			fmt.Fprintf(os.Stderr, "🔴 WATCHDOG FAILED: %v\n", err)
			os.Exit(1)
		}

	case *init:
		// Init estabelece a identidade da chain — exige PRESENÇA humana
		// (M3 TTY + M4 consentimento-ao-conteúdo + M5 tempo constante), mas NÃO
		// exige desproteger a chave antiga (caminho de bootstrap, a chave ainda
		// pode não existir).
		if err := cli.VerifyDonPresence(abs, os.Stdin); err != nil {
			fmt.Fprintf(os.Stderr, "🔴 INIT ABORTED — presença do Don negada: %v\n", err)
			os.Exit(1)
		}
		result, err := integrity.InitChain(abs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "🔴 INIT FAILED: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Chain initialized — Block %d signed (Ed25519, machine-bound)\n", result.BlockNumber)
		fmt.Printf("   Hash: %s\n", result.BlockHash)
		fmt.Printf("   Files: %d\n", result.FilesSigned)

	case *rekey:
		// Rekey é o caminho de RECUPERAÇÃO: a chave antiga pode estar ilegível,
		// no formato legado ou falhar ao desproteger — exigir o unprotect aqui
		// bloquearia a própria recuperação. Exige apenas PRESENÇA humana
		// (M3+M4+M5), sem o fator de máquina.
		if err := cli.VerifyDonPresence(abs, os.Stdin); err != nil {
			fmt.Fprintf(os.Stderr, "🔴 REKEY ABORTED — presença do Don negada: %v\n", err)
			os.Exit(1)
		}
		if err := integrity.Rekey(abs); err != nil {
			fmt.Fprintf(os.Stderr, "🔴 REKEY FAILED: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Novo par de chaves gerado (machine-bound via DPAPI — sem passphrase).")
		fmt.Println("   A chave privada antiga foi substituída (irrecuperável, como esperado).")
		fmt.Println("   Próximo passo: git add internal/embed/cosca/keys/kernel_public.key && git commit")
		fmt.Println("   Depois: cosca-check --sign-auto (re-assinar a chain, chave pública mudou)")

	case *signAuto:
		// Passphrase-free git-anchored auto-sign (immutability witness, NOT Don authority).
		result, err := integrity.SignAuto(abs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "🔴 SIGN-AUTO FAILED: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Block %d git-anchored (SIGNATURE=GIT-ANCHORED — testemunho de imutabilidade, sem autoridade do Don)\n", result.BlockNumber)
		fmt.Printf("   Hash: %s\n", result.BlockHash)
		fmt.Printf("   Prev: %s\n", result.PrevHash)
		fmt.Printf("   Files: %d\n", result.FilesSigned)

	case *sign:
		// Assinatura MANUAL exige o portão do Don (máquina + consentimento-ao-
		// conteúdo). Sem o nonce real, NEGA — nunca cai em git-anchor silencioso.
		// O digest consentido é retornado para a checagem anti-TOCTOU abaixo.
		consented, err := cli.VerifyDonIdentity(abs, os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "🔴 SIGN ABORTED — portão do Don negado: %v\n", err)
			os.Exit(1)
		}
		result, err := integrity.Sign(abs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "🔴 SIGN FAILED: %v\n", err)
			os.Exit(1)
		}
		// TOCTOU: o conteúdo assinado ainda deve ser EXATAMENTE o que o Don
		// consentiu. Sign() só apenda um bloco FORA do embed, então uma
		// divergência aqui significa que algo alterou o embed entre os dois.
		if err := verifyDonSignDigest(abs, consented); err != nil {
			fmt.Fprintf(os.Stderr, "🔴 SIGN ABORTED — %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Block %d signed (SIGNATURE=Ed25519 — autoridade do Don)\n", result.BlockNumber)
		fmt.Printf("   Hash: %s\n", result.BlockHash)
		fmt.Printf("   Prev: %s\n", result.PrevHash)
		fmt.Printf("   Files: %d\n", result.FilesSigned)

	case *push:
		// Push NUNCA é automático. Exige: (a) portão do Don (máquina + nonce de
		// consentimento-ao-conteúdo); (b) credencial de SESSÃO que não reside na
		// máquina (COSCA_GIT_TOKEN efêmero, ou GIT_ASKPASS → helper não
		// persistido). Sem nonce ou sem credencial → ABORTA.
		if _, err := cli.VerifyDonIdentity(abs, os.Stdin); err != nil {
			fmt.Fprintf(os.Stderr, "🔴 PUSH ABORTED — portão do Don negado: %v\n", err)
			os.Exit(1)
		}
		if err := pushRemote(abs); err != nil {
			fmt.Fprintf(os.Stderr, "🔴 PUSH ABORTED: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Push realizado com credencial de sessão (não persistida).")

	default:
		// Verify mode (default) — no gate required, no credential needed.
		info, err := integrity.Check(abs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "🔴 FATAL: %v\n", err)
			fmt.Fprintf(os.Stderr, "   The chain could not be verified.\n")
			fmt.Fprintf(os.Stderr, "   Run: cosca-check --sign (o Don re-assina após mudança autorizada)\n")
			os.Exit(1)
		}
		if !info.Valid {
			fmt.Println("\n╔══════════════════════════════════════════╗")
			fmt.Println("║  🔴 FAMILY CHAIN BREACH DETECTED         ║")
			fmt.Println("║  Engine startup blocked for safety       ║")
			fmt.Println("╚══════════════════════════════════════════╝")
			for _, e := range info.Errors {
				fmt.Printf("  ❌ %s\n", e)
			}
			fmt.Println("\n  Only the kernel can re-sign after authorized changes.")
			fmt.Println("  Run: cosca-check --sign (portão do Don: máquina + nonce)")
			os.Exit(1)
		}
		fmt.Printf("✅ Chain valid: %d blocks, %d files\n", info.Blocks, info.Files)
	}
}

// verifyDonSignDigest confirma que o conteúdo assinado ainda é o MESMO que o Don
// consentiu (checagem anti-TOCTOU). integrity.Sign apenas apenda um bloco FORA
// do diretório de embed, então uma divergência aqui significa que algo externo
// alterou o embed entre o consentimento e a assinatura — e o bloco não deve ser
// aceito. A comparação é em TEMPO CONSTANTE (M5), como o token de presença.
func verifyDonSignDigest(root, consented string) error {
	current, err := cli.DonConsentDigest(root)
	if err != nil {
		return fmt.Errorf("não foi possível re-verificar o conteúdo assinado: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(current), []byte(consented)) != 1 {
		return fmt.Errorf("conteúdo mudou entre consentimento e assinatura")
	}
	return nil
}

// pushRemote publica para o remoto SEM persistir credencial na máquina. A
// credencial deve vir da SESSÃO: COSCA_GIT_TOKEN (env efêmero) ou um
// GIT_ASKPASS que o Don apontou para um helper não persistido. Sem credencial
// de sessão → aborta (fail-closed). Nunca empurra por padrão.
func pushRemote(root string) error {
	if tok := strings.TrimSpace(os.Getenv("COSCA_GIT_TOKEN")); tok != "" {
		return pushWithSessionToken(root, tok)
	}
	if os.Getenv("GIT_ASKPASS") != "" {
		return runGitPush(root, nil)
	}
	return fmt.Errorf("credencial de push ausente — o Don deve fornecer COSCA_GIT_TOKEN (sessão efêmera) ou apontar GIT_ASKPASS para um helper não persistido")
}

// pushWithSessionToken injeta COSCA_GIT_TOKEN via um helper GIT_ASKPASS
// efêmero (sem token no arquivo — o token é passado pelo ambiente do
// processo). O helper é removido ao final; nada é gravado na config do git.
func pushWithSessionToken(root, token string) error {
	helper, err := sessionAskPass()
	if err != nil {
		return fmt.Errorf("criar helper de askpass: %w", err)
	}
	defer os.Remove(helper)

	extra := []string{
		"COSCA_SESSION_TOKEN=" + token,
		"GIT_ASKPASS=" + helper,
		"GIT_TERMINAL_PROMPT=0",
	}
	return runGitPush(root, extra)
}

// runGitPush executa `git -C root push` propagando o env base do processo mais
// as variáveis extras (credencial de sessão). Retorna erro se o push falhar.
func runGitPush(root string, extraEnv []string) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git não encontrado no PATH (push requer git)")
	}
	cmd := exec.Command("git", "-C", root, "push")
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat")
	cmd.Env = append(cmd.Env, extraEnv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("git push: %s", msg)
		}
		return fmt.Errorf("git push: %w", err)
	}
	if strings.TrimSpace(string(out)) != "" {
		fmt.Print(string(out))
	}
	return nil
}

// sessionAskPass cria um script temporário de GIT_ASKPASS que responde a
// prompts de usuário com um nome fixo e a prompts de senha/token com
// COSCA_SESSION_TOKEN (fornecido via ambiente). O token NUNCA é gravado no
// arquivo — apenas o ambiente do subprocesso git o carrega.
func sessionAskPass() (string, error) {
	dir := os.TempDir()
	var path, script string

	if runtime.GOOS == "windows" {
		f, err := os.CreateTemp(dir, "cosca-askpass-*.bat")
		if err != nil {
			return "", err
		}
		path = f.Name()
		f.Close()
		script = "@echo off\r\n" +
			"set ARGS=%~1\r\n" +
			"echo %ARGS% | findstr /i \"username login\" >nul\r\n" +
			"if %errorlevel%==0 (echo opendev_user) else (echo %COSCA_SESSION_TOKEN%)\r\n"
	} else {
		f, err := os.CreateTemp(dir, "cosca-askpass-*.sh")
		if err != nil {
			return "", err
		}
		path = f.Name()
		f.Close()
		script = "#!/bin/sh\n" +
			"case \"$1\" in *[Uu]sername*|*[Ll]ogin*) echo \"opendev_user\" ;; *) echo \"$COSCA_SESSION_TOKEN\" ;; esac\n"
	}

	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		return "", err
	}
	return path, nil
}
