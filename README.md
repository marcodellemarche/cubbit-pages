# Cubbit Pages

CLI to deploy static sites to [Cubbit](https://cubbit.io) S3, with optional AES-256-GCM client-side encryption.

Zero backend. Zero server. Zero npm dependencies.

## How it works

1. You have a folder with a static site (HTML, CSS, JS, images...)
2. Cubbit Pages uploads it to a Cubbit bucket, making it publicly accessible as a website
3. Optionally, all files are encrypted: the only plaintext page is `index.html` (an auto-generated login page)
4. Once the correct password is entered, the browser decrypts every page directly from the bucket

## Installation

### Automatic script

```bash
curl -sSL \
  https://github.com/marcodellemarche/cubbit-pages/releases/latest/download/install.sh \
  | bash
```

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
4. Show the bucket configuration snippets:

```bash
cubbit-pages snippets --bucket MY-BUCKET
```

5. For encrypted sites, also configure CORS:

```bash
cubbit-pages snippets --bucket MY-BUCKET --type cors
```

## Usage

### Plain deploy

```bash
cubbit-pages deploy ./my-site \
  --bucket my-bucket \
  --access-key AKIAIOSFODNN7EXAMPLE \
  --secret-key wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

### Encrypted deploy

```bash
cubbit-pages deploy ./my-site \
  --bucket my-bucket \
  --access-key ... \
  --secret-key ... \
  --encrypt \
  --password "my-secret-password"
```

If `--password` is not provided with `--encrypt`, the CLI prompts interactively.

### Environment variables

All credentials can be passed via env:

```bash
export CUBBIT_ACCESS_KEY=...
export CUBBIT_SECRET_KEY=...
export CUBBIT_BUCKET=my-bucket

cubbit-pages deploy ./my-site --encrypt --password "..."
```

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
6. For each HTML file, a "loader" page handles direct navigation (e.g., bookmark to `/about.html`)

This means **multi-file sites work out of the box** — SPAs (Vite, React, etc.), sites with separate JS/CSS/images, all work transparently after login.

## CLI reference

### `cubbit-pages deploy <directory>`

| Flag | Description |
|------|-------------|
| `--bucket`, `-b` | Bucket name (or `CUBBIT_BUCKET`) |
| `--access-key` | API key (or `CUBBIT_ACCESS_KEY`) |
| `--secret-key` | Secret key (or `CUBBIT_SECRET_KEY`) |
| `--endpoint` | S3 endpoint (default: `https://s3.cubbit.eu`) |
| `--encrypt`, `-e` | Enable AES-256-GCM encryption |
| `--password`, `-p` | Encryption password |
| `--public-bucket` | Assume public bucket policy (skip per-object ACL) |
| `--dry-run` | Show what would be uploaded without uploading |
| `--concurrency` | Parallel uploads (default: 5) |
| `--prefix` | S3 key prefix for all files |

### `cubbit-pages snippets`

| Flag | Description |
|------|-------------|
| `--bucket`, `-b` | Bucket name |
| `--type` | `bucket-policy`, `cors`, `iam`, `lifecycle`, `all` (default: `all`) |

### `cubbit-pages version`

Shows version, commit hash and build date.

## Security

- AES-256-GCM encryption with PBKDF2 key derivation (100,000 iterations, SHA-256)
- Random salt and nonce per file (16 bytes and 12 bytes)
- Password never transmitted over the network — decryption happens in the browser
- Canary file (`_verify.enc`) validates the password without downloading large files
- Service worker holds the password only in memory (never persisted to disk)
- `sw.js` is the only unencrypted file besides the login page (it contains no secrets)
- S3 credentials are never saved to disk by the CLI

## Related: Cubbit Seal

Cubbit Pages is the companion project to [Cubbit Seal](https://github.com/marcodellemarche/cubbit-seal):

- **Visual style**: same colors, fonts and patterns in the login page
- **Crypto format**: intentionally different (`CPGS` vs `CBSH`) — not interoperable
- **Conventions**: same project structure and code style

## License

[MIT](LICENSE)
