package rest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/brainweb"
)

// TestReadActivityLog é um conjunto de regressão (table-driven, AAA) para a
// função não exportada readActivityLog. Cobre: arquivo inexistente, limite de
// 3 sobre 5 válidas, pulo de linha malformada/vazia, roll agent→actor e
// formato do ID.
//
// Após a correção de segurança (bug G6), o campo de atividade mudou de
// `Prompt` para `Action`, que carrega APENAS o rótulo seguro (a constante
// "COMMAND_EXECUTED") — nunca a entrada/prompt sensível do usuário. Este
// teste ajusta o assert para esse comportamento correto e também garante que
// nenhum prompt sensível vaza no payload público.
func TestReadActivityLog(t *testing.T) {
	dir := t.TempDir()

	write := func(t *testing.T, name, content string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		return p
	}

	// 5 linhas válidas — a fonte de atividade real (append-only). Cada linha
	// traz o rótulo seguro ("COMMAND_EXECUTED") e um prompt sensível que NUNCA
	// deve vazar no payload público (regressão do bug G6).
	valid5 := "" +
		`{"at":1,"actor":"don","action":"COMMAND_EXECUTED","prompt":"SENSIVEL-1","status":"ok"}` + "\n" +
		`{"at":2,"actor":"kernel","agent":"Backend Chief","action":"COMMAND_EXECUTED","prompt":"SENSIVEL-2","status":"ok"}` + "\n" +
		`{"at":3,"actor":"sys","agent":"QA","action":"COMMAND_EXECUTED","prompt":"SENSIVEL-3","status":"ok"}` + "\n" +
		`{"at":4,"actor":"x","action":"COMMAND_EXECUTED","prompt":"SENSIVEL-4","status":"ok"}` + "\n" +
		`{"at":5,"actor":"y","action":"COMMAND_EXECUTED","prompt":"SENSIVEL-5","status":"ok"}` + "\n"

	cases := []struct {
		name    string
		content string
		write   bool // false => caminho aponta para arquivo inexistente
		limit   int
		wantLen int
		check   func(t *testing.T, out []brainweb.Activity)
	}{
		{
			name:    "arquivo_inexistente_retorna_vazio",
			write:   false,
			limit:   10,
			wantLen: 0,
		},
		{
			name:    "cinco_validas_limite_3_mais_recentes",
			content: valid5,
			write:   true,
			limit:   3,
			wantLen: 3,
			check: func(t *testing.T, out []brainweb.Activity) {
				// readActivityLog devolve as mais recentes primeiro (at desc).
				want := []brainweb.Activity{
					{ID: "5-y", Agent: "y", Action: "COMMAND_EXECUTED", At: 5},   // agent vazio => actor "y"
					{ID: "4-x", Agent: "x", Action: "COMMAND_EXECUTED", At: 4},   // agent vazio => actor "x"
					{ID: "3-QA", Agent: "QA", Action: "COMMAND_EXECUTED", At: 3}, // agent presente => "QA"
				}
				for i, w := range want {
					got := out[i]
					if got.ID != w.ID || got.Agent != w.Agent || got.Action != w.Action || got.At != w.At {
						t.Fatalf("item %d = %+v, esperado %+v", i, got, w)
					}
				}
			},
		},
		{
			name: "linha_malformada_e_vazia_puladas",
			content: "" +
				`{"at":10,"actor":"a","action":"COMMAND_EXECUTED","prompt":"SENSIVEL-A","status":"ok"}` + "\n" +
				`{"at":20,"actor":"b","action":"COMMAND_EXECUTED","prompt":"SENSIVEL-B","status":"ok"}` + "\n" +
				`isto não é json valido` + "\n" +
				`{"at":"100","actor":"c","action":"COMMAND_EXECUTED","status":"ok"}` + "\n" +
				"\n" +
				`{"at":"notanint55","action":"COMMAND_EXECUTED"}` + "\n",
			write:   true,
			limit:   10,
			wantLen: 2,
			check: func(t *testing.T, out []brainweb.Activity) {
				want := []brainweb.Activity{
					{ID: "20-b", Agent: "b", Action: "COMMAND_EXECUTED", At: 20},
					{ID: "10-a", Agent: "a", Action: "COMMAND_EXECUTED", At: 10},
				}
				for i, w := range want {
					got := out[i]
					if got.ID != w.ID || got.Agent != w.Agent || got.Action != w.Action || got.At != w.At {
						t.Fatalf("item %d = %+v, esperado %+v", i, got, w)
					}
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var path string
			if tc.write {
				path = write(t, tc.name+".jsonl", tc.content)
			} else {
				path = filepath.Join(dir, tc.name+"-nao-existe.jsonl")
			}
			out := readActivityLog(path, tc.limit)
			if len(out) != tc.wantLen {
				t.Fatalf("len=%d, esperado %d (out=%+v)", len(out), tc.wantLen, out)
			}
			// Nenhum prompt sensível pode vazar no payload público (bug G6).
			if strings.Contains(tc.content, "SENSIVEL") {
				for i := range out {
					if strings.Contains(strings.Join(fieldValues(out[i]), ","), "SENSIVEL") {
						t.Fatalf("item %d vaza prompt sensível: %+v", i, out[i])
					}
				}
			}
			if tc.check != nil {
				tc.check(t, out)
			}
		})
	}
}

// fieldValues devolve todos os valores string da Activity para checar vazamento.
func fieldValues(a brainweb.Activity) []string {
	return []string{a.ID, a.Agent, a.Status, a.Action, a.Provider, a.Model}
}
