# Cubbit Pages

CLI Go per deploy di siti statici su Cubbit S3, con cifratura opzionale AES-256-GCM.

## Architettura
- Binario Go standalone, zero dipendenze runtime
- Cifratura AES-256-GCM con PBKDF2 per derivazione chiave
- Upload diretto su bucket Cubbit dell'utente via aws-sdk-go-v2
- Pagina di login generata automaticamente e iniettata come index.html
- Password memorizzata in localStorage lato browser per evitare re-login ad ogni pagina

## Comandi
- `make build` — build per la piattaforma corrente
- `make release` — cross-compile per tutte le piattaforme target
- `make test` — go test ./...
- `make test-coverage` — coverage report
- `make lint` — golangci-lint

## Uso base
```bash
# Deploy in chiaro
cubbit-pages deploy ./mio-sito \
  --bucket mio-bucket \
  --access-key AKIAIOSFODNN7EXAMPLE \
  --secret-key wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY

# Deploy cifrato
cubbit-pages deploy ./mio-sito \
  --bucket mio-bucket \
  --access-key ... \
  --secret-key ... \
  --encrypt \
  --password "parola-parola-parola"

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
- `internal/login/generator.go` — generazione pagina di login
- `internal/s3/upload.go` — upload con gestione ACL

## Convenzioni
- Errori sempre wrappati con `fmt.Errorf("context: %w", err)`
- Nessun `panic` nel codice di produzione
- Funzioni pure in `internal/crypto/` e `internal/login/`
- Test per tutta la logica in `internal/`
- `forcePathStyle: true` obbligatorio per Cubbit — non rimuovere mai
