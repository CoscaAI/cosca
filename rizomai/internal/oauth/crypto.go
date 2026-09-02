// Package oauth concentra a criptografia de tokens em repouso e o broker OAuth
// (ADR-006): AES-256-GCM com chave de env RIZOMAI_TOKEN_KEY (32 bytes hex).
// O cliente nunca vê tokens de rede social — apenas a API key.
package oauth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// ErrInvalidKey indica chave de criptografia malformada.
var ErrInvalidKey = errors.New("oauth: chave RIZOMAI_TOKEN_KEY inválida")

// TokenKey parseia a chave de 32 bytes: aceita hex (64 chars) ou raw (32 bytes).
func TokenKey(hexKey string) ([]byte, error) {
	if len(hexKey) == 64 {
		b, err := hex.DecodeString(hexKey)
		if err != nil {
			return nil, fmt.Errorf("%w: hex inválido", ErrInvalidKey)
		}
		return b, nil
	}
	if len(hexKey) == 32 {
		return []byte(hexKey), nil
	}
	return nil, fmt.Errorf("%w: esperado 64 hex chars ou 32 bytes, veio %d", ErrInvalidKey, len(hexKey))
}

// Encrypt cifra plaintext com AES-256-GCM. Formato: nonce(12) || ciphertext.
func Encrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt decifra ciphertext no formato nonce(12) || ciphertext.
func Decrypt(ciphertext, key []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, nil // coluna vazia (conta sem token ainda)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("oauth: ciphertext muito curto")
	}
	nonce, ct := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ct, nil)
}
