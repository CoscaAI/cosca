package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/integrity"
)

func main() {
	sign := flag.Bool("sign", false, "Scan embed dir, sign a new block, and append to chain")
	signAuto := flag.Bool("sign-auto", false, "Auto-sign using git anchor (no passphrase required)")
	init := flag.Bool("init", false, "Generate Ed25519 keypair + create genesis block")
	rekey := flag.Bool("rekey", false, "Generate a NEW keypair with a new passphrase (forgot-passphrase recovery)")
	watch := flag.Bool("watch", false, "Real-time integrity watchdog (monitors chain + keys + embed)")
	watchInterval := flag.Int("watch-interval", 30, "Polling interval in seconds for --watch fallback")
	passphraseStdin := flag.Bool("passphrase-stdin", false, "Read kernel passphrase from stdin (for --sign/--init)")
	root := flag.String("root", ".", "Cosca project root directory")

	flag.Parse()

	abs, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "🔴 invalid root: %v\n", err)
		os.Exit(1)
	}

	// Read passphrase from stdin if requested
	var passphrase string
	if *passphraseStdin {
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "🔴 cannot read passphrase from stdin: %v\n", err)
			os.Exit(1)
		}
		passphrase = strings.TrimSpace(input)
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
		if passphrase == "" {
			fmt.Fprintf(os.Stderr, "🔴 --init requires --passphrase-stdin (the kernel passphrase)\n")
			os.Exit(1)
		}
		result, err := integrity.InitChain(abs, passphrase)
		if err != nil {
			fmt.Fprintf(os.Stderr, "🔴 INIT FAILED: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Chain initialized — Block %d signed\n", result.BlockNumber)
		fmt.Printf("   Hash: %s\n", result.BlockHash)
		fmt.Printf("   Files: %d\n", result.FilesSigned)

	case *rekey:
		if passphrase == "" {
			fmt.Fprintf(os.Stderr, "🔴 --rekey requires --passphrase-stdin (the NEW passphrase)\n")
			os.Exit(1)
		}
		if err := integrity.Rekey(abs, passphrase); err != nil {
			fmt.Fprintf(os.Stderr, "🔴 REKEY FAILED: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Novo par de chaves gerado — passphrase atualizada.")
		fmt.Println("   A chave privada antiga foi substituída (irrecuperável, como esperado).")
		fmt.Println("   Próximo passo: git add internal/embed/cosca/keys/kernel_public.key && git commit")
		fmt.Println("   Depois: cosca-check --sign-auto (re-assinar a chain, chave pública mudou)")

	case *signAuto:
		// Passphrase-free git-anchored auto-sign.
		result, err := integrity.SignAuto(abs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "🔴 SIGN-AUTO FAILED: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Block %d git-anchored and appended to chain (no passphrase)\n", result.BlockNumber)
		fmt.Printf("   Hash: %s\n", result.BlockHash)
		fmt.Printf("   Prev: %s\n", result.PrevHash)
		fmt.Printf("   Files: %d\n", result.FilesSigned)

	case *sign:
		if passphrase != "" {
			// Don has the passphrase → full Ed25519 signing.
			result, err := integrity.Sign(abs, passphrase)
			if err == nil {
				fmt.Printf("✅ Block %d signed and appended to chain\n", result.BlockNumber)
				fmt.Printf("   Hash: %s\n", result.BlockHash)
				fmt.Printf("   Prev: %s\n", result.PrevHash)
				fmt.Printf("   Files: %d\n", result.FilesSigned)
				break
			}
			fmt.Fprintf(os.Stderr, "⚠️  Ed25519 signing failed (%v) — falling back to git anchor\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "ℹ️  No passphrase provided — using git-anchored auto-sign (SIGNATURE=GIT-ANCHORED). Use --passphrase-stdin for full Ed25519 signing.\n")
		}
		result, err := integrity.SignAuto(abs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "🔴 SIGN FAILED (git-anchor fallback): %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Block %d git-anchored and appended to chain\n", result.BlockNumber)
		fmt.Printf("   Hash: %s\n", result.BlockHash)
		fmt.Printf("   Prev: %s\n", result.PrevHash)
		fmt.Printf("   Files: %d\n", result.FilesSigned)

	default:
		// Verify mode (default) — no passphrase needed
		info, err := integrity.Check(abs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "🔴 FATAL: %v\n", err)
			fmt.Fprintf(os.Stderr, "   The chain could not be verified.\n")
			fmt.Fprintf(os.Stderr, "   Run: cosca-check --sign --passphrase-stdin\n")
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
			fmt.Println("  Run: cosca-check --sign --passphrase-stdin")
			os.Exit(1)
		}
		fmt.Printf("✅ Chain valid: %d blocks, %d files\n", info.Blocks, info.Files)
	}
}
