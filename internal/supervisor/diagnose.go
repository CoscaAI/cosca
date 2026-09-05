package supervisor

import (
	"fmt"
	"strings"
)

// DiagnoseResult is the outcome of an automated diagnosis.
type DiagnoseResult struct {
	Problem    string          `json:"problem"`
	Cause      string          `json:"cause"`
	IsFatal    bool            `json:"is_fatal"`
	Adjustment *Adjustment     `json:"adjustment,omitempty"`
	Level      EscalationLevel `json:"level"`
}

// Diagnose analyzes a kernel error or anomaly and decides what to do.
func Diagnose(status *KernelStatus, resp *InterruptResponse) DiagnoseResult {
	// Case 1: No response at all → kernel is dead.
	if resp == nil {
		return DiagnoseResult{
			Problem: fmt.Sprintf("kernel %q não responde — último heartbeat: %v atrás",
				status.KernelID, status.LastHeartbeat),
			Cause:   "processo morto ou travado sem responder",
			IsFatal: true,
			Level:   LevelRestart,
		}
	}

	// Case 2: Response has error → analyze the error.
	if resp.Error != "" {
		return diagnoseError(status, resp)
	}

	// Case 3: Tests failed → analyze test failures.
	if resp.TestsRun > resp.TestsPassed {
		return diagnoseTestFailure(status, resp)
	}

	// Case 4: Stopped cleanly → can resume.
	if resp.Status == "STOPPED" {
		return DiagnoseResult{
			Problem: fmt.Sprintf("kernel %q interrompido com checkpoint %s",
				status.KernelID, resp.CheckpointID),
			Cause: "interrupção externa",
			Level: LevelAutoCorrect,
		}
	}

	return DiagnoseResult{
		Problem: "estado desconhecido",
		Cause:   "sem diagnóstico específico",
		Level:   LevelRestart,
	}
}

// diagnoseError analyzes the error string from a kernel response.
func diagnoseError(status *KernelStatus, resp *InterruptResponse) DiagnoseResult {
	errLower := strings.ToLower(resp.Error)

	// Compilation errors.
	if strings.Contains(errLower, "syntax error") || strings.Contains(errLower, "undefined") ||
		strings.Contains(errLower, "cannot use") || strings.Contains(errLower, "imported and not used") {
		return DiagnoseResult{
			Problem: "erro de compilação detectado",
			Cause:   resp.Error,
			Level:   LevelAutoCorrect,
			Adjustment: &Adjustment{
				Description: "corrigir erro de sintaxe e recompilar",
				Commands:    []string{"go build ./..."},
			},
		}
	}

	// Type errors.
	if strings.Contains(errLower, "type mismatch") || strings.Contains(errLower, "cannot assign") {
		return DiagnoseResult{
			Problem: "erro de tipo detectado",
			Cause:   resp.Error,
			Level:   LevelRollback,
		}
	}

	// Import/dependency errors.
	if strings.Contains(errLower, "no required module") || strings.Contains(errLower, "missing go.sum") {
		return DiagnoseResult{
			Problem: "dependência ausente",
			Cause:   resp.Error,
			Level:   LevelAutoCorrect,
			Adjustment: &Adjustment{
				Description: "executar go mod tidy para resolver dependências",
				Commands:    []string{"go mod tidy", "go build ./..."},
			},
		}
	}

	// Nil pointer / panic.
	if strings.Contains(errLower, "nil pointer") || strings.Contains(errLower, "panic") ||
		strings.Contains(errLower, "index out of range") {
		return DiagnoseResult{
			Problem: "erro de runtime — provável nil pointer ou panic",
			Cause:   resp.Error,
			IsFatal: true,
			Level:   LevelRollback,
		}
	}

	// Default: unknown error → escalate.
	return DiagnoseResult{
		Problem: "erro não categorizado",
		Cause:   resp.Error,
		Level:   LevelConflict,
	}
}

// diagnoseTestFailure analyzes test failures.
func diagnoseTestFailure(status *KernelStatus, resp *InterruptResponse) DiagnoseResult {
	failRate := float64(resp.TestsRun-resp.TestsPassed) / float64(resp.TestsRun)

	// Few failures (<20%) → auto-correct.
	if failRate <= 0.2 && resp.TestsPassed > 0 {
		return DiagnoseResult{
			Problem: fmt.Sprintf("%d/%d testes falharam (%.0f%%)",
				resp.TestsRun-resp.TestsPassed, resp.TestsRun, failRate*100),
			Cause: "poucos testes falhando — provável ajuste pequeno",
			Level: LevelAutoCorrect,
			Adjustment: &Adjustment{
				Description: "revisar e corrigir testes falhos, retestar",
				RetryTests:  []int{},
			},
		}
	}

	// Moderate failures (20-50%) → restart with rollback check.
	if failRate <= 0.5 {
		return DiagnoseResult{
			Problem: fmt.Sprintf("%d/%d testes falharam (%.0f%%)",
				resp.TestsRun-resp.TestsPassed, resp.TestsRun, failRate*100),
			Cause: "múltiplos testes falhando — possível regressão",
			Level: LevelRollback,
		}
	}

	// Catastrophic failures (>50%) → critical.
	return DiagnoseResult{
		Problem: fmt.Sprintf("%d/%d testes falharam (%.0f%%) — regressão severa",
			resp.TestsRun-resp.TestsPassed, resp.TestsRun, failRate*100),
		Cause:   "maioria dos testes quebrou — rollback necessário",
		IsFatal: true,
		Level:   LevelRollback,
	}
}

// ShouldAutoFix returns true if the diagnosis allows automatic correction.
func (d DiagnoseResult) ShouldAutoFix() bool {
	return d.Level <= LevelAutoCorrect && d.Adjustment != nil
}
