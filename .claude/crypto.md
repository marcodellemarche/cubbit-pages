# Implementazione Crypto

## Stack Go
- `crypto/aes` + `crypto/cipher` dalla standard library
- `golang.org/x/crypto/pbkdf2` per derivazione chiave
- `crypto/rand` per generazione IV e salt

## Derivazione chiave (PBKDF2)
```
salt:       16 byte random (crypto/rand)
iterations: 100_000
hash:       sha256
keylen:     32 byte (256 bit)
```

## Cifratura (AES-256-GCM)
```
nonce: 12 byte random (crypto/rand)
tag:   16 byte (GCM default)
```

## Formato file cifrato (.enc)
```
[4 byte magic "CPGS"] [1 byte version=1] [16 byte salt] [12 byte nonce] [N byte ciphertext+tag]
```
Magic diverso da Cubbit Seal ("CBSH") per distinguere i formati.

## Canary file (_verify.enc)
Contiene la stringa `"cubbit-pages-ok"` cifrata con la stessa password.
Usato dalla pagina di login per verificare la password prima di salvarla
in localStorage, senza dover scaricare file di contenuto.

## Decifratura lato browser (Web Crypto API)
Il JS della pagina di login e dei loader usa la stessa logica:
- `window.crypto.subtle` per AES-GCM
- PBKDF2 per derivazione chiave
- Parsing dell'header binario per estrarre salt e nonce
