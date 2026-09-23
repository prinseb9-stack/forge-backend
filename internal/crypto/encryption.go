// Package crypto provides encryption helpers for FORGE.
//
// Tokens issued by platform APIs (Bluesky JWTs, future platform OAuth
// tokens) grant posting rights on the user's behalf. They are stored
// encrypted at rest with AES-256-GCM.
//
// The encryption key is a 32-byte value provided as a base64-encoded
// string via the TOKEN_ENCRYPTION_KEY environment variable.
//
// Storage format: base64(iv || ciphertext || authTag)
//   - iv:      12 random bytes, generated per encryption
//   - authTag: 16-byte GCM authentication tag
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
)

const (
	// KeySize is the required AES-256 key length in bytes.
	KeySize = 32

	// IVSize is the GCM nonce length in bytes.
	IVSize = 12
)

// EncryptionService handles encryption and decryption of tokens at rest.
type EncryptionService struct {
	aead cipher.AEAD
}

// NewEncryptionService creates a new service from a base64-encoded
// 32-byte key.
//
// Returns an error if:
//   - base64 encoding is invalid
//   - decoded key is not exactly 32 bytes
//   - underlying cipher cannot be constructed
func NewEncryptionService(base64Key string) (*EncryptionService, error) {
	if base64Key == "" {
		return nil, fmt.Errorf("encryption key is empty")
	}

	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, fmt.Errorf("encryption key is not valid base64: %w", err)
	}

	if len(key) != KeySize {
		return nil, fmt.Errorf("encryption key must be %d bytes, got %d", KeySize, len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	return &EncryptionService{aead: aead}, nil
}

// Encrypt encrypts plaintext and returns a base64-encoded string
// containing the IV, ciphertext, and authentication tag.
//
// An empty plaintext returns an empty string (not an encrypted blob).
func (s *EncryptionService) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	iv := make([]byte, IVSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", fmt.Errorf("failed to generate IV: %w", err)
	}

	// Seal appends the ciphertext + tag to iv
	sealed := s.aead.Seal(nil, iv, []byte(plaintext), nil)

	// Concatenate iv || sealed
	combined := make([]byte, 0, IVSize+len(sealed))
	combined = append(combined, iv...)
	combined = append(combined, sealed...)

	return base64.StdEncoding.EncodeToString(combined), nil
}

// Decrypt reverses Encrypt.
//
// An empty input returns an empty plaintext.
func (s *EncryptionService) Decrypt(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}

	combined, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("encoded value is not valid base64: %w", err)
	}

	if len(combined) < IVSize+16 {
		return "", fmt.Errorf("encoded value is too short to contain IV + tag")
	}

	iv := combined[:IVSize]
	ciphertext := combined[IVSize:]

	plaintext, err := s.aead.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// EncryptBytes is a convenience wrapper for byte slices.
func (s *EncryptionService) EncryptBytes(data []byte) ([]byte, error) {
	encoded, err := s.Encrypt(string(data))
	if err != nil {
		return nil, err
	}
	return []byte(encoded), nil
}

// helper: unused import guard (kept for future utility functions)
var _ = binary.BigEndian
