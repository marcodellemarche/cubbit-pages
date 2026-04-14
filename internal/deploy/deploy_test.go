package deploy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

	// Check _verify.enc is present
	hasVerify := false
	for _, f := range result.Files {
		if f == "_verify.enc" {
			hasVerify = true
		}
	}
	if !hasVerify {
		t.Fatal("encrypted deploy missing _verify.enc")
	}

	// Check login page (index.html)
	hasLogin := false
	for _, f := range result.Files {
		if f == "index.html" {
			hasLogin = true
		}
	}
	if !hasLogin {
		t.Fatal("encrypted deploy missing login index.html")
	}

	// Check all source files have .enc versions
	for _, f := range files {
		hasEnc := false
		for _, rf := range result.Files {
			if rf == f.RelPath+".enc" {
				hasEnc = true
				break
			}
		}
		if !hasEnc {
			t.Fatalf("missing .enc for %s", f.RelPath)
		}
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

	// Check _verify.enc
	if _, ok := uploaded["_verify.enc"]; !ok {
		t.Fatal("missing _verify.enc")
	}

	// Check login page
	if data, ok := uploaded["index.html"]; !ok {
		t.Fatal("missing login index.html")
	} else {
		html := string(data)
		if !strings.Contains(html, "login-form") {
			t.Fatal("index.html is not the login page")
		}
	}

	// Check all .enc files
	for _, f := range files {
		encKey := f.RelPath + ".enc"
		if _, ok := uploaded[encKey]; !ok {
			t.Fatalf("missing encrypted file: %s", encKey)
		}
	}

	// Check HTML loader pages exist for HTML files
	htmlFiles := []string{"index.html", "about.html", "sub/page.html"}
	for _, h := range htmlFiles {
		if data, ok := uploaded[h]; ok {
			html := string(data)
			// index.html is the login page, others are loaders
			if h != "index.html" && !strings.Contains(html, ".enc") {
				t.Fatalf("loader for %s missing .enc reference", h)
			}
		}
	}

	// Verify total uploaded count
	if result.FilesUploaded < len(files) {
		t.Fatalf("expected at least %d uploads, got %d", len(files), result.FilesUploaded)
	}
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
