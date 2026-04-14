package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	plaintext := []byte("Hello, Cubbit Pages!")
	password := "test-password-123"

	encrypted, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := Decrypt(encrypted, password)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Fatalf("round-trip mismatch: got %q, want %q", decrypted, plaintext)
	}
}

func TestDecryptWrongPassword(t *testing.T) {
	plaintext := []byte("secret data")
	encrypted, err := Encrypt(plaintext, "correct-password")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = Decrypt(encrypted, "wrong-password")
	if err == nil {
		t.Fatal("expected error decrypting with wrong password")
	}
	if err != ErrDecryptionFailed {
		t.Fatalf("expected ErrDecryptionFailed, got: %v", err)
	}
}

func TestEncryptProducesDifferentOutput(t *testing.T) {
	plaintext := []byte("same content")
	password := "same-password"

	enc1, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("Encrypt 1 failed: %v", err)
	}

	enc2, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("Encrypt 2 failed: %v", err)
	}

	if bytes.Equal(enc1, enc2) {
		t.Fatal("two encryptions of the same content produced identical output")
	}
}

func TestMagicHeaderPresent(t *testing.T) {
	encrypted, err := Encrypt([]byte("data"), "password")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if encrypted[0] != MagicByte0 || encrypted[1] != MagicByte1 ||
		encrypted[2] != MagicByte2 || encrypted[3] != MagicByte3 {
		t.Fatalf("magic header CPGS not found, got: %x %x %x %x",
			encrypted[0], encrypted[1], encrypted[2], encrypted[3])
	}
}

func TestDecryptFileTooShort(t *testing.T) {
	_, err := Decrypt([]byte("short"), "password")
	if err != ErrFileTooShort {
		t.Fatalf("expected ErrFileTooShort, got: %v", err)
	}
}

func TestDecryptInvalidMagic(t *testing.T) {
	data := make([]byte, HeaderLength+16)
	data[0] = 0xFF // wrong magic
	_, err := Decrypt(data, "password")
	if err != ErrInvalidMagic {
		t.Fatalf("expected ErrInvalidMagic, got: %v", err)
	}
}

func TestCanaryVerifyCorrectPassword(t *testing.T) {
	canary, err := EncryptCanary("my-password")
	if err != nil {
		t.Fatalf("EncryptCanary failed: %v", err)
	}

	if !VerifyCanary(canary, "my-password") {
		t.Fatal("VerifyCanary returned false with correct password")
	}
}

func TestCanaryVerifyWrongPassword(t *testing.T) {
	canary, err := EncryptCanary("my-password")
	if err != nil {
		t.Fatalf("EncryptCanary failed: %v", err)
	}

	if VerifyCanary(canary, "wrong-password") {
		t.Fatal("VerifyCanary returned true with wrong password")
	}
}

func TestEncryptEmptyPlaintext(t *testing.T) {
	encrypted, err := Encrypt([]byte{}, "password")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := Decrypt(encrypted, "password")
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if len(decrypted) != 0 {
		t.Fatalf("expected empty plaintext, got %d bytes", len(decrypted))
	}
}

func TestVersionByte(t *testing.T) {
	encrypted, err := Encrypt([]byte("data"), "password")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if encrypted[MagicLength] != Version {
		t.Fatalf("expected version %d, got %d", Version, encrypted[MagicLength])
	}
}
