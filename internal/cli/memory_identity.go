package cli

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/CoscaAI/cosca/internal/integrity"
	"github.com/CoscaAI/cosca/internal/kernel"
)

// readSecret lê um segredo do operador sem eco no terminal (anti shoulder-
// surfing). Quando `useTerminal` é falso (testes), cai no bufio.Reader
// tradicional — o texto digitado aparece, comportamento histórico.
func readSecret(reader *bufio.Reader, prompt string, useTerminal bool) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	if useTerminal {
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		return strings.TrimSpace(string(b)), err
	}
	s, _ := reader.ReadString('\n')
	return strings.TrimSpace(s), nil
}

// verifyDonIdentity executa a prova de identidade em 3 fatores antes de uma
// operação privilegiada na memória (ordem do Don: "quero as 3"):
//
//  1. Passphrase (2FA) — decripta a chave Ed25519 do kernel. "Algo que você sabe".
//  2. War phrase — verifica contra o bcrypt do Don. Segundo segredo independente.
//  3. Presença — nonce aleatório digitado de volta ao vivo. "Algo que você é/faz".
//
// A passphrase pode vir do env COSCA_KERNEL_PASSPHRASE; os demais fatores são
// lidos do `in` (stdin interativo). O nonce prova que há um humano presente no
// terminal — não um processo roubado com as chaves na mão.
func verifyDonIdentity(root string, in io.Reader) error {
	reader := bufio.NewReader(in)

	// Detecção do terminal real: só escondemos a digitação quando `in` é o
	// próprio os.Stdin (produção). Testes usam readers sintéticos e mantêm
	// o comportamento visível.
	useTerminal := false
	if f, ok := in.(*os.File); ok && f == os.Stdin {
		useTerminal = true
	}

	// Fator 1 — passphrase (obrigatória). Sem ela, nega: 2FA é inegociável.
	passphrase := strings.TrimSpace(os.Getenv("COSCA_KERNEL_PASSPHRASE"))
	if passphrase == "" {
		passphrase, _ = readSecret(reader, "Passphrase do kernel: ", useTerminal)
	}
	if passphrase == "" {
		return fmt.Errorf("fator 1 (passphrase) ausente — 2FA é obrigatório")
	}
	if err := integrity.VerifyKernelIdentity(root, passphrase); err != nil {
		return fmt.Errorf("fator 1 (passphrase) falhou: %w", err)
	}

	// Fator 2 — war phrase (bcrypt). Se não armada, exige armar antes.
	hash, err := readDonPhraseFile()
	if err != nil || len(hash) == 0 {
		return fmt.Errorf("fator 2 (war phrase) indisponível — arme com: cosca don phrase <frase>")
	}
	phrase, _ := readSecret(reader, "Frase de guerra: ", useTerminal)

	auth := kernel.NewDonAuth()
	if err := auth.SetPhraseHash(hash); err != nil {
		return fmt.Errorf("fator 2 (war phrase) inválido: %w", err)
	}
	if st, lerr := readDonStateFile(); lerr == nil {
		auth.Restore(st)
	}
	verr := auth.Verify(phrase, "memory:register", "cli")
	_ = writeDonStateFile(auth.Snapshot())
	if verr != nil {
		return fmt.Errorf("fator 2 (war phrase) falhou: %w", verr)
	}

	// Fator 3 — presença (nonce digitado de volta ao vivo).
	nonceBytes := make([]byte, 4)
	if _, err := rand.Read(nonceBytes); err != nil {
		return fmt.Errorf("fator 3 (presença) falhou ao gerar nonce: %w", err)
	}
	nonce := hex.EncodeToString(nonceBytes)
	if useTerminal {
		typed, _ := readSecret(reader, fmt.Sprintf("Nonce %s — digite para confirmar presença: ", nonce), true)
		if typed != nonce {
			return fmt.Errorf("nonce não confere")
		}
		return nil
	}
	if err := confirmPresence(reader, nonce); err != nil {
		return fmt.Errorf("fator 3 (presença) falhou: %w", err)
	}

	return nil
}

// confirmPresence prints the nonce and requires the operator to type it back,
// proving a human is present at the terminal. Extractable for testing.
func confirmPresence(reader *bufio.Reader, nonce string) error {
	fmt.Fprintf(os.Stderr, "Nonce %s — digite para confirmar presença: ", nonce)
	typed, _ := reader.ReadString('\n')
	typed = strings.TrimSpace(typed)
	if typed != nonce {
		return fmt.Errorf("nonce não confere")
	}
	return nil
}
