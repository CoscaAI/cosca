package integrity

// Helpers de TESTE para isolar o singleton `identityDigest`.
//
// SetIdentityDigest usa um sync.Once (correto para o boot real: seta UMA vez).
// Mas os testes compartilham o MESMO processo, então o Once que dispara primeiro
// "vence" e os testes seguintes não conseguem re-setar o digest → asserções
// quebram (pre-existing bug). Estes helpers escrevem a var diretamente,
// permitindo isolar cada teste.

// resetIdentityForTest limpa o digest (estado limpo p/ cada teste).
func resetIdentityForTest() { identityDigest = "" }

// setIdentityForTest seta o digest diretamente (ignora o Once).
func setIdentityForTest(d string) { identityDigest = d }
