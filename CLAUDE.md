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
# Chiede: access key, secret key, endpoint, bucket, locale
# Alla fine fa HeadBucket per verificare la connessione
cubbit-pages setup

# Deploy in chiaro
cubbit-pages deploy ./mio-sito --bucket mio-bucket

# Deploy cifrato (password da flag, env var, o prompt interattivo)
cubbit-pages deploy ./mio-sito --bucket mio-bucket --encrypt --password "parola"
CUBBIT_PASSWORD="parola" cubbit-pages deploy ./mio-sito --bucket mio-bucket --encrypt

# Deploy con dry-run (stesso formato dell'output reale, con [dry] per ogni file)
cubbit-pages deploy ./mio-sito --bucket mio-bucket --encrypt --password "parola" --dry-run

# Lista file nel bucket
cubbit-pages list --bucket mio-bucket
cubbit-pages list --bucket mio-bucket --prefix weekly/2026-05-11/

# Cancella file dal bucket (chiede conferma; warning esplicito se --prefix assente)
cubbit-pages delete --bucket mio-bucket --prefix weekly/2026-05-11/

# Apre il sito nel browser (usa il prefix dell'ultimo deploy se --prefix non specificato)
cubbit-pages open --bucket mio-bucket

# Mostra config corrente + ultimo deploy
cubbit-pages status

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
- `CUBBIT_PASSWORD` — password cifratura (alternativa a `--password` e al prompt interattivo)

## Comportamenti chiave
- `open` non richiede credenziali (solo bucket ed endpoint); usa il prefix dell'ultimo deploy come default
- `delete` senza `--prefix`: warning su stderr, exit 1 su abort (non 0)
- `list`/`delete`: prefix normalizzato con `strings.Trim` (stessa logica di deploy)
- Output deploy: dimensioni leggibili (KB/MB), ordine deterministico (serializzato dopo `wg.Wait()`)
- `last_deploy` salvato in `~/.cubbit/pages/config.yaml` dopo ogni deploy non-dry-run riuscito
- Messaggi di errore contestuali con suggerimento del comando/flag corretto

## File chiave
- `internal/crypto/crypto.go` — logica AES-256-GCM
- `internal/deploy/deploy.go` — orchestrazione deploy; `formatSize()`, `siteURL()`, `dryRun()`
- `internal/login/generator.go` — generazione pagina di login e service worker (usa `html/template`)
- `internal/login/locales.go` — struct `Strings`, map `locales` (en/it), `KnownLocales()`, `LocaleStrings()`, `IsKnownLocale()`
- `internal/login/sw.js` — service worker per decryption trasparente di asset .enc
- `internal/s3/upload.go` — upload con gestione ACL
- `internal/s3/client.go` — client S3, `ListObjects` (paginato), `DeleteObjects` (batch 1000), `HeadBucket`
- `internal/config/config.go` — `Resolve()`, `ResolveOpen()`, `Validate()`, `SiteURL()`
- `internal/config/file.go` — load/save `~/.cubbit/pages/config.yaml`; struct `FileConfig` con `LastDeploy`
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
