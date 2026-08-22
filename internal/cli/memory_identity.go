package cli

import (
	"bufio"
	"crypto/subtle"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/term"

	"github.com/CoscaAI/cosca/internal/integrity"
)

// VerifyDonIdentity is the EXPORTED entry-point of the FULL Don identity gate
// (machine factor + human presence/consent), used by CLI tools (e.g.
// cosca-check) that must authenticate BEFORE a privileged signing/publishing
// operation. It returns the consented content digest so the caller can detect a
// TOCTOU divergence between the content the Don consented to and the content
// actually signed afterwards.
func VerifyDonIdentity(root string, in io.Reader) (string, error) {
	return verifyDonIdentityDigest(root, in)
}

// VerifyDonPresence is the EXPORTED presence-only gate: TTY fail-closed (M3) +
// consent-to-content (M4) + constant-time token compare (M5). It does NOT call
// integrity.VerifyKernelIdentity (no machine factor). It backs --init/--rekey,
// the bootstrap / recovery path where the OLD key may be unreadable or fail to
// unprotect — requiring it here would deadlock the recovery flow.
func VerifyDonPresence(root string, in io.Reader) error {
	_, err := presenceFactor(root, in)
	return err
}

// verifyDonIdentity is the in-package entry-point retained for callers inside
// the cli package (cosca memory register): machine factor + presence factor.
func verifyDonIdentity(root string, in io.Reader) error {
	_, err := verifyDonIdentityDigest(root, in)
	return err
}

// verifyDonIdentityDigest runs the FULL Don gate (machine + human presence) and
// returns the consented content digest.
//
//	A prova de identidade do Don tem DOIS fatores independentes antes de uma
//	operaçao privilegiada:
//
//	Fator 1 — MÁQUINA (integrity.VerifyKernelIdentity): prova que a chave
//	  privada Ed25519 do kernel é legível E decriptável (DPAPI) — o mesmo
//	  usuário na mesma máquina. Nenhum segredo a lembrar: o segredo é o
//	  vínculo máquina+usuário.
//	Fator 2 — PRESENÇA / CONSENTIMENTO-ao-conteúdo: o Don digita um token
//	  derivado do conteúdo que será assinado (nº de arquivos + resumo do
//	  bloco). Se o conteúdo mudar, o token muda — o Don aprova aquele bloco
//	  EXATO. A comparação é em TEMPO CONSTANTE.
//
// O parâmetro `in` é mantido apenas por compatibilidade de assinatura (os
// callers passam os.Stdin); a leitura real é feita do os.Stdin (TTY), em
// conformidade com o M3.
func verifyDonIdentityDigest(root string, in io.Reader) (string, error) {
	// M3 — fail-closed: presença exige terminal interativo real. Nunca caímos
	// em fallback legível com stdin piped.
	if err := m3RequireTTY(in); err != nil {
		return "", err
	}

	// Fator 1 — máquina: prova posse + decriptação da chave via DPAPI.
	if err := integrity.VerifyKernelIdentity(root); err != nil {
		return "", fmt.Errorf("fator 1 (máquina) falhou: %w", err)
	}

	// Fator 2 — presença + consentimento ao conteúdo.
	return consentFactor(root)
}

// presenceFactor é o portão humano SEM o fator de máquina — usado por
// --init/--rekey (recovery). Mantém M3 (TTY fail-closed) + M4 (consentimento-ao-
// conteúdo) + M5 (tempo constante). Retorna o digest consentido.
func presenceFactor(root string, in io.Reader) (string, error) {
	if err := m3RequireTTY(in); err != nil {
		return "", err
	}
	return consentFactor(root)
}

// m3RequireTTY é o guarda M3: presença exige terminal interativo REAL. Se
// os.Stdin não é um TTY, NEGA — nunca cai no caminho legível com stdin piped.
// A leitura é sempre do os.Stdin autêntico (o parâmetro `in` é reconhecido, mas
// ignorado por compatibilidade de assinatura).
func m3RequireTTY(_ io.Reader) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("presença exige terminal interativo real (stdin não é um TTY)")
	}
	return nil
}

// consentFactor é o fator M4+M5 do gate: deriva o challenge (nº de arquivos +
// resumo + token) a partir do conteúdo e exige que o Don digite o token ao
// vivo, comparando em TEMPO CONSTANTE. Retorna o digest consentido.
func consentFactor(root string) (string, error) {
	files, digest, token, err := consentChallenge(root)
	if err != nil {
		return "", fmt.Errorf("consentimento falhou ao derivar o challenge: %w", err)
	}

	fmt.Fprintf(os.Stderr, "\n📦 Consentimento-ao-conteúdo — o que será assinado:\n")
	fmt.Fprintf(os.Stderr, "   arquivos:   %d\n", files)
	fmt.Fprintf(os.Stderr, "   resumo:     %s…\n", digest[:16])
	fmt.Fprintf(os.Stderr, "   token:      %s\n", token)

	reader := bufio.NewReader(os.Stdin)
	if err := confirmPresence(reader, token); err != nil {
		return "", fmt.Errorf("presença/consentimento falhou: %w", err)
	}

	return digest, nil
}

// DonConsentDigest recomputa (sem nenhum prompt) o digest do conteúdo que o Don
// consentiu, para a checagem anti-TOCTOU: o conteúdo aprovado no consentimento
// deve continuar idêntico ao conteúdo assinado em seguida. Sign() apenas apenda
// um bloco FORA do diretório de embed, então uma divergência aqui só ocorre se
// algo alterou o embed entre o consentimento e a assinatura.
func DonConsentDigest(root string) (string, error) {
	_, digest, _, err := consentChallenge(root)
	if err != nil {
		return "", err
	}
	return digest, nil
}

// consentChallenge deriva, de forma determinística, um resumo do conteúdo que
// será assinado (o diretório de embed do kernel) e um token curto de
// consentimento. O digest e o token mudam sempre que o conteúdo muda, de modo
// que o Don aprova EXATAMENTE aquele bloco. O token não é um segredo — é um
// código de aprovação exibido e digitado ao vivo; a prova de presença vem do
// humano no TTY.
func consentChallenge(coscaRoot string) (files int, digestHex, token string, err error) {
	embedDir := filepath.Join(coscaRoot, "internal", "embed", "cosca")

	type entry struct {
		path string
		hash string
		size int64
	}
	var entries []entry

	err = filepath.Walk(embedDir, func(path string, info os.FileInfo, wErr error) error {
		if wErr != nil {
			return wErr
		}
		if info.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(coscaRoot, path)
		if relErr != nil {
			return relErr
		}
		h, hErr := integrity.HashFile(integrity.HashBLAKE3, path)
		if hErr != nil {
			return hErr
		}
		entries = append(entries, entry{path: rel, hash: h, size: info.Size()})
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			// Embed ainda não materializado — consentimento sobre "0 arquivos".
			entries = nil
		} else {
			return 0, "", "", fmt.Errorf("scan embed: %w", err)
		}
	}

	// Ordenação determinística para um digest estável entre execuções.
	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })

	var canon strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&canon, "%s:%s:%d\n", e.path, e.hash, e.size)
	}
	digestHex = integrity.HashBytes(integrity.HashBLAKE3, []byte(canon.String()))

	// Token de aprovação derivado do conteúdo (determinístico, muda com ele).
	// 16 hex (64 bits) — entropia suficiente para um código de consentimento
	// exibido e digitado ao vivo; 8 hex ficava curto demais para a revisão.
	token = integrity.HashBytes(integrity.HashBLAKE3, []byte("cosca-consent:"+digestHex))
	if len(token) > 16 {
		token = token[:16]
	}

	return len(entries), digestHex, token, nil
}

// confirmPresence exige que o operador digite de volta o challenge fornecido,
// provando que há um humano presente no terminal e consentindo com o conteúdo
// do qual o challenge foi derivado. A comparação é feita em TEMPO CONSTANTE
// (crypto/subtle) — nunca `typed != challenge`.
func confirmPresence(reader *bufio.Reader, challenge string) error {
	fmt.Fprintf(os.Stderr, "Digite o token para aprovar ESTE conteúdo: ")
	typed, _ := reader.ReadString('\n')
	typed = strings.TrimSpace(typed)
	fmt.Fprintln(os.Stderr)

	if subtle.ConstantTimeCompare([]byte(typed), []byte(challenge)) != 1 {
		return fmt.Errorf("token de consentimento não confere")
	}
	return nil
}
