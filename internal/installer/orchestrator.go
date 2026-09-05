// Package installer — COSCA Environment Provisioner.
//
// Este arquivo implementa o ORQUESTRADOR: o maestro que executa as phases na
// ordem canônica, persiste o Installation State (idempotência) e produz a
// trilha de evidência (StepResult) de cada etapa.
package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Phase é uma etapa do provisionamento: um conjunto de Checks orquestrados.
type Phase struct {
	// ID e Name identificam a fase (ex: "preflight", "dependencies").
	ID   string `json:"id"`
	Name string `json:"name"`
	// Checks são as capabilities verificadas/instaladas nesta fase.
	Checks []Check `json:"-"`
	// NextState é o installation state alcançado quando a fase completa.
	NextState State `json:"next_state"`
}

// RunReport é o resultado completo do provisionamento (persistido como
// evidência — o "certificate" do instalador).
type RunReport struct {
	// InstallationID identifica esta execução.
	InstallationID string `json:"installation_id"`
	// Version é a versão do COSCA provisionado.
	Version string `json:"version"`
	// Steps é a trilha de evidência de cada etapa executada.
	Steps []StepResult `json:"steps"`
	// CurrentState é o installation state atual.
	CurrentState State `json:"current_state"`
	// Certified indica se chegou a CERTIFIED.
	Certified bool `json:"certified"`
}

// PersistDir devolve o diretório de evidência do instalador (.cosca/install).
func PersistDir(dataDir string) string {
	return filepath.Join(dataDir, "install")
}

// statePath é o arquivo do installation state persistido.
func statePath(dataDir string) string {
	return filepath.Join(PersistDir(dataDir), "installation.json")
}

// LoadState lê o installation state persistido ("" se nunca rodou).
func LoadState(dataDir string) State {
	data, err := os.ReadFile(statePath(dataDir))
	if err != nil {
		return StateNotReady
	}
	var rep RunReport
	if err := json.Unmarshal(data, &rep); err != nil {
		return StateNotReady
	}
	return rep.CurrentState
}

// Run executa as phases do provisionamento a partir do estado atual (ou do
// início), registrando evidência e persistindo após cada fase. Idempotente:
// uma fase já completa (state >= NextState) é pulada com RESULT=PASS.
//
// `emit` (opcional) recebe os eventos em tempo real — o contrato da UI
// (cosca setup --watch). Nil = modo silencioso.
func Run(dataDir, version string, phases []Phase, emit EmitFunc) (*RunReport, error) {
	dir := PersistDir(dataDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create install dir: %w", err)
	}

	current := LoadState(dataDir)
	rep := &RunReport{
		InstallationID: fmt.Sprintf("COSCA-%s", version),
		Version:        version,
		CurrentState:   current,
		Certified:      current == StateCertified,
	}
	if emit == nil {
		emit = NopEmitter()
	}

	for _, phase := range phases {
		// Idempotência: fase já alcançada é pulada (com evidência).
		if stateIndex(current) >= stateIndex(phase.NextState) {
			step := StepResult{
				Check:    "installer.phase." + phase.ID,
				Action:   "skip",
				Result:   ResultPass,
				State:    current,
				Evidence: []string{"phase already complete (state " + string(current) + ")"},
			}
			rep.Steps = append(rep.Steps, step)
			emit(Event{Type: EventStepComplete, Check: step.Check, Action: step.Action,
				Result: step.Result, State: current, Phase: phase.ID, Message: "already complete", Time: now()})
			continue
		}

		// Executa os checks da fase na ordem.
		phaseOK := true
		for _, c := range phase.Checks {
			emit(Event{Type: EventStepStarted, Check: c.ID(), Phase: phase.ID,
				Message: c.Name(), Time: now()})
			step := runCheck(c, phase.NextState)
			rep.Steps = append(rep.Steps, step)
			emit(Event{Type: EventStepComplete, Check: step.Check, Action: step.Action,
				Result: step.Result, State: phase.NextState, Phase: phase.ID,
				Evidence: step.Evidence, Time: now()})
			if step.Result != ResultPass {
				phaseOK = false
				// Fail = bloqueia a fase; Skip (não aplicável) não bloqueia.
				if step.Result == ResultFail {
					rep.CurrentState = current // permanece onde está
					_ = persist(rep, dataDir)
					emit(Event{Type: EventError, Check: c.ID(), Result: ResultFail,
						State: current, Phase: phase.ID, Message: "phase blocked", Time: now()})
					return rep, fmt.Errorf("phase %s falhou no check %s", phase.ID, c.ID())
				}
			}
		}

		// Fase completa → avança o estado e persiste.
		if phaseOK {
			from := current
			current = phase.NextState
			rep.CurrentState = current
			rep.Certified = current == StateCertified
			emit(Event{Type: EventStateChanged, FromState: from, State: current,
				Phase: phase.ID, Message: phase.Name, Time: now()})
		}
		if err := persist(rep, dataDir); err != nil {
			return nil, err
		}
	}

	if rep.Certified {
		emit(Event{Type: EventCertified, State: StateCertified, Message: "COSCA READY", Time: now()})
	}
	return rep, nil
}

// runCheck executa o ciclo Detectar → Instalar → Validar de um Check.
func runCheck(c Check, next State) StepResult {
	// 1. DETECTAR (sem instalar nada).
	res, ev := c.Detect()
	if res == ResultPass {
		return NewStep(c.ID(), "detect", ResultPass, next, ev...)
	}

	// 2. EXECUTAR (instalar/ativar quando detect != PASS e não é skip).
	if res != ResultSkip {
		if err := c.Install(); err != nil {
			return NewStep(c.ID(), "install", ResultFail, next,
				"install failed: "+err.Error())
		}
	}

	// 3. VALIDAR (provar que FUNCIONA, não só que existe).
	vres, vev := c.Validate()
	if vres != ResultPass {
		return NewStep(c.ID(), "validate", vres, next, vev...)
	}
	return NewStep(c.ID(), "install", ResultPass, next,
		append(ev, vev...)...)
}

// persist grava o relatório no disco (evidência + estado).
func persist(rep *RunReport, dataDir string) error {
	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	path := statePath(dataDir)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// stateIndex devolve a posição de um estado na ordem canônica (-1 se ausente).
func stateIndex(s State) int {
	for i, o := range Order {
		if o == s {
			return i
		}
	}
	return -1
}

// now é o relógio do orquestrador (indireto para testes determinísticos).
var now = time.Now
