# Cubbit Pages

CLI Go per deploy di siti statici su Cubbit S3, con cifratura opzionale AES-256-GCM.

## Architettura
- Binario Go standalone, zero dipendenze runtime
- Cifratura AES-256-GCM con PBKDF2 per derivazione chiave
- Upload diretto su bucket Cubbit dell'utente via aws-sdk-go-v2
- Pagina di login generata automaticamente e iniettata come index.html
- Password memorizzata in localStorage lato browser per evitare re-login ad ogni pagina
- Service worker (`sw.js`) intercetta i fetch e decripta .enc al volo per siti multi-file

## Comandi
- `make build` — build per la piattaforma corrente
- `make release` — cross-compile per tutte le piattaforme target
- `make test` — go test ./...
- `make test-integration` — build + integration test end-to-end (richiede CUBBIT_ACCESS_KEY, CUBBIT_SECRET_KEY, CUBBIT_BUCKET)
- `make test-coverage` — coverage report
- `make lint` — golangci-lint

## Uso base
```bash
# Setup interattivo (crea il bucket se non esiste, salva in ~/.cubbit/pages/config.yaml)
cubbit-pages setup

# Deploy in chiaro
cubbit-pages deploy ./mio-sito --bucket mio-bucket

# Deploy cifrato (password da flag)
cubbit-pages deploy ./mio-sito --bucket mio-bucket --encrypt --password "parola-parola-parola"

# Deploy cifrato (password da stdin)
echo "parola-parola-parola" | cubbit-pages deploy ./mio-sito --bucket mio-bucket --encrypt

# Mostra snippet bucket policy
cubbit-pages snippets --bucket mio-bucket
```

## Variabili d'ambiente
- `CUBBIT_ACCESS_KEY`
- `CUBBIT_SECRET_KEY`
- `CUBBIT_BUCKET`
- `CUBBIT_ENDPOINT` (default: https://s3.cubbit.eu)

## File chiave
- `internal/crypto/crypto.go` — logica AES-256-GCM
- `internal/deploy/deploy.go` — orchestrazione deploy
- `internal/login/generator.go` — generazione pagina di login e service worker
- `internal/login/sw.js` — service worker per decryption trasparente di asset .enc
- `internal/s3/upload.go` — upload con gestione ACL
- `internal/config/file.go` — load/save `~/.cubbit/pages/config.yaml` (YAML, 0600)
- `scripts/test-deploy.sh` — integration test: 6 scenari + verifica decrypt con Node.js
- `scripts/verify-decrypt.mjs` — decryption JS (Web Crypto API) per verifica roundtrip

## Convenzioni
- Errori sempre wrappati con `fmt.Errorf("context: %w", err)`
- Nessun `panic` nel codice di produzione
- Funzioni pure in `internal/crypto/` e `internal/login/`
- Test per tutta la logica in `internal/`
- Test interop Go↔JS in `internal/crypto/interop_test.go` (richiede Node.js)
- Costanti cripto (MAGIC, VERSION, SALT_LEN, NONCE_LEN, ITERATIONS) devono coincidere tra Go e tutti i file JS — verificato da test
- `forcePathStyle: true` obbligatorio per Cubbit — non rimuovere mai
