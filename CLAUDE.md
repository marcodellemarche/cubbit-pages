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
- `make add-locale LOCALE=<code>` — wizard interattivo per aggiungere una nuova locale (modifica `internal/login/locales.go`, solo per sviluppatori)

## Uso base
```bash
# Setup interattivo (crea il bucket se non esiste, salva in ~/.cubbit/pages/config.yaml)
cubbit-pages setup

# Deploy in chiaro
cubbit-pages deploy ./mio-sito --bucket mio-bucket

# Deploy cifrato (password da flag)
cubbit-pages deploy ./mio-sito --bucket mio-bucket --encrypt --password "parola-parola-parola"

# Deploy cifrato con login page in italiano
cubbit-pages deploy ./mio-sito --bucket mio-bucket --encrypt --password "parola-parola-parola" --locale it

# Lista file nel bucket
cubbit-pages list --bucket mio-bucket
cubbit-pages list --bucket mio-bucket --prefix weekly/2026-05-11/

# Cancella file dal bucket (chiede conferma)
cubbit-pages delete --bucket mio-bucket --prefix weekly/2026-05-11/

# Apre il sito nel browser
cubbit-pages open --bucket mio-bucket

# Mostra snippet bucket policy
cubbit-pages snippets --bucket mio-bucket
```

## UX post-login
- Dopo login, prima di `document.write(html)` viene iniettato un overlay scuro (`#0a0e17`) con spinner Cubbit
- L'overlay si dissolve su `window.load` (quando tutti i CSS sono pronti) — niente flash bianco
- Logica in `injectLoadingOverlay()` in `internal/login/template.html` e `internal/login/loader.html`

## Demo site cifrata
- `site-demo/` — sito demo (index.html, about.html, style.css) deployato cifrato su bucket `pages`, prefix `demo`
- URL: `https://pages.s3.cubbit.eu/demo/index.html` — password pubblica: `pippofranco`
- Deploy gestito da `.github/workflows/deploy-site.yml` (secondo step, dopo la landing page)

## Variabili d'ambiente
- `CUBBIT_ACCESS_KEY`
- `CUBBIT_SECRET_KEY`
- `CUBBIT_BUCKET`
- `CUBBIT_ENDPOINT` (default: https://s3.cubbit.eu)
- `CUBBIT_LOCALE` (default: en)

## File chiave
- `internal/crypto/crypto.go` — logica AES-256-GCM
- `internal/deploy/deploy.go` — orchestrazione deploy
- `internal/login/generator.go` — generazione pagina di login e service worker (usa `html/template`)
- `internal/login/locales.go` — struct `Strings`, map `locales` (en/it), `KnownLocales()`, `LocaleStrings()`, `IsKnownLocale()`
- `internal/login/sw.js` — service worker per decryption trasparente di asset .enc
- `internal/s3/upload.go` — upload con gestione ACL
- `internal/s3/client.go` — client S3, `ListObjects` (paginato), `DeleteObjects` (batch 1000)
- `internal/config/file.go` — load/save `~/.cubbit/pages/config.yaml` (YAML, 0600)
- `scripts/add-locale.go` — wizard interattivo per aggiungere locale (build tag `ignore`, solo `go run`/`make add-locale`)
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
