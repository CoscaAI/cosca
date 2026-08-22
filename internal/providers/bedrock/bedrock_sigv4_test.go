package bedrock

import (
	"context"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/embeddings"
)

// Vetores de teste do AWS SigV4 (https://docs.aws.amazon.com/AmazonS3/latest/API/sig-v4-header-based-auth.html)
var (
	testSecret  = "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY"
	testDate    = "20130524"
	testRegion  = "us-east-1"
	testService = "s3"
)

func TestSigV4SigningKey(t *testing.T) {
	key := getSignatureKey(testSecret, testDate, testRegion, testService)
	if len(key) != 32 { // SHA-256 HMAC → 32 bytes
		t.Fatalf("signing key len = %d", len(key))
	}
	// Determinístico.
	key2 := getSignatureKey(testSecret, testDate, testRegion, testService)
	if string(key) != string(key2) {
		t.Fatal("signing key não determinístico")
	}
	// Diferentes datas → chaves diferentes.
	key3 := getSignatureKey(testSecret, "20130525", testRegion, testService)
	if string(key) == string(key3) {
		t.Fatal("datas diferentes devem gerar chaves diferentes")
	}
}

func TestHmacSHA256AndSHA256Hex(t *testing.T) {
	// HMAC determinístico e tamanho correto.
	h := hmacSHA256([]byte("key"), []byte("data"))
	if len(h) != 32 {
		t.Fatalf("hmac len = %d", len(h))
	}
	if string(h) != string(hmacSHA256([]byte("key"), []byte("data"))) {
		t.Fatal("hmac não determinístico")
	}
	// sha256Hex: 64 hex chars.
	hexStr := sha256Hex([]byte("hello"))
	if len(hexStr) != 64 {
		t.Fatalf("sha256Hex len = %d", len(hexStr))
	}
	if _, err := hex.DecodeString(hexStr); err != nil {
		t.Fatalf("sha256Hex não é hex válido: %v", err)
	}
}

func TestIsBadRequest(t *testing.T) {
	if !isBadRequest(&fakeErr{"service error (400)"}) {
		t.Fatal("400 deve ser bad request")
	}
	if !isBadRequest(&fakeErr{"unauthorized (403)"}) {
		t.Fatal("403 deve ser bad request")
	}
	if isBadRequest(&fakeErr{"network timeout (500)"}) {
		t.Fatal("500 não deve ser bad request")
	}
}

func TestFactoryAndRegister(t *testing.T) {
	// Factory retorna nome + factory não-nil.
	name, factory := Factory()
	if name != "bedrock" || factory == nil {
		t.Fatalf("factory: %q, %v", name, factory)
	}
	// Chamada da factory com config vazio: pode retornar provider ou erro —
	// o contrato é NUNCA panicar.
	_, _ = factory(context.Background(), &embeddings.Config{})
	// Register não deve panicar.
	Register()
	_ = strings.ToUpper
}

type fakeErr struct{ msg string }

func (e *fakeErr) Error() string { return e.msg }
