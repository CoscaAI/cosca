// Testes da criptografia de tokens em repouso (ADR-006 §1.2 — AES-256-GCM).
package oauth

import (
	"bytes"
	"testing"
)

func testKey(t *testing.T) []byte {
	t.Helper()
	key, err := TokenKey("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func TestEncryptDecryptRoundtrip(t *testing.T) {
	key := testKey(t)
	plaintext := []byte(`{"access_token":"secret-token-xyz"}`)

	ct, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if bytes.Equal(ct, plaintext) {
		t.Error("ciphertext igual ao plaintext — não criptografou")
	}
	if len(ct) <= 12 {
		t.Errorf("ciphertext curto demais (nonce ausente?): %d bytes", len(ct))
	}

	pt, err := Decrypt(ct, key)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(pt, plaintext) {
		t.Errorf("roundtrip divergiu:\n got %q\nwant %q", pt, plaintext)
	}
}

func TestEncryptUsesFreshNonce(t *testing.T) {
	key := testKey(t)
	c1, _ := Encrypt([]byte("mesmo-plaintext"), key)
	c2, _ := Encrypt([]byte("mesmo-plaintext"), key)
	if bytes.Equal(c1, c2) {
		t.Error("dois encrypts do mesmo plaintext produziram o mesmo ciphertext (nonce reutilizado?)")
	}
}

func TestDecryptWrongKeyFails(t *testing.T) {
	key := testKey(t)
	ct, _ := Encrypt([]byte("segredo"), key)

	other, _ := TokenKey("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
	if _, err := Decrypt(ct, other); err == nil {
		t.Error("decrypt com chave errada deveria falhar (GCM autentica)")
	}
}

func TestDecryptEmpty(t *testing.T) {
	key := testKey(t)
	pt, err := Decrypt(nil, key)
	if err != nil {
		t.Fatalf("Decrypt(nil): %v", err)
	}
	if pt != nil {
		t.Errorf("Decrypt(nil) = %q, esperado nil", pt)
	}
}

func TestTokenKeyInvalid(t *testing.T) {
	if _, err := TokenKey("abc"); err == nil {
		t.Error("chave malformada deveria falhar")
	}
	if _, err := TokenKey(""); err == nil {
		t.Error("chave vazia deveria falhar")
	}
	if _, err := TokenKey("zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"); err == nil {
		t.Error("hex inválido deveria falhar")
	}
}
