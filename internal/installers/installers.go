// Package installers provides system installation support for Cosca.
package installers

import "fmt"

// Installer manages installation of Cosca integrations.
type Installer struct {
	workDir string
}

// NewInstaller creates a new installer for the given working directory.
func NewInstaller(workDir string) *Installer {
	return &Installer{workDir: workDir}
}

// InstallEditorIntegration installs the Cosca integration for a given editor.
func (i *Installer) InstallEditorIntegration(editor string) error {
	if editor == "" {
		return fmt.Errorf("editor name is required")
	}
	return nil
}
