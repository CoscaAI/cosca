package compute

import (
	"os"
	"testing"
)

// =============================================================================
// envPercent — over-subscription permitido (ordem do Don: pools mais altos)
// =============================================================================

func TestEnvPercent_OverSubscription(t *testing.T) {
	// O Don pediu pools acima de 100% dos cores (over-subscription) — o antigo
	// limite de 1.0 rejeitava 1.5 silenciosamente (bug real encontrado ao vivo).
	tests := []struct {
		name string
		val  string
		want float64
	}{
		{"vazio usa default", "", 0.75},
		{"1.0 válido", "1.0", 1.0},
		{"1.5 over-subscription", "1.5", 1.5},
		{"2.0 over-subscription", "2.0", 2.0},
		{"4.0 teto máximo", "4.0", 4.0},
		{"5.0 acima do teto → default", "5.0", 0.75},
		{"negativo → default", "-1", 0.75},
		{"zero → default", "0", 0.75},
		{"lixo → default", "abc", 0.75},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("COSCA_FABRIC_TEST_PCT", tt.val)
			defer os.Unsetenv("COSCA_FABRIC_TEST_PCT")
			if got := envPercent("COSCA_FABRIC_TEST_PCT", 0.75); got != tt.want {
				t.Errorf("envPercent(%q) = %v, want %v", tt.val, got, tt.want)
			}
		})
	}
}

func TestEnvPercentMax_CustomCap(t *testing.T) {
	os.Setenv("COSCA_FABRIC_TEST_MAX", "2.5")
	defer os.Unsetenv("COSCA_FABRIC_TEST_MAX")
	// Com teto 2.0, 2.5 é rejeitado → default.
	if got := envPercentMax("COSCA_FABRIC_TEST_MAX", 0.5, 2.0); got != 0.5 {
		t.Errorf("envPercentMax(cap 2.0, val 2.5) = %v, want 0.5 (default)", got)
	}
	// Com teto 3.0, 2.5 é aceito.
	if got := envPercentMax("COSCA_FABRIC_TEST_MAX", 0.5, 3.0); got != 2.5 {
		t.Errorf("envPercentMax(cap 3.0, val 2.5) = %v, want 2.5", got)
	}
}
