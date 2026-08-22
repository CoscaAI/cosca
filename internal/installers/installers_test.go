package installers

import (
	"testing"
)

func TestNewInstaller(t *testing.T) {
	t.Parallel()

	inst := NewInstaller("/work/dir")
	if inst == nil {
		t.Fatal("NewInstaller returned nil")
	}
	if inst.workDir != "/work/dir" {
		t.Errorf("workDir = %q", inst.workDir)
	}
}

func TestInstallEditorIntegration(t *testing.T) {
	t.Parallel()

	inst := NewInstaller("/tmp")
	err := inst.InstallEditorIntegration("vim")
	if err != nil {
		t.Fatalf("InstallEditorIntegration error: %v", err)
	}
}

func TestInstallEditorIntegrationEmpty(t *testing.T) {
	t.Parallel()

	inst := NewInstaller("/tmp")
	err := inst.InstallEditorIntegration("")
	if err == nil {
		t.Error("expected error for empty editor name")
	}
}

func TestNewInstallerEmptyDir(t *testing.T) {
	t.Parallel()

	inst := NewInstaller("")
	if inst.workDir != "" {
		t.Errorf("workDir = %q", inst.workDir)
	}
}
