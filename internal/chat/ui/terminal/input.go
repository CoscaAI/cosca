package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func openExternalEditor(currentContent string) (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	tmpFile, err := os.CreateTemp("", "cosca-input-*.txt")
	if err != nil {
		return currentContent, err
	}
	defer os.Remove(tmpFile.Name())

	if currentContent != "" {
		if _, err := tmpFile.WriteString(currentContent); err != nil {
			return currentContent, err
		}
	}
	tmpFile.Close()

	cmd := exec.Command(editor, tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return currentContent, err
	}

	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return currentContent, err
	}

	return strings.TrimSpace(string(data)), nil
}

func isBashCommand(input string) bool {
	if len(input) == 0 {
		return false
	}
	return input[0] == '!' && len(input) > 1
}

func stripBashPrefix(input string) string {
	if len(input) > 1 && input[0] == '!' {
		return input[1:]
	}
	return ""
}

func resolveFileReference(input string) []string {
	if !strings.Contains(input, "@") {
		return nil
	}

	var refs []string
	parts := strings.Fields(input)
	for _, part := range parts {
		if strings.HasPrefix(part, "@") && len(part) > 1 {
			refs = append(refs, part[1:])
		}
	}
	return refs
}

func saveHistory(history []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	coscaDir := home + "/.cosca"
	if err := os.MkdirAll(coscaDir, 0755); err != nil {
		return err
	}

	f, err := os.Create(coscaDir + "/terminal_history")
	if err != nil {
		return err
	}
	defer f.Close()

	for _, line := range history {
		if line != "" {
			f.WriteString(line + "\n")
		}
	}
	return nil
}

func loadHistory() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(home + "/.cosca/terminal_history")
	if err != nil {
		// Arquivo inexistente é estado legítimo (primeira execução);
		// qualquer outro erro (permissão, corrupção) NÃO é "sem histórico" —
		// precisa ser propagado, não engolido.
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read terminal history: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	var history []string
	for _, line := range lines {
		if line != "" {
			history = append(history, line)
		}
	}
	return history, nil
}
