package crypto

import (
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestGoEncryptJSDecrypt encrypts data with Go and decrypts with Node.js
// using the same Web Crypto API code that runs in the service worker.
// This is the critical interop test: if Go and JS crypto are incompatible,
// this test catches it.
func TestGoEncryptJSDecrypt(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not found, skipping JS interop test")
	}

	testCases := []struct {
		name      string
		plaintext string
		password  string
	}{
		{"simple text", "Hello, Cubbit Pages!", "test-password-123"},
		{"canary", CanaryPlaintext, "my-secret"},
		{"empty", "", "password"},
		{"unicode", "日本語テスト 🔐", "пароль"},
		{"html", "<html><head><script>alert('xss')</script></head></html>", "p@ss!"},
		{"binary-like", "\x00\x01\x02\xff\xfe\xfd", "binary-password"},
		{"large", strings.Repeat("A", 100000), "large-file-password"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			encrypted, err := Encrypt([]byte(tc.plaintext), tc.password)
			if err != nil {
				t.Fatalf("Go Encrypt failed: %v", err)
			}

			// Write encrypted data to temp file as hex (safe for all byte values)
			tmpDir := t.TempDir()
			encFile := filepath.Join(tmpDir, "data.enc.hex")
			if err := os.WriteFile(encFile, []byte(hex.EncodeToString(encrypted)), 0o644); err != nil {
				t.Fatalf("writing temp file: %v", err)
			}

			// Node.js script that decrypts using Web Crypto API (same as sw.js)
			jsScript := fmt.Sprintf(`
const { subtle } = require('crypto').webcrypto || globalThis.crypto;
const fs = require('fs');

const MAGIC = [0x43, 0x50, 0x47, 0x53];
const VERSION = %d;
const SALT_LEN = %d;
const NONCE_LEN = %d;
const HEADER_LEN = 4 + 1 + SALT_LEN + NONCE_LEN;
const ITERATIONS = %d;

async function decrypt(hexData, password) {
  const data = Buffer.from(hexData, 'hex');

  if (data.length < HEADER_LEN) throw new Error('too short');
  for (let i = 0; i < 4; i++) {
    if (data[i] !== MAGIC[i]) throw new Error('bad magic');
  }
  if (data[4] !== VERSION) throw new Error('bad version');

  let off = 5;
  const salt = data.slice(off, off + SALT_LEN); off += SALT_LEN;
  const nonce = data.slice(off, off + NONCE_LEN); off += NONCE_LEN;
  const ct = data.slice(off);

  const enc = new TextEncoder();
  const km = await subtle.importKey('raw', enc.encode(password), 'PBKDF2', false, ['deriveKey']);
  const key = await subtle.deriveKey(
    { name: 'PBKDF2', salt: salt, iterations: ITERATIONS, hash: 'SHA-256' },
    km,
    { name: 'AES-GCM', length: 256 },
    false,
    ['decrypt']
  );
  const plain = await subtle.decrypt({ name: 'AES-GCM', iv: nonce, tagLength: 128 }, key, ct);
  return Buffer.from(plain);
}

(async () => {
  const hexData = fs.readFileSync(process.argv[2], 'utf-8').trim();
  const password = process.argv[3];
  const result = await decrypt(hexData, password);
  // Output as hex so we can compare binary-safe
  process.stdout.write(result.toString('hex'));
})().catch(e => { console.error(e.message); process.exit(1); });
`, Version, SaltLength, NonceLength, PBKDF2Iterations)

			jsFile := filepath.Join(tmpDir, "decrypt.js")
			if err := os.WriteFile(jsFile, []byte(jsScript), 0o644); err != nil {
				t.Fatalf("writing JS file: %v", err)
			}

			cmd := exec.Command("node", jsFile, encFile, tc.password)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Node.js decryption failed: %v\nOutput: %s", err, string(output))
			}

			decryptedBytes, err := hex.DecodeString(strings.TrimSpace(string(output)))
			if err != nil {
				t.Fatalf("decoding Node.js hex output: %v", err)
			}

			if string(decryptedBytes) != tc.plaintext {
				t.Fatalf("JS decrypted content doesn't match Go plaintext.\nGot:  %q\nWant: %q",
					string(decryptedBytes), tc.plaintext)
			}
		})
	}
}

// TestGoEncryptJSDecryptWrongPassword verifies that JS correctly rejects wrong passwords.
func TestGoEncryptJSDecryptWrongPassword(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not found, skipping JS interop test")
	}

	encrypted, err := Encrypt([]byte("secret"), "correct-password")
	if err != nil {
		t.Fatalf("Go Encrypt failed: %v", err)
	}

	tmpDir := t.TempDir()
	encFile := filepath.Join(tmpDir, "data.enc.hex")
	if err := os.WriteFile(encFile, []byte(hex.EncodeToString(encrypted)), 0o644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}

	jsScript := fmt.Sprintf(`
const { subtle } = require('crypto').webcrypto || globalThis.crypto;
const fs = require('fs');
async function decrypt(hexData, password) {
  const data = Buffer.from(hexData, 'hex');
  let off = 5;
  const salt = data.slice(off, off + %d); off += %d;
  const nonce = data.slice(off, off + %d); off += %d;
  const ct = data.slice(off);
  const enc = new TextEncoder();
  const km = await subtle.importKey('raw', enc.encode(password), 'PBKDF2', false, ['deriveKey']);
  const key = await subtle.deriveKey(
    { name: 'PBKDF2', salt, iterations: %d, hash: 'SHA-256' },
    km, { name: 'AES-GCM', length: 256 }, false, ['decrypt']
  );
  return subtle.decrypt({ name: 'AES-GCM', iv: nonce, tagLength: 128 }, key, ct);
}
(async () => {
  const hexData = fs.readFileSync(process.argv[2], 'utf-8').trim();
  await decrypt(hexData, 'wrong-password');
  console.log('ERROR: decryption should have failed');
  process.exit(1);
})().catch(() => { process.exit(0); });
`, SaltLength, SaltLength, NonceLength, NonceLength, PBKDF2Iterations)

	jsFile := filepath.Join(tmpDir, "decrypt_wrong.js")
	if err := os.WriteFile(jsFile, []byte(jsScript), 0o644); err != nil {
		t.Fatalf("writing JS file: %v", err)
	}

	cmd := exec.Command("node", jsFile, encFile)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Node.js should exit 0 on expected decryption failure, got: %v", err)
	}
}

// TestEncryptedFileStructure verifies the binary layout of .enc files
// matches what the JS parser expects: [4B magic][1B version][16B salt][12B nonce][...ciphertext+tag]
func TestEncryptedFileStructure(t *testing.T) {
	plaintext := []byte("test data for structure validation")
	password := "structure-test"

	encrypted, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Minimum size: header (33) + ciphertext (at least = plaintext) + GCM tag (16)
	minSize := HeaderLength + len(plaintext) + 16 // GCM tag is 16 bytes
	if len(encrypted) != minSize {
		t.Fatalf("encrypted size = %d, want %d (header %d + plaintext %d + tag 16)",
			len(encrypted), minSize, HeaderLength, len(plaintext))
	}

	// Magic bytes at offset 0-3
	if encrypted[0] != 0x43 || encrypted[1] != 0x50 || encrypted[2] != 0x47 || encrypted[3] != 0x53 {
		t.Fatalf("magic bytes wrong: got %x", encrypted[0:4])
	}

	// Version at offset 4
	if encrypted[4] != 1 {
		t.Fatalf("version = %d, want 1", encrypted[4])
	}

	// Salt at offset 5-20 (16 bytes) — should not be all zeros
	salt := encrypted[5:21]
	allZero := true
	for _, b := range salt {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Fatal("salt is all zeros — random generation likely failed")
	}

	// Nonce at offset 21-32 (12 bytes) — should not be all zeros
	nonce := encrypted[21:33]
	allZero = true
	for _, b := range nonce {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Fatal("nonce is all zeros — random generation likely failed")
	}

	// Ciphertext + tag starts at offset 33
	ciphertextWithTag := encrypted[33:]
	expectedLen := len(plaintext) + 16 // GCM tag
	if len(ciphertextWithTag) != expectedLen {
		t.Fatalf("ciphertext+tag length = %d, want %d", len(ciphertextWithTag), expectedLen)
	}
}
