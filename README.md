# Cubbit Pages

CLI per il deploy di siti statici su [Cubbit](https://cubbit.io) S3, con cifratura opzionale AES-256-GCM lato client.

Zero backend. Zero server. Zero dipendenze npm.

## Come funziona

1. Hai una cartella con un sito statico (HTML, CSS, JS, immagini...)
2. Cubbit Pages la carica su un bucket Cubbit, rendendola accessibile come sito web
3. Opzionalmente, tutti i file vengono cifrati: l'unica pagina in chiaro è `index.html` (pagina di login generata automaticamente)
4. Inserita la password corretta, il browser decifra ogni pagina direttamente dal bucket

## Installazione

### Script automatico

```bash
curl -sSL https://s3.cubbit.eu/cubbit-pages-releases/install.sh | bash
```

### Build da sorgente

```bash
git clone https://github.com/marcodellemarche/cubbit-pages.git
cd cubbit-pages
make build
# Il binario è in bin/cubbit-pages
```

## Setup Cubbit

1. Crea un account su [console.cubbit.io](https://console.cubbit.io)
2. Crea un bucket
3. Genera una API key da [API Keys](https://console.cubbit.io/api-keys)
4. Applica la bucket policy per accesso pubblico:

```bash
cubbit-pages snippets --bucket MIO-BUCKET --type bucket-policy
```

5. Per siti cifrati, configura anche CORS:

```bash
cubbit-pages snippets --bucket MIO-BUCKET --type cors
```

## Uso

### Deploy in chiaro

```bash
cubbit-pages deploy ./mio-sito \
  --bucket mio-bucket \
  --access-key AKIAIOSFODNN7EXAMPLE \
  --secret-key wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

### Deploy cifrato

```bash
cubbit-pages deploy ./mio-sito \
  --bucket mio-bucket \
  --access-key ... \
  --secret-key ... \
  --encrypt \
  --password "parola-parola-parola"
```

Se `--password` non è fornita con `--encrypt`, la CLI la chiede interattivamente.

### Variabili d'ambiente

Tutte le credenziali possono essere passate via env:

```bash
export CUBBIT_ACCESS_KEY=...
export CUBBIT_SECRET_KEY=...
export CUBBIT_BUCKET=mio-bucket

cubbit-pages deploy ./mio-sito --encrypt --password "..."
```

## Navigazione su sito cifrato

Quando si fa un deploy cifrato:

1. Tutti i file vengono cifrati con AES-256-GCM e caricati con estensione `.enc`
2. Un `index.html` di login viene generato automaticamente
3. L'`index.html` originale dell'utente diventa `index.html.enc`
4. Un file `_verify.enc` (canary) permette di validare la password
5. Per ogni file HTML, viene creato un "loader" che:
   - Controlla se la password è in localStorage
   - Se assente, redirect alla pagina di login
   - Se presente, scarica il `.enc`, decifra in memoria, renderizza

L'URL nel browser rimane `index.html` durante la navigazione (limitazione v1).

## Reference CLI

### `cubbit-pages deploy <cartella>`

| Flag | Descrizione |
|------|-------------|
| `--bucket`, `-b` | Nome bucket (o `CUBBIT_BUCKET`) |
| `--access-key` | API key (o `CUBBIT_ACCESS_KEY`) |
| `--secret-key` | Secret key (o `CUBBIT_SECRET_KEY`) |
| `--endpoint` | Endpoint S3 (default: `https://s3.cubbit.eu`) |
| `--encrypt`, `-e` | Abilita cifratura AES-256-GCM |
| `--password`, `-p` | Password per cifratura |
| `--public-bucket` | Assume bucket policy pubblica (no ACL per-oggetto) |
| `--dry-run` | Mostra cosa verrebbe caricato senza farlo |
| `--concurrency` | Upload paralleli (default: 5) |
| `--prefix` | Prefisso S3 per tutti i file |

### `cubbit-pages snippets`

| Flag | Descrizione |
|------|-------------|
| `--bucket`, `-b` | Nome bucket |
| `--type` | `bucket-policy`, `cors`, `iam`, `lifecycle`, `all` (default: `all`) |

### `cubbit-pages version`

Mostra versione, commit hash e data di build.

## Sicurezza

- Cifratura AES-256-GCM con chiave derivata via PBKDF2 (100.000 iterazioni, SHA-256)
- Salt e nonce random per ogni file (16 byte e 12 byte)
- La password non viene mai trasmessa in rete — la decifratura avviene nel browser
- Il file canary (`_verify.enc`) permette di validare la password senza scaricare file grandi
- Le credenziali S3 non vengono mai salvate su disco dalla CLI

## Relazione con Cubbit Seal

Cubbit Pages è il progetto companion di [Cubbit Seal](https://github.com/marcodellemarche/cubbit-seal):

- **Stile visivo**: stessi colori, font e motivi della pagina di login
- **Formato crypto**: diverso intenzionalmente (`CPGS` vs `CBSH`) — non sono interoperabili
- **Convenzioni**: stessa struttura di progetto e stile di codice
