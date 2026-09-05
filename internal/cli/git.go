package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/gitmgr"
)

// NewGitCommand expõe o COSCA Git Manager v0 via CLI.
// internal/gitmgr é a API segura; aqui é a porta de operação do Don.
//
// Regras (professor):
//   READ  (sempre permitido): status, log, diff, branches, provenance
//   WRITE (não-destrutivo, requer --yes): branch create, commit, pull, push
//   DESTRUCTIVE (fail-closed): reset, branch delete, force push — exigem gate
//
// O pacote gitmgr NUNCA executa destrutivo sem autorização; o CLI traduz o
// --yes em um DestructiveGate explícito.
func NewGitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "git",
		Short: "COSCA Git Manager (colaboração e proveniência)",
		Long: `COSCA Git Manager v0 — operar o repositório Git do projeto para
colaboração e proveniência de datasets/campanhas/LoRAs.

Leitura (sempre permitida): status, log, diff, branches, provenance.
Escrita (requer --yes): branch create, commit, pull, push.
Destrutivas (fail-closed): reset, branch delete, force push — exigem --yes
explícito e o pacote gitmgr NUNCA executa sem autorização.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newGitStatusCommand())
	cmd.AddCommand(newGitLogCommand())
	cmd.AddCommand(newGitDiffCommand())
	cmd.AddCommand(newGitBranchesCommand())
	cmd.AddCommand(newGitProvenanceCommand())
	cmd.AddCommand(newGitBranchCreateCommand())
	cmd.AddCommand(newGitCommitCommand())
	return cmd
}

// openRepo abre o repositório Git do CWD (o projeto Cosca).
func openRepo() (*gitmgr.Manager, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("git: getcwd: %w", err)
	}
	m, err := gitmgr.Open(wd)
	if err != nil {
		return nil, fmt.Errorf("git: %w", err)
	}
	return m, nil
}

// ─── READ (sempre permitido) ────────────────────────────────────────────────

func newGitStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Estado do working tree (arquivos modificados)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			f := GetFormatter(cmd)
			m, err := openRepo()
			if err != nil {
				return err
			}
			st, err := m.Status(context.Background())
			if err != nil {
				return err
			}
			if strings.TrimSpace(st) == "" {
				f.Success("working tree limpo")
				return nil
			}
			f.Print(st)
			return nil
		},
	}
}

func newGitLogCommand() *cobra.Command {
	var n int
	cmd := &cobra.Command{
		Use:   "log",
		Short: "Histórico de commits",
		RunE: func(cmd *cobra.Command, _ []string) error {
			f := GetFormatter(cmd)
			m, err := openRepo()
			if err != nil {
				return err
			}
			commits, err := m.Log(context.Background(), n)
			if err != nil {
				return err
			}
			for _, c := range commits {
				f.Printf("%s  %-40s %s\n", c.Hash, truncateStr(c.Message, 40), c.Author)
			}
			return nil
		},
	}
	cmd.Flags().IntVarP(&n, "count", "n", 10, "número de commits")
	return cmd
}

func newGitDiffCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "diff",
		Short: "Resumo das mudanças no working tree",
		RunE: func(cmd *cobra.Command, _ []string) error {
			f := GetFormatter(cmd)
			m, err := openRepo()
			if err != nil {
				return err
			}
			d, err := m.Diff(context.Background())
			if err != nil {
				return err
			}
			if strings.TrimSpace(d) == "" {
				f.Success("sem mudanças")
				return nil
			}
			f.Print(d)
			return nil
		},
	}
}

func newGitBranchesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "branches",
		Short: "Lista branches e o branch atual",
		RunE: func(cmd *cobra.Command, _ []string) error {
			f := GetFormatter(cmd)
			m, err := openRepo()
			if err != nil {
				return err
			}
			cur, _ := m.CurrentBranch(context.Background())
			branches, err := m.Branches(context.Background())
			if err != nil {
				return err
			}
			for _, b := range branches {
				mark := "  "
				if b == cur {
					mark = "* "
				}
				f.Printf("%s%s\n", mark, b)
			}
			return nil
		},
	}
}

func newGitProvenanceCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "provenance",
		Short: "Mostra/registra proveniência (dataset → campanha → LoRA → avaliação)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			f := GetFormatter(cmd)
			f.Printf("Proveniência é mantida em .cosca/provenance/. Use git provenance --write com os campos para registrar.\n")
			m, err := openRepo()
			if err != nil {
				return err
			}
			_ = m
			return nil
		},
	}
}

// ─── WRITE (não-destrutivo, requer --yes) ───────────────────────────────────

func newGitBranchCreateCommand() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "branch-create <name>",
		Short: "Cria um branch (seguro, requer --yes)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f := GetFormatter(cmd)
			if !yes {
				return fmt.Errorf("git: operação de escrita exige confirmação (--yes)")
			}
			m, err := openRepo()
			if err != nil {
				return err
			}
			if err := m.CreateBranch(context.Background(), args[0]); err != nil {
				return err
			}
			f.Success("branch criado: " + args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "confirmar a operação de escrita")
	return cmd
}

func newGitCommitCommand() *cobra.Command {
	var yes bool
	var author, email string
	cmd := &cobra.Command{
		Use:   "commit <message>",
		Short: "Commit das mudanças (requer --yes)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f := GetFormatter(cmd)
			if !yes {
				return fmt.Errorf("git: commit exige confirmação (--yes)")
			}
			if author == "" {
				author = "cosca"
			}
			if email == "" {
				email = "cosca@localhost"
			}
			m, err := openRepo()
			if err != nil {
				return err
			}
			h, err := m.Commit(context.Background(), gitmgr.CommitArgs{
				Message: args[0], Author: author, Email: email,
			})
			if err != nil {
				return err
			}
			f.Success("commit criado: " + h)
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "confirmar o commit")
	cmd.Flags().StringVar(&author, "author", "", "autor do commit (default: cosca)")
	cmd.Flags().StringVar(&email, "email", "", "email do autor")
	return cmd
}
