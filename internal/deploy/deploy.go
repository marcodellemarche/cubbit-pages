package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/marcodellemarche/cubbit-pages/internal/crypto"
	"github.com/marcodellemarche/cubbit-pages/internal/login"
	s3client "github.com/marcodellemarche/cubbit-pages/internal/s3"
)

// Options configures a deploy operation.
type Options struct {
	SourceDir    string
	Bucket       string
	Endpoint     string
	AccessKey    string
	SecretKey    string
	Region       string
	Encrypt      bool
	Password     string
	PublicBucket bool
	DryRun       bool
	Concurrency  int
	Prefix       string
}

// Result holds the result of a deploy operation.
type Result struct {
	FilesUploaded int
	SiteURL       string
	Files         []string
}

// UploadFunc is the function signature for uploading a file.
// Used to allow mocking in tests.
type UploadFunc func(ctx context.Context, key string, data []byte) error

// Run executes the deploy operation.
func Run(ctx context.Context, opts Options) (*Result, error) {
	files, err := WalkDir(opts.SourceDir)
	if err != nil {
		return nil, fmt.Errorf("walking source directory: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files found in %s", opts.SourceDir)
	}

	if opts.DryRun {
		return dryRun(files, opts)
	}

	client, err := s3client.NewClient(opts.Endpoint, opts.AccessKey, opts.SecretKey, opts.Region, opts.Bucket)
	if err != nil {
		return nil, fmt.Errorf("creating S3 client: %w", err)
	}

	uploader := s3client.NewUploader(client, !opts.PublicBucket, opts.Prefix)
	uploadFn := uploader.Upload

	return runWithUploader(ctx, files, opts, uploadFn)
}

// runWithUploader executes the deploy with a given upload function (for testability).
func runWithUploader(ctx context.Context, files []FileEntry, opts Options, uploadFn UploadFunc) (*Result, error) {
	type uploadItem struct {
		key  string
		data []byte
	}

	var items []uploadItem

	if opts.Encrypt {
		// Generate login page
		loginHTML := login.GenerateLoginPage()
		items = append(items, uploadItem{key: "index.html", data: []byte(loginHTML)})

		// Generate service worker (must be unencrypted and at scope root)
		swJS := login.GenerateServiceWorker()
		items = append(items, uploadItem{key: "sw.js", data: []byte(swJS)})

		// Generate canary file
		canary, err := crypto.EncryptCanary(opts.Password)
		if err != nil {
			return nil, fmt.Errorf("creating canary: %w", err)
		}
		items = append(items, uploadItem{key: "_verify.enc", data: canary})

		// Process each file
		for _, f := range files {
			data, err := os.ReadFile(f.AbsPath)
			if err != nil {
				return nil, fmt.Errorf("reading %s: %w", f.RelPath, err)
			}

			// Encrypt the file
			encrypted, err := crypto.Encrypt(data, opts.Password)
			if err != nil {
				return nil, fmt.Errorf("encrypting %s: %w", f.RelPath, err)
			}
			encKey := f.RelPath + ".enc"
			items = append(items, uploadItem{key: encKey, data: encrypted})

			// For HTML files (except index.html which is replaced by login page),
			// create a loader page that fetches and decrypts the .enc version
			if isHTMLFile(f.RelPath) && f.RelPath != "index.html" {
				loader := login.GenerateLoader(encKey)
				items = append(items, uploadItem{key: f.RelPath, data: []byte(loader)})
			}
		}
	} else {
		// Plain deploy: upload files as-is
		for _, f := range files {
			data, err := os.ReadFile(f.AbsPath)
			if err != nil {
				return nil, fmt.Errorf("reading %s: %w", f.RelPath, err)
			}
			items = append(items, uploadItem{key: f.RelPath, data: data})
		}
	}

	// Upload with concurrency
	var (
		uploaded int64
		errs     []error
		mu       sync.Mutex
		sem      = make(chan struct{}, opts.Concurrency)
		wg       sync.WaitGroup
	)

	var uploadedFiles []string

	for _, item := range items {
		sem <- struct{}{}
		wg.Add(1)
		go func(key string, data []byte) {
			defer wg.Done()
			defer func() { <-sem }()

			fmt.Printf("  %-50s %d bytes\n", key, len(data))

			if err := uploadFn(ctx, key, data); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("%s: %w", key, err))
				mu.Unlock()
				return
			}

			atomic.AddInt64(&uploaded, 1)
			mu.Lock()
			uploadedFiles = append(uploadedFiles, key)
			mu.Unlock()
		}(item.key, item.data)
	}

	wg.Wait()

	if len(errs) > 0 {
		return nil, fmt.Errorf("upload errors: %v", errs)
	}

	prefix := ""
	if opts.Prefix != "" {
		prefix = opts.Prefix + "/"
	}
	siteURL := fmt.Sprintf("%s/%s/%sindex.html", opts.Endpoint, opts.Bucket, prefix)

	return &Result{
		FilesUploaded: int(uploaded),
		SiteURL:       siteURL,
		Files:         uploadedFiles,
	}, nil
}

// dryRun simulates a deploy without uploading.
func dryRun(files []FileEntry, opts Options) (*Result, error) {
	var resultFiles []string

	if opts.Encrypt {
		resultFiles = append(resultFiles, "index.html")
		resultFiles = append(resultFiles, "sw.js")
		resultFiles = append(resultFiles, "_verify.enc")

		for _, f := range files {
			encKey := f.RelPath + ".enc"
			resultFiles = append(resultFiles, encKey)
			fmt.Printf("  [dry-run] %s → %s (encrypted)\n", f.RelPath, encKey)

			if isHTMLFile(f.RelPath) && f.RelPath != "index.html" {
				resultFiles = append(resultFiles, f.RelPath)
				fmt.Printf("  [dry-run] %s (loader)\n", f.RelPath)
			}
		}
	} else {
		for _, f := range files {
			resultFiles = append(resultFiles, f.RelPath)
			fmt.Printf("  [dry-run] %s\n", f.RelPath)
		}
	}

	prefix := ""
	if opts.Prefix != "" {
		prefix = opts.Prefix + "/"
	}

	return &Result{
		FilesUploaded: 0,
		SiteURL:       fmt.Sprintf("%s/%s/%sindex.html", opts.Endpoint, opts.Bucket, prefix),
		Files:         resultFiles,
	}, nil
}

func isHTMLFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".html" || ext == ".htm"
}
