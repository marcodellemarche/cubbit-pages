# Cubbit Pages

CLI Go per deploy di siti statici su Cubbit S3, con cifratura opzionale AES-256-GCM.

## Architettura
- Binario Go standalone, zero dipendenze runtime
- Cifratura AES-256-GCM con PBKDF2 per derivazione chiave
- Upload diretto su bucket Cubbit dell'utente via aws-sdk-go-v2
- Pagina di login generata automaticamente e iniettata come index.html
- Password memorizzata in sessionStorage lato browser (login/loader page); il service worker usa IndexedDB (scope SW, non accessibile agli script di pagina)
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

# Aggiorna il binario alla release più recente (scarica da GitHub Releases)
cubbit-pages update

# Deploy in chiaro (--clean=true di default: rimuove file S3 non più presenti nella sorgente)
cubbit-pages deploy ./mio-sito --bucket mio-bucket
cubbit-pages deploy ./mio-sito --bucket mio-bucket --clean=false  # disabilita clean

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

# Mostra config corrente + ultimo deploy (da file locale, veloce, funziona offline)
cubbit-pages status

# Mostra inventario completo da S3 (tutti i deploy del bucket, anche da macchina nuova)
cubbit-pages status --deep
cubbit-pages status --deep --bucket mio-bucket --access-key ... --secret-key ...

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
- `last_deploy` salvato in `~/.cubbit/pages/config.yaml` dopo ogni deploy non-dry-run riuscito, anche se `setup` non è mai stato eseguito (crea il file se non esiste)
- Al deploy, `index.html` riceve 5 header S3 `x-amz-meta-cubbit-pages-*`: `encrypted`, `locale`, `version`, `prefix`, `timestamp`
- `status --deep` fa 1 ListObjects + 1 HeadObject per deploy (O(N) API call); raggruppa i file per prefix, legge metadati, mostra inventario ordinato per data; funziona senza config locale passando credenziali via flag
- `status --json` emette JSON strutturato (config + last_deploy + inventory se --deep); utile per CI/CD scripting
- Deploy senza metadati (pre-v0.5.0) mostrano `(no metadata)` con fallback su LastModified da ListObjects
- `deploy --clean` (default: true): dopo l'upload fa ListObjects sul prefix e cancella i file S3 non presenti nella sorgente; disabilitabile con `--clean=false`; skippato in dry-run
- `update`: scarica l'ultima release da GitHub API, sostituisce il binario in-place; richiede permesso di scrittura sulla directory di installazione
- `--region` disponibile su `deploy`, `list`, `delete`, `status --deep` (default: eu-west-1)
- Messaggi di errore contestuali con suggerimento del comando/flag corretto

## File chiave
- `internal/crypto/crypto.go` — logica AES-256-GCM
- `internal/deploy/deploy.go` — orchestrazione deploy; `formatSize()`, `cleanStale()`, `dryRun()`; `Options.{Version,Clean}` e `Result.{FilesRemoved,RemovedFiles}`
- `internal/login/generator.go` — generazione pagina di login e service worker (usa `html/template`)
- `internal/login/locales.go` — struct `Strings`, map `locales` (en/it), `KnownLocales()`, `LocaleStrings()`, `IsKnownLocale()`
- `internal/login/sw.js` — service worker per decryption trasparente di asset .enc
- `internal/s3/upload.go` — upload con gestione ACL; `DeployMeta` struct; metadata S3 iniettati su `index.html`
- `internal/s3/client.go` — client S3, `ListObjects` (paginato), `DeleteObjects` (batch 1000), `HeadBucket`; `DiscoverDeploys()` per status --deep; `BuildSiteURL()`
- `internal/config/config.go` — `Resolve()`, `ResolveOpen()`, `Validate()`, `SiteURL()`; `Config.Clean` e `Config.Region`
- `internal/config/file.go` — load/save `~/.cubbit/pages/config.yaml`; struct `FileConfig` con `LastDeploy`
- `scripts/add-locale.go` — wizard interattivo per aggiungere locale (build tag `ignore`, solo `go run`/`make add-locale`)
- `scripts/test-deploy.sh` — integration test: 10 scenari (6 deploy + 3 status --deep + 1 clean)
- `scripts/verify-decrypt.mjs` — decryption JS (Web Crypto API) per verifica roundtrip
- `CHANGELOG.md` — cronologia release in formato Keep a Changelog

## Convenzioni
- Errori sempre wrappati con `fmt.Errorf("context: %w", err)`
- Nessun `panic` nel codice di produzione
- Funzioni pure in `internal/crypto/` e `internal/login/`
- Test per tutta la logica in `internal/`
- Test interop Go↔JS in `internal/crypto/interop_test.go` (richiede Node.js)
- Costanti cripto (MAGIC, VERSION, SALT_LEN, NONCE_LEN, ITERATIONS) devono coincidere tra Go e tutti i file JS — verificato da test
- `forcePathStyle: true` obbligatorio per Cubbit — non rimuovere mai
