// guard.go — Guard de vazamento de segredo na fronteira de conteúdo externo.
//
// Mineração git-secrets: "vazamento como enforcement no fluxo". Aqui o
// enforcement é aplicado no PONTO CERTO da arquitetura do Cosca — o conteúdo
// que veio de fora (OriginTool/OriginMCP/OriginPlugin) — antes de entrar no
// contexto/persistência, respeitando o I8 (conteúdo externo nunca concede
// autoridade) e ampliando com I6 (quarentena) + I2 (fail-closed).
package contenttrust

import (
	"github.com/CoscaAI/cosca/internal/security"
)

// GuardSecrets varre o conteúdo do item em busca de segredos embutidos. Se um
// segredo for detectado:
//   - marca o item como Quarantined (I6) — é excluído via IsExcluded (fail-closed, I2);
//   - mascara o segredo no próprio conteúdo (defesa em profundidade — o valor cru
//     não flui para contexto/persistência/ledger).
//
// Determinístico (I1): pura regex, zero LLM. NUNCA promove autoridade (I8).
func GuardSecrets(item Item) Item {
	res := security.DetectBytes([]byte(item.Content), item.Source)
	if !res.HasSecrets() {
		return item
	}
	item.PolicyState = StateQuarantined
	item.Content = maskByRange(item.Content, res)
	return item
}

// maskByRange substitui cada segredo (por faixa de bytes) pela forma mascarada.
// Aplica em ORDEM INVERSA para não deslocar os offsets das faixas posteriores.
func maskByRange(content string, res *security.SecretScanResult) string {
	if res == nil || len(res.Matches) == 0 {
		return content
	}
	data := []byte(content)
	for i := len(res.Matches) - 1; i >= 0; i-- {
		m := res.Matches[i]
		if m.Start < 0 || m.End > len(data) || m.Start >= m.End {
			continue
		}
		data = append(data[:m.Start], append([]byte(m.Masked), data[m.End:]...)...)
	}
	return string(data)
}

// HasGuardError é conveniência para quem quer saber se o guard marcou o item.
func HasGuardError(item Item) bool {
	return item.PolicyState == StateQuarantined
}
