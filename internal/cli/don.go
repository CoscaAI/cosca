package cli

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/CoscaAI/cosca/internal/kernel"
	"github.com/spf13/cobra"
)

// donAuthPath is where the war phrase bcrypt hash is stored.
// The phrase itself NEVER touches disk — only its hash, and only here.
const donAuthPath = ".cosca/don.phr"

// NewDonCommand creates the `cosca don` command tree — identity verification
// for the Don. Protects against impersonation: high-risk orders must be
// confirmed by the Don's war phrase.
func NewDonCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "don",
		Short: "Don identity protection (war phrase)",
		Long: `Protect the chain of command against impersonation.

The Don's war phrase is a secret known only to him. High-risk orders
(jail, sudo, --force, destructive operations) require confirmation via
this phrase. Only the bcrypt hash is stored in .cosca/don.phr — never the
phrase itself — and every verification attempt is recorded for audit.

Subcommands:
  phrase     Set or verify the Don's war phrase
  status     Show whether Don authentication is armed
  attempts   Show the audit trail of verification attempts
  verify     Verify a phrase against the configured war phrase`,
	}

	cmd.AddCommand(
		NewDonPhraseCommand(),
		NewDonStatusCommand(),
		NewDonAttemptsCommand(),
		NewDonVerifyCommand(),
	)
	return cmd
}

// NewDonPhraseCommand sets (or tests) the Don's war phrase.
func NewDonPhraseCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "phrase",
		Short: "Set the Don's war phrase",
		Long: `Set the Don's war phrase. Only the bcrypt hash is stored.

Usage:
  cosca don phrase "minha frase secreta"   # arm
  cosca don phrase --check "minha frase"   # verify against current hash`,
		RunE: func(cmd *cobra.Command, args []string) error {
			check, _ := cmd.Flags().GetBool("check")
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			if len(args) < 1 {
				return errors.New("usage: cosca don phrase <frase> [--check]")
			}
			phrase := strings.Join(args, " ")

			auth := kernel.NewDonAuth()

			if check {
				return verifyAgainstStored(auth, phrase, cmd, formatter, useJSON)
			}

			// Arm: hash and persist.
			hash, err := hashDonPhrase(phrase)
			if err != nil {
				return fmt.Errorf("cannot hash war phrase: %w", err)
			}
			if err := writeDonPhraseFile(hash); err != nil {
				return fmt.Errorf("cannot store war phrase hash: %w", err)
			}
			// A fresh phrase resets any brute-force lock state.
			_ = os.Remove(donStatePath)

			if useJSON {
				return printJSON(cmd, map[string]any{
					"armed":   true,
					"stored":  donAuthPath,
					"warning": "keep the phrase secret; only its hash was stored",
				})
			}
			formatter.Success("War phrase armed.")
			formatter.KeyValue("Hash stored at", donAuthPath)
			formatter.Warning("The phrase itself was never stored. Keep it secret.")
			return nil
		},
	}
	cmd.Flags().Bool("check", false, "verify the given phrase against the current war phrase")
	return cmd
}

// NewDonStatusCommand shows whether Don authentication is armed.
func NewDonStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show Don authentication status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			hash, err := readDonPhraseFile()
			armed := err == nil && len(hash) > 0

			if useJSON {
				return printJSON(cmd, map[string]any{
					"armed":  armed,
					"stored": donAuthPath,
				})
			}
			if armed {
				formatter.Success("Don authentication is ARMED.")
				formatter.KeyValue("Hash file", donAuthPath)
			} else {
				formatter.Warning("Don authentication is DISARMED.")
				formatter.KeyValue("To arm", `cosca don phrase "sua frase secreta"`)
			}
			return nil
		},
	}
}

// NewDonAttemptsCommand shows the audit trail of verification attempts.
func NewDonAttemptsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "attempts",
		Short: "Show the audit trail of phrase verification attempts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			// In-process attempts are the trail for this process. The CLI
			// reports them from the auth instance used by verify.
			auth := kernel.NewDonAuth()
			hash, err := readDonPhraseFile()
			if err == nil && len(hash) > 0 {
				_ = auth.SetPhraseHash(hash)
			}
			attempts := auth.Attempts()

			if useJSON {
				return printJSON(cmd, attempts)
			}
			if len(attempts) == 0 {
				formatter.Println("No verification attempts recorded in this session.")
				return nil
			}
			formatter.Header("Don Verification Attempts")
			for _, a := range attempts {
				status := "FAIL"
				if a.OK {
					status = "OK"
				}
				formatter.KeyValue(
					fmt.Sprintf("%s %s", a.At.Format("2006-01-02 15:04:05"), status),
					fmt.Sprintf("%s via %s (%s)", a.Challenge, a.Source, a.Reason),
				)
			}
			return nil
		},
	}
}

// NewDonVerifyCommand verifies a phrase against the configured war phrase,
// recording the attempt in the audit trail.
func NewDonVerifyCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "verify <frase>",
		Short: "Verify a phrase against the Don's war phrase",
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			if len(args) < 1 {
				return errors.New("usage: cosca don verify <frase>")
			}
			phrase := strings.Join(args, " ")

			auth := kernel.NewDonAuth()
			return verifyAgainstStored(auth, phrase, cmd, formatter, useJSON)
		},
	}
}

// verifyAgainstStored loads the stored hash and verifies the phrase against
// it, recording the attempt. The brute-force lock state is loaded before and
// persisted after, so the lockout survives process restarts.
func verifyAgainstStored(auth *kernel.DonAuth, phrase string, cmd *cobra.Command, formatter *OutputFormatter, useJSON bool) error {
	hash, err := readDonPhraseFile()
	if err != nil || len(hash) == 0 {
		return errors.New("no war phrase configured — run: cosca don phrase <frase>")
	}
	if err := auth.SetPhraseHash(hash); err != nil {
		return fmt.Errorf("invalid stored hash: %w", err)
	}

	// Restore cross-process brute-force state so repeated CLI invocations
	// still accumulate failures and lock.
	if st, lerr := readDonStateFile(); lerr == nil {
		auth.Restore(st)
	}

	err = auth.Verify(phrase, "cli:verify", "cli")

	// Persist the updated lock state regardless of outcome.
	if serr := writeDonStateFile(auth.Snapshot()); serr != nil {
		// State persistence failure must not silently weaken the lock.
		if err == nil {
			err = fmt.Errorf("cannot persist don lock state: %w", serr)
		}
	}

	if err == nil {
		if useJSON {
			return printJSON(cmd, map[string]any{"ok": true})
		}
		formatter.Success("Verified: phrase matches the Don's war phrase.")
		return nil
	}
	if useJSON {
		return printJSON(cmd, map[string]any{"ok": false, "error": err.Error()})
	}
	return err
}

// hashDonPhrase hashes the war phrase with bcrypt (cost 12, house standard).
func hashDonPhrase(phrase string) ([]byte, error) {
	return kernel.HashDonPhrase(phrase)
}

// writeDonPhraseFile persists the bcrypt hash (base64) with strict perms.
func writeDonPhraseFile(hash []byte) error {
	if err := os.MkdirAll(".cosca", 0o700); err != nil {
		return err
	}
	return os.WriteFile(donAuthPath, []byte(base64.StdEncoding.EncodeToString(hash)), 0o600)
}

// readDonPhraseFile loads the stored bcrypt hash (base64).
func readDonPhraseFile() ([]byte, error) {
	raw, err := os.ReadFile(donAuthPath)
	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
}

// donStatePath stores the brute-force lock state between process runs.
const donStatePath = ".cosca/don.state"

// writeDonStateFile persists the lock state with strict perms.
func writeDonStateFile(st kernel.DonState) error {
	if err := os.MkdirAll(".cosca", 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return os.WriteFile(donStatePath, raw, 0o600)
}

// readDonStateFile loads a previously persisted lock state.
func readDonStateFile() (kernel.DonState, error) {
	var st kernel.DonState
	raw, err := os.ReadFile(donStatePath)
	if err != nil {
		return st, err
	}
	err = json.Unmarshal(raw, &st)
	return st, err
}
