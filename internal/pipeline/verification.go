package pipeline

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type VerificationRunner struct {
	workDir string
}

func NewVerificationRunner(workDir string) *VerificationRunner {
	return &VerificationRunner{workDir: workDir}
}

func (v *VerificationRunner) detectProjectType() (string, error) {
	if _, err := os.Stat(filepath.Join(v.workDir, "go.mod")); err == nil {
		return "go", nil
	}
	if _, err := os.Stat(filepath.Join(v.workDir, "package.json")); err == nil {
		return "node", nil
	}
	if _, err := os.Stat(filepath.Join(v.workDir, "requirements.txt")); err == nil {
		return "python", nil
	}
	if _, err := os.Stat(filepath.Join(v.workDir, "pyproject.toml")); err == nil {
		return "python", nil
	}
	if _, err := os.Stat(filepath.Join(v.workDir, "setup.py")); err == nil {
		return "python", nil
	}
	entries, err := os.ReadDir(v.workDir)
	if err != nil {
		// Falha de leitura NÃO é "projeto desconhecido": verificação honesta
		// precisa saber que não conseguiu inspecionar o diretório.
		return "", fmt.Errorf("detect project type: %w", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".py") {
			return "python", nil
		}
		if strings.HasSuffix(e.Name(), ".go") {
			return "go", nil
		}
		if strings.HasSuffix(e.Name(), ".js") || strings.HasSuffix(e.Name(), ".ts") {
			return "node", nil
		}
	}
	return "unknown", nil
}

func (v *VerificationRunner) BuildVerify() (bool, string, error) {
	projType, err := v.detectProjectType()
	if err != nil {
		return false, "", err
	}
	var cmd *exec.Cmd
	switch projType {
	case "go":
		pkgs := v.goPackages()
		cmd = exec.Command("go", append([]string{"build"}, pkgs...)...)
		cmd.Dir = v.workDir
	case "python":
		cmd = exec.Command("python3", "-c", "import py_compile; py_compile.compile('.')")
		cmd.Dir = v.workDir
	case "node":
		cmd = exec.Command("npm", "run", "build")
		cmd.Dir = v.workDir
	default:
		return true, "no build command for project type", nil
	}
	out, err := cmd.CombinedOutput()
	output := string(out)
	return err == nil, output, nil
}

func (v *VerificationRunner) TestVerify() (bool, string, error) {
	projType, err := v.detectProjectType()
	if err != nil {
		return false, "", err
	}
	var cmd *exec.Cmd
	switch projType {
	case "go":
		pkgs := v.goPackages()
		cmd = exec.Command("go", append([]string{"test"}, append(pkgs, "-count=1", "-timeout=120s")...)...)
		cmd.Dir = v.workDir
	case "python":
		cmd = exec.Command("python3", "-m", "pytest")
		cmd.Dir = v.workDir
	case "node":
		cmd = exec.Command("npm", "test")
		cmd.Dir = v.workDir
	default:
		return true, "no test command for project type", nil
	}
	out, err := cmd.CombinedOutput()
	output := string(out)
	return err == nil, output, nil
}

func (v *VerificationRunner) goPackages() []string {
	entries, err := os.ReadDir(v.workDir)
	if err != nil {
		return []string{"./..."}
	}
	systemDirs := map[string]bool{
		"dev": true, "etc": true, "lib": true, "lib64": true,
		"proc": true, "tmp": true, "usr": true, "var": true,
		"sys": true, "run": true, "bin": true, "sbin": true,
		"boot": true, "opt": true, "srv": true, "mnt": true,
		"home": true, "root": true, ".cache": true, ".config": true,
	}
	for _, e := range entries {
		if e.IsDir() && !systemDirs[e.Name()] && !strings.HasPrefix(e.Name(), ".") {
			goFile, err := os.ReadDir(filepath.Join(v.workDir, e.Name()))
			if err != nil {
				continue // subdir ilegível: não é base para decidir; fallback abaixo cobre
			}
			for _, f := range goFile {
				if strings.HasSuffix(f.Name(), ".go") {
					return []string{"./..."}
				}
			}
		}
	}
	return []string{"."}
}
