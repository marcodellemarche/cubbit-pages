package deploy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marcodellemarche/cubbit-pages/internal/crypto"
)

func createTestSite(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	files := map[string]string{
		"index.html":        "<html><body>Home</body></html>",
		"about.html":        "<html><body>About</body></html>",
		"css/style.css":     "body { color: red; }",
		"js/app.js":         "console.log('hello');",
		"images/logo.png":   "fakepng",
		".gitignore":        "node_modules",
		".DS_Store":         "",
		"sub/page.html":     "<html><body>Sub</body></html>",
	}

	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	return dir
}

func TestWalkDirFindsAllFiles(t *testing.T) {
	dir := createTestSite(t)
	files, err := WalkDir(dir)
	if err != nil {
		t.Fatalf("WalkDir failed: %v", err)
	}

	// Should find: index.html, about.html, css/style.css, js/app.js, images/logo.png, sub/page.html
	// Should NOT find: .gitignore, .DS_Store
	if len(files) != 6 {
		names := make([]string, len(files))
		for i, f := range files {
			names[i] = f.RelPath
		}
		t.Fatalf("expected 6 files, got %d: %v", len(files), names)
	}
}

func TestWalkDirSkipsHiddenFiles(t *testing.T) {
	dir := createTestSite(t)
	files, err := WalkDir(dir)
	if err != nil {
		t.Fatalf("WalkDir failed: %v", err)
	}

	for _, f := range files {
		if strings.HasPrefix(filepath.Base(f.RelPath), ".") {
			t.Fatalf("hidden file not skipped: %s", f.RelPath)
		}
	}
}

func TestDryRunPlainDeploy(t *testing.T) {
	dir := createTestSite(t)
	files, err := WalkDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	opts := Options{
		SourceDir: dir,
		Endpoint:  "https://s3.cubbit.eu",
		Bucket:    "test-bucket",
		DryRun:    true,
	}

	result, err := dryRun(files, opts)
	if err != nil {
		t.Fatalf("dryRun failed: %v", err)
	}

	if result.FilesUploaded != 0 {
		t.Fatalf("dry run should upload 0 files, got %d", result.FilesUploaded)
	}

	if len(result.Files) != 6 {
		t.Fatalf("expected 6 files in dry run, got %d", len(result.Files))
	}
}

func TestDryRunEncryptedDeploy(t *testing.T) {
	dir := createTestSite(t)
	files, err := WalkDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	opts := Options{
		SourceDir: dir,
		Endpoint:  "https://s3.cubbit.eu",
		Bucket:    "test-bucket",
		Encrypt:   true,
		Password:  "test-password",
		DryRun:    true,
	}

	result, err := dryRun(files, opts)
	if err != nil {
		t.Fatalf("dryRun failed: %v", err)
	}

	// Build a set of result files for easy lookup
	fileSet := make(map[string]bool)
	for _, f := range result.Files {
		fileSet[f] = true
	}

	// Must have: index.html (login page), sw.js, _verify.enc
	for _, required := range []string{"index.html", "sw.js", "_verify.enc"} {
		if !fileSet[required] {
			t.Fatalf("encrypted deploy missing %s", required)
		}
	}

	// Every source file must have a .enc version
	for _, f := range files {
		if !fileSet[f.RelPath+".enc"] {
			t.Fatalf("missing .enc for %s", f.RelPath)
		}
	}

	// HTML files (except index.html) must have loaders
	for _, f := range files {
		if isHTMLFile(f.RelPath) && f.RelPath != "index.html" {
			if !fileSet[f.RelPath] {
				t.Fatalf("missing loader for HTML file %s", f.RelPath)
			}
		}
	}

	// Verify exact total count:
	// 3 generated (index.html, sw.js, _verify.enc)
	// + 6 .enc files (one per source file)
	// + 2 loaders (about.html, sub/page.html)
	// = 11
	expected := 3 + len(files) + 2 // 2 non-index HTML loaders
	if len(result.Files) != expected {
		t.Fatalf("expected %d files in encrypted dry run, got %d: %v", expected, len(result.Files), result.Files)
	}
}

func TestPlainDeployWithMockUploader(t *testing.T) {
	dir := createTestSite(t)
	files, err := WalkDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	uploaded := make(map[string][]byte)
	mockUpload := func(ctx context.Context, key string, data []byte) error {
		uploaded[key] = data
		return nil
	}

	opts := Options{
		SourceDir:   dir,
		Endpoint:    "https://s3.cubbit.eu",
		Bucket:      "test-bucket",
		Concurrency: 1,
	}

	result, err := runWithUploader(context.Background(), files, opts, mockUpload)
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}

	if result.FilesUploaded != 6 {
		t.Fatalf("expected 6 uploads, got %d", result.FilesUploaded)
	}

	// Check correct paths
	expectedPaths := []string{"index.html", "about.html", "css/style.css", "js/app.js", "images/logo.png", "sub/page.html"}
	for _, p := range expectedPaths {
		if _, ok := uploaded[p]; !ok {
			t.Fatalf("missing upload for %s", p)
		}
	}

	// Plain deploy must NOT have sw.js or _verify.enc
	if _, ok := uploaded["sw.js"]; ok {
		t.Fatal("plain deploy should not include sw.js")
	}
	if _, ok := uploaded["_verify.enc"]; ok {
		t.Fatal("plain deploy should not include _verify.enc")
	}
}

func TestEncryptedDeployWithMockUploader(t *testing.T) {
	dir := createTestSite(t)
	files, err := WalkDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	uploaded := make(map[string][]byte)
	mockUpload := func(ctx context.Context, key string, data []byte) error {
		uploaded[key] = data
		return nil
	}

	opts := Options{
		SourceDir:   dir,
		Endpoint:    "https://s3.cubbit.eu",
		Bucket:      "test-bucket",
		Encrypt:     true,
		Password:    "test-password",
		Concurrency: 1,
	}

	result, err := runWithUploader(context.Background(), files, opts, mockUpload)
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}

	// --- sw.js must be uploaded and must be plain JS (NOT encrypted) ---
	swData, ok := uploaded["sw.js"]
	if !ok {
		t.Fatal("missing sw.js in encrypted deploy")
	}
	swStr := string(swData)
	if !strings.Contains(swStr, "SET_PASSWORD") {
		t.Fatal("sw.js content is wrong — missing SET_PASSWORD handler")
	}
	// sw.js must NOT be encrypted (must NOT start with CPGS magic header)
	if len(swData) >= 4 && swData[0] == 0x43 && swData[1] == 0x50 && swData[2] == 0x47 && swData[3] == 0x53 {
		t.Fatal("sw.js must NOT be encrypted — it starts with CPGS magic header")
	}

	// --- _verify.enc must be uploaded and must be encrypted ---
	verifyData, ok := uploaded["_verify.enc"]
	if !ok {
		t.Fatal("missing _verify.enc")
	}
	if len(verifyData) < 4 || verifyData[0] != 0x43 || verifyData[1] != 0x50 || verifyData[2] != 0x47 || verifyData[3] != 0x53 {
		t.Fatal("_verify.enc must start with CPGS magic header")
	}
	// _verify.enc must decrypt to the canary plaintext
	canaryPlain, err := crypto.Decrypt(verifyData, opts.Password)
	if err != nil {
		t.Fatalf("_verify.enc decryption failed: %v", err)
	}
	if string(canaryPlain) != crypto.CanaryPlaintext {
		t.Fatalf("_verify.enc decrypted to %q, want %q", string(canaryPlain), crypto.CanaryPlaintext)
	}

	// --- index.html must be the login page (NOT encrypted, NOT original) ---
	loginData, ok := uploaded["index.html"]
	if !ok {
		t.Fatal("missing login index.html")
	}
	loginStr := string(loginData)
	if !strings.Contains(loginStr, "login-form") {
		t.Fatal("index.html must be the login page")
	}
	if !strings.Contains(loginStr, "ensureServiceWorker") {
		t.Fatal("index.html login page must contain service worker registration")
	}

	// --- All source files must have .enc versions that are actually encrypted ---
	for _, f := range files {
		encKey := f.RelPath + ".enc"
		encData, ok := uploaded[encKey]
		if !ok {
			t.Fatalf("missing encrypted file: %s", encKey)
		}
		// Every .enc file must start with CPGS magic
		if len(encData) < 4 || encData[0] != 0x43 || encData[1] != 0x50 || encData[2] != 0x47 || encData[3] != 0x53 {
			t.Fatalf("%s must start with CPGS magic header — file is not actually encrypted", encKey)
		}
	}

	// --- HTML loader pages must exist for non-index HTML files ---
	for _, f := range files {
		if !isHTMLFile(f.RelPath) || f.RelPath == "index.html" {
			continue
		}
		loaderData, ok := uploaded[f.RelPath]
		if !ok {
			t.Fatalf("missing loader for HTML file %s", f.RelPath)
		}
		loaderStr := string(loaderData)
		// Loader must reference the .enc file
		if !strings.Contains(loaderStr, f.RelPath+".enc") {
			t.Fatalf("loader for %s missing reference to %s.enc", f.RelPath, f.RelPath)
		}
		// Loader must register SW
		if !strings.Contains(loaderStr, "ensureServiceWorker") {
			t.Fatalf("loader for %s missing service worker registration", f.RelPath)
		}
	}

	// --- Non-HTML files must NOT have loaders (only .enc versions) ---
	nonHTMLFiles := []string{"css/style.css", "js/app.js", "images/logo.png"}
	for _, f := range nonHTMLFiles {
		// The plain version must NOT exist (only .enc)
		if data, ok := uploaded[f]; ok {
			// If it exists, it should not be there
			_ = data
			t.Fatalf("non-HTML file %s should not be uploaded in plain form during encrypted deploy", f)
		}
	}

	// --- Verify exact file count ---
	// 3 generated (index.html, sw.js, _verify.enc)
	// + 6 .enc files
	// + 2 loaders (about.html, sub/page.html)
	// = 11
	expectedCount := 3 + len(files) + 2
	if result.FilesUploaded != expectedCount {
		t.Fatalf("expected exactly %d uploads, got %d", expectedCount, result.FilesUploaded)
	}
	if len(uploaded) != expectedCount {
		keys := make([]string, 0, len(uploaded))
		for k := range uploaded {
			keys = append(keys, k)
		}
		t.Fatalf("expected exactly %d unique uploads, got %d: %v", expectedCount, len(uploaded), keys)
	}
}

// TestEncryptedFilesAreDecryptable verifies that .enc files produced by the deploy
// can be decrypted back to their original content using the same password.
func TestEncryptedFilesAreDecryptable(t *testing.T) {
	dir := createTestSite(t)
	files, err := WalkDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	uploaded := make(map[string][]byte)
	mockUpload := func(ctx context.Context, key string, data []byte) error {
		uploaded[key] = data
		return nil
	}

	password := "test-password-123"
	opts := Options{
		SourceDir:   dir,
		Endpoint:    "https://s3.cubbit.eu",
		Bucket:      "test-bucket",
		Encrypt:     true,
		Password:    password,
		Concurrency: 1,
	}

	_, err = runWithUploader(context.Background(), files, opts, mockUpload)
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}

	// For each source file, verify the .enc version can be decrypted to the original
	for _, f := range files {
		original, err := os.ReadFile(f.AbsPath)
		if err != nil {
			t.Fatalf("reading original %s: %v", f.RelPath, err)
		}

		encData, ok := uploaded[f.RelPath+".enc"]
		if !ok {
			t.Fatalf("missing .enc for %s", f.RelPath)
		}

		// Decrypt using the crypto package (same as what the JS would do)
		// Import here to avoid circular deps — we test via the actual encrypt/decrypt path
		decrypted, err := decryptTestData(encData, password)
		if err != nil {
			t.Fatalf("decrypting %s.enc failed: %v", f.RelPath, err)
		}

		if string(decrypted) != string(original) {
			t.Fatalf("%s.enc decrypted content doesn't match original.\nGot: %q\nWant: %q",
				f.RelPath, string(decrypted), string(original))
		}
	}
}

// decryptTestData decrypts using the real crypto package to verify
// that .enc files uploaded during deploy are actually valid and decryptable.
func decryptTestData(data []byte, password string) ([]byte, error) {
	return crypto.Decrypt(data, password)
}

func TestWalkDirUsesForwardSlashes(t *testing.T) {
	dir := createTestSite(t)
	files, err := WalkDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range files {
		if strings.Contains(f.RelPath, "\\") {
			t.Fatalf("path contains backslash: %s", f.RelPath)
		}
	}
}
