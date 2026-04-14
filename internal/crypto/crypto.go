package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

const (
	// Magic header "CPGS" (Cubbit Pages)
	MagicByte0 = 0x43 // C
	MagicByte1 = 0x50 // P
	MagicByte2 = 0x47 // G
	MagicByte3 = 0x53 // S

	Version        = 1
	SaltLength     = 16
	NonceLength    = 12
	MagicLength    = 4
	HeaderLength   = MagicLength + 1 + SaltLength + NonceLength // 33 bytes
	KeyLength      = 32                                         // 256-bit
	PBKDF2Iterations = 100_000

	CanaryPlaintext = "cubbit-pages-ok"
)

var (
	Magic = [MagicLength]byte{MagicByte0, MagicByte1, MagicByte2, MagicByte3}

	ErrFileTooShort    = errors.New("file too short to be a valid Cubbit Pages file")
	ErrInvalidMagic    = errors.New("invalid file: missing CPGS magic header")
	ErrUnsupportedVer  = errors.New("unsupported file version")
	ErrDecryptionFailed = errors.New("decryption failed — wrong password or corrupted file")
)

// DeriveKey derives a 256-bit AES key from a password and salt using PBKDF2-SHA256.
func DeriveKey(password string, salt []byte) []byte {
	return pbkdf2.Key([]byte(password), salt, PBKDF2Iterations, KeyLength, sha256.New)
}

// Encrypt encrypts plaintext with AES-256-GCM using the given password.
// Output format: [4B magic][1B version][16B salt][12B nonce][N bytes ciphertext+tag]
func Encrypt(plaintext []byte, password string) ([]byte, error) {
	salt := make([]byte, SaltLength)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("generating salt: %w", err)
	}

	nonce := make([]byte, NonceLength)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}

	key := DeriveKey(password, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	output := make([]byte, 0, HeaderLength+len(ciphertext))
	output = append(output, Magic[:]...)
	output = append(output, Version)
	output = append(output, salt...)
	output = append(output, nonce...)
	output = append(output, ciphertext...)

	return output, nil
}

// Decrypt decrypts data produced by Encrypt.
func Decrypt(data []byte, password string) ([]byte, error) {
	if len(data) < HeaderLength {
		return nil, ErrFileTooShort
	}

	// Verify magic
	if data[0] != Magic[0] || data[1] != Magic[1] || data[2] != Magic[2] || data[3] != Magic[3] {
		return nil, ErrInvalidMagic
	}

	// Verify version
	if data[MagicLength] != Version {
		return nil, fmt.Errorf("%w: %d", ErrUnsupportedVer, data[MagicLength])
	}

	offset := MagicLength + 1
	salt := data[offset : offset+SaltLength]
	offset += SaltLength
	nonce := data[offset : offset+NonceLength]
	offset += NonceLength
	ciphertext := data[offset:]

	key := DeriveKey(password, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// EncryptCanary encrypts the canary plaintext for password verification.
func EncryptCanary(password string) ([]byte, error) {
	return Encrypt([]byte(CanaryPlaintext), password)
}

// VerifyCanary attempts to decrypt canary data and checks the result.
func VerifyCanary(data []byte, password string) bool {
	plaintext, err := Decrypt(data, password)
	if err != nil {
		return false
	}
	return string(plaintext) == CanaryPlaintext
}
