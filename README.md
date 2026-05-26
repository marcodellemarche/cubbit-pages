# Cubbit Pages

CLI to deploy static sites to [Cubbit](https://cubbit.io) S3, with optional AES-256-GCM client-side encryption.

Zero backend. Zero server. Zero npm dependencies.

## How it works

1. You have a folder with a static site (HTML, CSS, JS, images...)
2. Cubbit Pages uploads it to a Cubbit bucket, making it publicly accessible as a website
3. Optionally, all files are encrypted: the only plaintext page is `index.html` (an auto-generated login page)
4. Once the correct password is entered, the browser decrypts every page directly from the bucket

## Installation

### Linux / macOS

```bash
curl -sSL \
  https://github.com/marcodellemarche/cubbit-pages/releases/latest/download/install.sh \
  | bash
```

### Windows (PowerShell)

```powershell
powershell -ExecutionPolicy Bypass -Command "irm https://github.com/marcodellemarche/cubbit-pages/releases/latest/download/install.ps1 | iex"
```

Installs to `%LOCALAPPDATA%\cubbit-pages\` and adds it to the user PATH.
To install a specific version: append `-Version v0.6.2` to the command above.

### Build from source

```bash
git clone https://github.com/marcodellemarche/cubbit-pages.git
cd cubbit-pages
make build
# Binary is in bin/cubbit-pages
```

## Cubbit setup

1. Create an account at [console.cubbit.io](https://console.cubbit.io)
2. Create a bucket
3. Generate an API key from [API Keys](https://console.cubbit.io/api-keys)
4. Run the interactive setup wizard:

```bash
cubbit-pages setup
```

The wizard prompts for Access Key, Secret Key, Endpoint, Bucket, and login page locale. It creates the bucket if it doesn't exist, verifies the connection, then saves everything to `~/.cubbit/pages/config.yaml` so you don't need to repeat credentials on every deploy.

5. Show the bucket configuration snippets:

```bash
cubbit-pages snippets --bucket MY-BUCKET
```

6. For encrypted sites, also configure CORS:

```bash
cubbit-pages snippets --bucket MY-BUCKET --type cors
```

## Usage

### Plain deploy

```bash
# After setup — credentials loaded from ~/.cubbit/pages/config.yaml
cubbit-pages deploy ./my-site

# Or pass credentials explicitly
cubbit-pages deploy ./my-site \
  --bucket my-bucket \
  --access-key AKIAIOSFODNN7EXAMPLE \
  --secret-key wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

### Encrypted deploy

```bash
# Password prompted interactively (recommended)
cubbit-pages deploy ./my-site --encrypt

# Password via environment variable (good for CI)
export CUBBIT_PASSWORD="my-secret-password"
cubbit-pages deploy ./my-site --encrypt

# Password from stdin
echo "my-secret-password" | cubbit-pages deploy ./my-site --encrypt
```

### Preview before deploying

```bash
# Dry run — shows exactly what would be uploaded with real file sizes, no upload
cubbit-pages deploy ./my-site --encrypt --dry-run
```

```
Deploying to s3://my-bucket/
Mode: encrypted (AES-256-GCM)
⚠ Dry run — no files will be uploaded

  index.html                                         10.2 KB  [dry]
  sw.js                                               7.9 KB  [dry]
  _verify.enc                                           64 B  [dry]
  about.html.enc                                       3.9 KB  [dry]
  style.css.enc                                        3.8 KB  [dry]

Deploy complete: 0 file(s) uploaded
URL: https://my-bucket.s3.cubbit.eu/index.html
```

### Multiple profiles

Save credentials for different environments (e.g. staging, production) as named profiles:

```bash
# Create profiles interactively
cubbit-pages setup                        # creates/updates "default" profile
cubbit-pages setup --profile staging
cubbit-pages setup --profile production

# Use a specific profile
cubbit-pages deploy ./my-site --profile production
cubbit-pages status --profile staging
cubbit-pages list --profile production

# Select profile via environment variable
export CUBBIT_PROFILE=production
cubbit-pages deploy ./my-site
```

Profiles are stored in `~/.cubbit/pages/config.yaml`:

```yaml
profiles:
  default:
    access_key: ...
    secret_key: ...
    bucket: my-bucket
    endpoint: https://s3.cubbit.eu
    locale: en
  production:
    access_key: ...
    secret_key: ...
    bucket: prod-bucket
    locale: it
```

Existing config files in the old flat format are migrated automatically on first read — no manual action needed.

You can set a different default profile by adding `default: production` at the top of the config file. This overrides the `default` profile name when no `--profile` flag or `CUBBIT_PROFILE` is given.

### Environment variables

All credentials can be passed via env (overrides config file):

```bash
export CUBBIT_ACCESS_KEY=...
export CUBBIT_SECRET_KEY=...
export CUBBIT_BUCKET=my-bucket
export CUBBIT_PASSWORD=my-secret-password  # for encrypted deploys
export CUBBIT_LOCALE=it                    # login page language
export CUBBIT_PROFILE=production           # select active profile

cubbit-pages deploy ./my-site --encrypt
```

Priority: CLI flags > environment variables > active profile in `~/.cubbit/pages/config.yaml`

### Check current config and last deploy

```bash
cubbit-pages status
```

```
  Config (~/.cubbit/pages/config.yaml)
  ────────────────────────────────────────────
  Bucket:      my-bucket
  Endpoint:    https://s3.cubbit.eu
  Locale:      en

  Last deploy
  ────────────────────────────────────────────
  Bucket:      my-bucket
  Prefix:      weekly/2026-05-11
  Files:       7
  Mode:        encrypted (AES-256-GCM)
  Date:        2026-05-11 14:32
  URL:         https://my-bucket.s3.cubbit.eu/weekly/2026-05-11/index.html
```

### Full bucket inventory (multi-machine, CI/CD)

Add `--deep` to query S3 directly and list all deploys in the bucket — useful on a new machine without a local config file, or to verify CI/CD deploys:

```bash
cubbit-pages status --deep
# credentials loaded from config file, or pass explicitly:
cubbit-pages status --deep --bucket my-bucket --access-key ... --secret-key ...
```

```
  Config (~/.cubbit/pages/config.yaml)
  ────────────────────────────────────────────
  Bucket:      my-bucket
  ...

  Bucket inventory: my-bucket (2 deploy)
  ────────────────────────────────────────────

  #1  demo-backup          7 files   142.3 KB  encrypted   2026-05-11 15:10
      https://my-bucket.s3.cubbit.eu/demo-backup/index.html

  #2  demo                 7 files   142.3 KB  encrypted   2026-05-11 14:32
      https://my-bucket.s3.cubbit.eu/demo/index.html
```

Deploy metadata (encryption mode, locale, CLI version, timestamp) is stored as S3 object metadata on `index.html` at deploy time and read back on demand. Deploys made before v0.5.0 show `(no metadata)`.

The `last_deploy` in `~/.cubbit/pages/config.yaml` is now always written after every successful deploy, even if `setup` was never run.

## GitHub Action

Cubbit Pages ships as a reusable GitHub Action. Drop it into any workflow:

```yaml
- uses: marcodellemarche/cubbit-pages@main
  with:
    source-dir: ./dist
    bucket: my-bucket
    access-key: ${{ secrets.CUBBIT_ACCESS_KEY }}
    secret-key: ${{ secrets.CUBBIT_SECRET_KEY }}
```

### Action inputs

| Input | Required | Default | Description |
|-------|----------|---------|-------------|
| `source-dir` | yes | — | Directory to deploy |
| `bucket` | yes | — | Cubbit S3 bucket name |
| `access-key` | yes | — | Cubbit access key |
| `secret-key` | yes | — | Cubbit secret key |
| `endpoint` | no | `https://s3.cubbit.eu` | S3 endpoint |
| `encrypt` | no | `false` | Enable AES-256-GCM encryption |
| `password` | no | — | Encryption password |
| `public-bucket` | no | `false` | Skip per-object ACL |
| `prefix` | no | — | S3 key prefix |
| `locale` | no | `en` | Login page language (`en`, `it`) |
| `concurrency` | no | `5` | Parallel uploads |
| `version` | no | `latest` | CLI version to download |

### Action output

| Output | Description |
|--------|-------------|
| `url` | URL of the deployed site |

## Encrypted site navigation

When deploying with `--encrypt`:

1. All files are encrypted with AES-256-GCM and uploaded with `.enc` extension
2. A login `index.html` is auto-generated
3. The user's original `index.html` becomes `index.html.enc`
4. A `_verify.enc` canary file validates the password
5. A **service worker** (`sw.js`) is uploaded alongside the login page:
   - After login, the SW is registered and receives the password via `postMessage`
   - It intercepts every browser fetch (scripts, stylesheets, images, fonts, etc.)
   - For each request: if the original resource returns 404, it tries `<url>.enc`, decrypts in-memory, and returns the plaintext with the correct `Content-Type`
   - Decrypted responses are cached for performance
   - Password is persisted to IndexedDB so it survives service worker restarts without requiring re-login
6. For each HTML file, a "loader" page handles direct navigation (e.g., bookmark to `/about.html`)
7. A dark loading overlay (Cubbit colors + spinner) is injected into every decrypted page before `document.write` and dissolves on `window.load`, eliminating the white flash while external CSS is being fetched and decrypted

This means **multi-file sites work out of the box** — SPAs (Vite, React, etc.), sites with separate JS/CSS/images, all work transparently after login.

## CLI reference

### `cubbit-pages deploy <directory>`

| Flag | Description |
|------|-------------|
| `--profile` | Config profile to use (or `CUBBIT_PROFILE`; default: `default`) |
| `--bucket`, `-b` | Bucket name (or `CUBBIT_BUCKET`) |
| `--access-key` | API key (or `CUBBIT_ACCESS_KEY`) |
| `--secret-key` | Secret key (or `CUBBIT_SECRET_KEY`) |
| `--endpoint` | S3 endpoint (default: `https://s3.cubbit.eu`) |
| `--region` | AWS/S3 region (default: `eu-west-1`) |
| `--encrypt`, `-e` | Enable AES-256-GCM encryption |
| `--password`, `-p` | Encryption password (or `CUBBIT_PASSWORD`; omit to be prompted) |
| `--public-bucket` | Assume public bucket policy (skip per-object ACL) |
| `--dry-run` | Show what would be uploaded without uploading |
| `--clean` | Delete S3 files not in source directory after upload (default: `true`; use `--clean=false` to disable) |
| `--concurrency` | Parallel uploads (default: 5) |
| `--prefix` | S3 key prefix for all files |
| `--locale` | Login page language: `en`, `it` (default: `en`, or `CUBBIT_LOCALE`) |

### `cubbit-pages setup`

Interactive wizard. Prompts for Profile name, Access Key, Secret Key, Endpoint, Bucket, and login page locale. Creates the bucket if it doesn't exist. Verifies the connection with `HeadBucket`. Saves the profile to `~/.cubbit/pages/config.yaml` (mode 0600).

| Flag | Description |
|------|-------------|
| `--profile` | Profile name to create or update (or `CUBBIT_PROFILE`; prompted interactively if omitted) |

### `cubbit-pages status`

| Flag | Description |
|------|-------------|
| `--profile` | Config profile to use (or `CUBBIT_PROFILE`; default: `default`) |
| `--deep` | Query S3 for full deploy inventory (requires credentials) |
| `--json` | Output as JSON (machine-readable; works with or without `--deep`) |
| `--bucket`, `-b` | Bucket name (or `CUBBIT_BUCKET`) — needed for `--deep` without config file |
| `--access-key` | API key (or `CUBBIT_ACCESS_KEY`) |
| `--secret-key` | Secret key (or `CUBBIT_SECRET_KEY`) |
| `--endpoint` | S3 endpoint |
| `--region` | AWS/S3 region (default: `eu-west-1`) |

Without `--deep`: reads from `~/.cubbit/pages/config.yaml` (fast, offline). With `--deep`: does 1 ListObjects + 1 HeadObject per deploy found; shows full inventory from S3.

### `cubbit-pages update`

Downloads and installs the latest release in-place:

```bash
cubbit-pages update
```

If the binary is in a system directory (e.g. `/usr/local/bin`), run with `sudo`. On Windows, run as Administrator.

### `cubbit-pages list`

| Flag | Description |
|------|-------------|
| `--profile` | Config profile to use (or `CUBBIT_PROFILE`; default: `default`) |
| `--bucket`, `-b` | Bucket name (or `CUBBIT_BUCKET`) |
| `--access-key` | API key (or `CUBBIT_ACCESS_KEY`) |
| `--secret-key` | Secret key (or `CUBBIT_SECRET_KEY`) |
| `--endpoint` | S3 endpoint |
| `--prefix` | Filter by S3 key prefix |

### `cubbit-pages delete`

| Flag | Description |
|------|-------------|
| `--profile` | Config profile to use (or `CUBBIT_PROFILE`; default: `default`) |
| `--bucket`, `-b` | Bucket name (or `CUBBIT_BUCKET`) |
| `--access-key` | API key (or `CUBBIT_ACCESS_KEY`) |
| `--secret-key` | Secret key (or `CUBBIT_SECRET_KEY`) |
| `--endpoint` | S3 endpoint |
| `--prefix` | Delete only files with this prefix (omitting deletes the entire bucket — a warning is shown) |
| `--yes`, `-y` | Skip confirmation prompt |

Exits with code 1 if the user aborts the confirmation.

### `cubbit-pages open`

| Flag | Description |
|------|-------------|
| `--profile` | Config profile to use (or `CUBBIT_PROFILE`; default: `default`) |
| `--bucket`, `-b` | Bucket name (or `CUBBIT_BUCKET`) |
| `--endpoint` | S3 endpoint |
| `--prefix` | S3 key prefix (defaults to the prefix of the last deploy) |

Opens the deployed site URL in the system browser (`xdg-open` on Linux, `open` on macOS, `start` on Windows). Does not require S3 credentials — only bucket and endpoint are needed to build the URL.

### `cubbit-pages snippets`

| Flag | Description |
|------|-------------|
| `--bucket`, `-b` | Bucket name |
| `--type` | `bucket-policy`, `cors`, `iam`, `lifecycle`, `all` (default: `all`) |

### `cubbit-pages version`

Shows version, commit hash and build date.

### Adding a new locale (contributors only)

```bash
make add-locale LOCALE=fr
```

Interactive wizard that prompts for each UI string and appends the new entry to `internal/login/locales.go`. Run `make test` afterwards to verify all fields are populated.

## Security

- AES-256-GCM encryption with PBKDF2 key derivation (100,000 iterations, SHA-256)
- Random salt and nonce per file (16 bytes and 12 bytes)
- Password never transmitted over the network — decryption happens in the browser
- Canary file (`_verify.enc`) validates the password without downloading large files
- Service worker persists the password in IndexedDB (SW scope only, not accessible to page scripts) so it survives SW restarts; the login and loader pages use `sessionStorage` (cleared on tab close); password is never transmitted or stored on disk
- `sw.js` is the only unencrypted file besides the login page (it contains no secrets)
- S3 credentials are never saved to disk by the CLI

## Related: Cubbit Seal

Cubbit Pages is the companion project to [Cubbit Seal](https://github.com/marcodellemarche/cubbit-seal):

- **Visual style**: same colors, fonts and patterns in the login page
- **Crypto format**: intentionally different (`CPGS` vs `CBSH`) — not interoperable
- **Conventions**: same project structure and code style

## License

[MIT](LICENSE)
