// Package estimator implementa o Execution Plan Estimator do Cosca: um sistema
// que, ANTES de qualquer aprovação de execução, apresenta uma estimativa
// estruturada de transparência — arquivos afetados, testes previstos,
// migrações, rollback, tempo, risco e confiança.
//
// Todos os cálculos usam apenas a stdlib e seguem heurísticas documentadas
// (campo Method), para que a estimativa seja honesta e rastreável.
package estimator

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ExecutionPlan é a estimativa estruturada de transparência de uma execução
// planejada. É o que o Don vê ANTES de aprovar qualquer execução.
type ExecutionPlan struct {
	// FilesAffected é o número de arquivos que serão afetados.
	FilesAffected int

	// Files é a lista dos arquivos que serão alterados (caminhos absolutos).
	Files []string

	// TestsExpected é a estimativa de funções de teste afetadas/previstas.
	TestsExpected int

	// TestPackages são os pacotes Go que seriam testados.
	TestPackages []string

	// Migrations são os nomes das migrações envolvidas (vazio = nenhuma).
	Migrations []string

	// RollbackAvailable indica se há rollback disponível (repo git com commits).
	RollbackAvailable bool

	// RollbackDetail descreve o rollback (ex: "disponível (git revert bdaef4c)").
	RollbackDetail string

	// EstimatedMinutes é o tempo estimado de execução em minutos.
	EstimatedMinutes int

	// RiskLevel é o risco da alteração: "baixo", "médio" ou "alto".
	RiskLevel string

	// ConfidencePercent é a confiança percentual (0-100) da estimativa.
	ConfidencePercent int

	// GeneratedAt é o momento em que o plano foi gerado.
	GeneratedAt time.Time

	// Method informa como a confiança foi calculada: "histórico" (Trust
	// Registry do agente) ou "heurístico" (default calibrado).
	Method string
}

// String formata o plano no formato exato exigido pelo Don:
//
//	Plano de Execução
//	─────────────────────────────
//	Arquivos afetados: 18
//	Testes previstos: 143
//	Migrações: nenhuma
//	Rollback: disponível
//	Tempo estimado: 6 min
//	Risco da alteração: baixo
//	Confiança: 96%
func (p *ExecutionPlan) String() string {
	migrations := "nenhuma"
	if len(p.Migrations) > 0 {
		migrations = strings.Join(p.Migrations, ", ")
	}
	rollback := "não disponível"
	if p.RollbackAvailable {
		rollback = "disponível"
	}
	return fmt.Sprintf(
		"Plano de Execução\n"+
			"─────────────────────────────\n"+
			"Arquivos afetados: %d\n"+
			"Testes previstos: %d\n"+
			"Migrações: %s\n"+
			"Rollback: %s\n"+
			"Tempo estimado: %d min\n"+
			"Risco da alteração: %s\n"+
			"Confiança: %d%%",
		p.FilesAffected,
		p.TestsExpected,
		migrations,
		rollback,
		p.EstimatedMinutes,
		p.RiskLevel,
		p.ConfidencePercent,
	)
}

// Estimate gera o ExecutionPlan a partir do escopo da execução:
//
//  1. Expande o escopo em arquivos reais (ExpandScope).
//  2. Deriva os pacotes que seriam testados e conta os testes (CountTests).
//  3. Detecta migrações entre os arquivos afetados.
//  4. Verifica disponibilidade de rollback (CheckRollback).
//  5. Calcula tempo (EstimateTime), risco (AssessRisk) e confiança
//     (AgentConfidence + ComputeConfidence).
func Estimate(scope ExecutionScope) (*ExecutionPlan, error) {
	if scope.ProjectDir == "" {
		return nil, fmt.Errorf("estimator: ProjectDir é obrigatório")
	}

	files, err := ExpandScope(scope)
	if err != nil {
		return nil, err
	}

	changeType := scope.ChangeType
	if changeType == "" {
		changeType = "feature"
	}

	testPackages := packagesForFiles(files)
	tests, err := CountTests(testPackages)
	if err != nil {
		return nil, err
	}

	migrations := detectMigrations(files)
	rollbackAvailable, rollbackDetail := CheckRollback(scope.ProjectDir)
	estimatedMinutes := EstimateTime(len(files), tests, changeType)
	risk := AssessRisk(files, tests, len(migrations), changeType)

	agentConfidence, found := lookupAgentConfidence(scope.Agent, trustRegistryPath(scope.ProjectDir))
	method := "heurístico"
	if found {
		method = "histórico"
	}
	confidence := ComputeConfidence(risk, agentConfidence)

	return &ExecutionPlan{
		FilesAffected:     len(files),
		Files:             files,
		TestsExpected:     tests,
		TestPackages:      testPackages,
		Migrations:        migrations,
		RollbackAvailable: rollbackAvailable,
		RollbackDetail:    rollbackDetail,
		EstimatedMinutes:  estimatedMinutes,
		RiskLevel:         risk,
		ConfidencePercent: confidence,
		GeneratedAt:       time.Now(),
		Method:            method,
	}, nil
}

// packagesForFiles deriva os pacotes Go que seriam testados: o diretório de
// cada arquivo .go afetado (testes vivem no mesmo diretório do pacote).
func packagesForFiles(files []string) []string {
	set := make(map[string]struct{})
	for _, f := range files {
		if !strings.HasSuffix(f, ".go") {
			continue
		}
		set[filepath.Dir(f)] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for d := range set {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// detectMigrations identifica arquivos de migração entre os afetados pelo
// caminho/nome (contém "migration" ou "migrate"). Retorna os nomes dos
// arquivos; vazio significa nenhuma migração.
func detectMigrations(files []string) []string {
	var out []string
	for _, f := range files {
		slashed := strings.ToLower(filepath.ToSlash(f))
		base := strings.ToLower(filepath.Base(f))
		if strings.Contains(slashed, "/migration") ||
			strings.Contains(slashed, "/migrate") ||
			strings.Contains(base, "migration") ||
			strings.Contains(base, "migrate") {
			out = append(out, filepath.Base(f))
		}
	}
	return out
}
