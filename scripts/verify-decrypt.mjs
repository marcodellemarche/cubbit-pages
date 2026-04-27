#!/usr/bin/env node
// Decrypts a .enc file produced by cubbit-pages.
//
// Usage:
//   node verify-decrypt.mjs <enc-file> <password>
//     → verifies it decrypts to the canary string "cubbit-pages-ok"
//
//   node verify-decrypt.mjs <enc-file> <password> --out <out-file>
//     → decrypts and writes plaintext to <out-file>, exits 0 on success
//
//   node verify-decrypt.mjs <enc-file> <password> --compare <plain-file>
//     → decrypts and byte-compares with <plain-file>, exits 0 if equal
//
// Exit 0 = OK, exit 1 = failed, exit 2 = usage error.
import { webcrypto } from 'node:crypto';
import { readFileSync, writeFileSync } from 'node:fs';

const { subtle } = webcrypto;

const MAGIC     = [0x43, 0x50, 0x47, 0x53]; // CPGS
const VERSION   = 1;
const SALT_LEN  = 16;
const NONCE_LEN = 12;
const HEADER_LEN = 4 + 1 + SALT_LEN + NONCE_LEN; // 33
const ITERATIONS = 100_000;
const CANARY = 'cubbit-pages-ok';

async function decrypt(data, password) {
  if (data.length < HEADER_LEN) throw new Error('file too short');
  for (let i = 0; i < 4; i++) {
    if (data[i] !== MAGIC[i]) throw new Error(`bad magic at byte ${i}: 0x${data[i].toString(16)}`);
  }
  if (data[4] !== VERSION) throw new Error(`unsupported version: ${data[4]}`);

  let off = 5;
  const salt  = data.slice(off, off + SALT_LEN);  off += SALT_LEN;
  const nonce = data.slice(off, off + NONCE_LEN); off += NONCE_LEN;
  const ct    = data.slice(off);

  const km = await subtle.importKey(
    'raw', new TextEncoder().encode(password), 'PBKDF2', false, ['deriveKey']
  );
  const key = await subtle.deriveKey(
    { name: 'PBKDF2', salt, iterations: ITERATIONS, hash: 'SHA-256' },
    km,
    { name: 'AES-GCM', length: 256 },
    false,
    ['decrypt']
  );
  const plain = await subtle.decrypt({ name: 'AES-GCM', iv: nonce, tagLength: 128 }, key, ct);
  return Buffer.from(plain);
}

const args = process.argv.slice(2);
const encFile  = args[0];
const password = args[1];

if (!encFile || !password) {
  console.error('Usage: verify-decrypt.mjs <enc-file> <password> [--out <file>|--compare <file>]');
  process.exit(2);
}

const mode    = args[2]; // '--out', '--compare', or undefined
const modeArg = args[3];

const data = readFileSync(encFile);

decrypt(data, password)
  .then(plaintext => {
    if (mode === '--out') {
      writeFileSync(modeArg, plaintext);
    } else if (mode === '--compare') {
      const expected = readFileSync(modeArg);
      if (!plaintext.equals(expected)) {
        console.error(`content mismatch: decrypted ${plaintext.length}B vs expected ${expected.length}B`);
        process.exit(1);
      }
    } else {
      // Default: verify canary
      const text = plaintext.toString('utf-8');
      if (text !== CANARY) {
        console.error(`canary mismatch\n  got:  ${JSON.stringify(text)}\n  want: ${JSON.stringify(CANARY)}`);
        process.exit(1);
      }
    }
    process.exit(0);
  })
  .catch(err => {
    console.error(`decryption failed: ${err.message}`);
    process.exit(1);
  });
