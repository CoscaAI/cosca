package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestBridgeCommand_HasSubcommands(t *testing.T) {
	root := NewRootCommand()

	var bridgeCmd *cobra.Command
	for _, sub := range root.Commands() {
		if sub.Name() == "bridge" {
			bridgeCmd = sub
			break
		}
	}

	if bridgeCmd == nil {
		t.Fatal("bridge subcommand not found")
	}

	names := make([]string, 0)
	for _, sub := range bridgeCmd.Commands() {
		names = append(names, sub.Name())
	}

	expected := []string{"serve", "connect", "demo"}
	for _, name := range expected {
		found := false
		for _, n := range names {
			if n == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing bridge subcommand: %s (have %v)", name, names)
		}
	}
}

func TestBridgeServeCommand_Flags(t *testing.T) {
	root := NewRootCommand()

	bridgeCmd := findSubcommand(root, "bridge")
	if bridgeCmd == nil {
		t.Fatal("bridge not found")
	}
	serveCmd := findSubcommand(bridgeCmd, "serve")
	if serveCmd == nil {
		t.Fatal("serve not found")
	}

	if serveCmd.Flags().Lookup("port") == nil {
		t.Error("missing --port flag on serve")
	}
}

func TestBridgeConnectCommand_Flags(t *testing.T) {
	root := NewRootCommand()
	bridgeCmd := findSubcommand(root, "bridge")
	connectCmd := findSubcommand(bridgeCmd, "connect")
	if connectCmd == nil {
		t.Fatal("connect not found")
	}

	if connectCmd.Flags().Lookup("url") == nil {
		t.Error("missing --url flag on connect")
	}
	if connectCmd.Flags().Lookup("interval") == nil {
		t.Error("missing --interval flag on connect")
	}
}

func TestBridgeDemoCommand_Flags(t *testing.T) {
	root := NewRootCommand()
	bridgeCmd := findSubcommand(root, "bridge")
	demoCmd := findSubcommand(bridgeCmd, "demo")
	if demoCmd == nil {
		t.Fatal("demo not found")
	}

	for _, f := range []string{"url", "type", "seed", "x", "z"} {
		if demoCmd.Flags().Lookup(f) == nil {
			t.Errorf("missing --%s flag on demo", f)
		}
	}
}

// helper: find a subcommand by name.
func findSubcommand(cmd *cobra.Command, name string) *cobra.Command {
	for _, sub := range cmd.Commands() {
		if sub.Name() == name {
			return sub
		}
	}
	return nil
}
