package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/compute"
)

// NewFabricCommand creates the `cosca fabric status` command.
// It is a read-only view of the Compute Fabric: worker pools, hardware
// snapshot and backpressure state, rendered from the Fabric's StatusReport().
func NewFabricCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fabric status",
		Short: "Mostra o status do Compute Fabric (pools, hardware, backpressure)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runFabricStatus(cmd)
		},
	}
	cmd.AddCommand(NewFabricStatusCommand())
	return cmd
}

// NewFabricStatusCommand creates the `cosca fabric status` subcommand,
// aliasing the same status rendering used when the parent runs without args.
func NewFabricStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Mostra o status do Compute Fabric (pools, hardware, backpressure)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runFabricStatus(cmd)
		},
	}
}

// runFabricStatus boots a Compute Fabric with the current config, prints its
// StatusReport and shuts it down. Start/Stop are bounded by the command ctx.
func runFabricStatus(cmd *cobra.Command) error {
	cfg := compute.LoadFabricConfig()
	f := compute.NewFabric(cfg)

	if err := f.Start(cmd.Context()); err != nil {
		return fmt.Errorf("fabric start: %w", err)
	}
	defer func() {
		if err := f.Stop(cmd.Context()); err != nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "fabric stop: %v\n", err)
		}
	}()

	_, err := fmt.Fprint(cmd.OutOrStdout(), f.StatusReport())
	if err != nil {
		return fmt.Errorf("fabric status: %w", err)
	}
	return nil
}
