# Changelog

All notable changes to cubbit-pages are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added
- `deploy --clean` (default on): deletes S3 files not present in the source directory after upload — eliminates stale content from previous deploys. Use `--clean=false` to disable.
- `deploy --region`: override the AWS/S3 region (default: `eu-west-1`). Also available on `list`, `delete`, `status --deep`.
- `status --json`: machine-readable JSON output including config, last deploy, and (with `--deep`) the full bucket inventory.
- `update` command: downloads and replaces the binary in-place from the latest GitHub release. Checks current version, skips if already up to date.

### Fixed
- `last_deploy` was silently skipped when no config file existed (i.e. `setup` was never run). Now always written after every successful non-dry-run deploy.

### Refactored
- `siteURL` (deploy.go) and `buildSiteURL` (client.go) consolidated into the exported `s3.BuildSiteURL`.

---

## [0.5.0] — 2026-05-11

### Added
- `status --deep`: queries S3 directly via `ListObjects` + `HeadObject` per deploy entry point. Shows a full inventory of all deploys in the bucket — useful on machines without a local config file and in CI/CD pipelines.
- S3 deploy metadata: `index.html` now receives five `x-amz-meta-cubbit-pages-*` headers at deploy time (`encrypted`, `locale`, `version`, `prefix`, `timestamp`). Read back by `status --deep`. Deploys made before v0.5.0 show `(no metadata)` with a `LastModified` fallback.

### Fixed
- `last_deploy` was silently dropped when `~/.cubbit/pages/config.yaml` did not exist yet (e.g. after a direct `deploy` without `setup`).

---

## [0.4.0] — 2026-05-10

### Added
- Unified dry-run output format: same layout as a real deploy, with `[dry]` marker and real file sizes for each entry.
- Connection test at the end of `setup`: `HeadBucket` is called after saving credentials to confirm the bucket is reachable.

### Changed
- Deploy output uses human-readable sizes (`KB`, `MB`) instead of raw byte counts.
- Output order is deterministic (serialized after `wg.Wait()`, not per-goroutine).

---

## [0.3.9] — 2026-05-09

### Added
- `status` command: shows current config file contents and metadata about the last successful deploy.
- `last_deploy` persistence: after every successful non-dry-run deploy the bucket, prefix, URL, file count, encryption mode, and timestamp are saved to `~/.cubbit/pages/config.yaml`.
- `open` command falls back to the prefix of the last deploy when `--prefix` is not specified.

---

## [0.3.8] — 2026-05-08

### Added
- `delete` without `--prefix` prints an explicit warning to stderr before prompting for confirmation.
- `setup` saves the login page locale to the config file.
- `CUBBIT_PASSWORD` environment variable as an alternative to `--password` and the interactive prompt.
- Contextual error messages with a suggestion of the correct command or flag when required config is missing.

---

## [0.3.7] — 2026-05-07

### Added
- `open` command does not require S3 credentials — only bucket and endpoint are needed to build the URL.

### Changed
- `list` and `delete` normalize the prefix with `strings.Trim` (same logic as `deploy`), preventing double-slash keys.
- Deploy output order is deterministic.

### Fixed
- `delete` exits with code 1 on user abort (was 0).

---

## [0.3.6] — 2026-05-06

### Fixed
- Deploy output URL was printed in path-style format even when virtual-hosted style was available.

---

## [0.3.5] — 2026-05-05

### Added
- Loading overlay injected after successful login: dark Cubbit-colored background + spinner dissolves on `window.load`, eliminating the white flash while external CSS is being fetched and decrypted.

---

## [0.3.4] — 2026-05-04

### Added
- Virtual-hosted-style URL (`https://bucket.s3.cubbit.eu/...`) with automatic path-style fallback when an explicit port is present (e.g. local MinIO).
- `open` command: opens the deployed site in the system browser.
- `locale` field in config file; `CUBBIT_LOCALE` environment variable.

---

## [0.3.2] — 2026-05-02

### Added
- `list` command: lists objects in a bucket, with optional `--prefix` filter.
- `delete` command: deletes objects with confirmation prompt; `--yes` to skip.

---

## [0.3.1] — 2026-05-01

### Added
- End-to-end integration test script (`scripts/test-deploy.sh`): 6 deploy scenarios × plaintext/encrypted, with Node.js roundtrip decryption verify.

---

## [0.3.0] — 2026-04-30

### Added
- `setup` wizard: interactive prompts for credentials, endpoint, bucket (creates if not existing), and locale. Saves to `~/.cubbit/pages/config.yaml` (mode 0600).
- Service worker (`sw.js`) for transparent multi-file decryption: intercepts fetches, decrypts `.enc` assets in-memory, caches responses.
- Password persisted in IndexedDB (Service Worker scope) to survive SW restarts without re-login.

---

## [0.2.x] and earlier

Initial versions: basic deploy (plain and encrypted), single-file sites, no setup wizard.
