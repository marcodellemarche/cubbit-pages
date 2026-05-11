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

func formatSize(n int) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := unit, 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}


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
	Clean        bool
	Concurrency  int
	Prefix       string
	Locale       string
	Version      string
}

// Result holds the result of a deploy operation.
type Result struct {
	FilesUploaded  int
	FilesRemoved   int
	SiteURL        string
	Files          []string
	RemovedFiles   []string
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

	meta := &s3client.DeployMeta{
		Encrypted: opts.Encrypt,
		Locale:    opts.Locale,
		Version:   opts.Version,
		Prefix:    opts.Prefix,
	}
	uploader := s3client.NewUploader(client, !opts.PublicBucket, opts.Prefix, meta)
	uploadFn := uploader.Upload

	result, err := runWithUploader(ctx, files, opts, uploadFn)
	if err != nil {
		return nil, err
	}

	if opts.Clean {
		removed, removedFiles, cleanErr := cleanStale(ctx, client, opts.Prefix, result.Files)
		if cleanErr != nil {
			return nil, fmt.Errorf("cleaning stale files: %w", cleanErr)
		}
		result.FilesRemoved = removed
		result.RemovedFiles = removedFiles
	}

	return result, nil
}

// cleanStale deletes S3 objects in prefix that are not in uploadedRelKeys.
func cleanStale(ctx context.Context, client *s3client.Client, prefix string, uploadedRelKeys []string) (int, []string, error) {
	existing, err := client.ListObjects(ctx, prefix)
	if err != nil {
		return 0, nil, err
	}

	uploaded := make(map[string]bool, len(uploadedRelKeys))
	for _, k := range uploadedRelKeys {
		if prefix != "" {
			uploaded[prefix+"/"+k] = true
		} else {
			uploaded[k] = true
		}
	}

	var toDelete []string
	for _, obj := range existing {
		if !uploaded[obj.Key] {
			toDelete = append(toDelete, obj.Key)
		}
	}

	if len(toDelete) == 0 {
		return 0, nil, nil
	}

	if err := client.DeleteObjects(ctx, toDelete); err != nil {
		return 0, nil, err
	}

	return len(toDelete), toDelete, nil
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
		loginHTML := login.GenerateLoginPage(opts.Locale)
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

	// Upload with concurrency, collecting results in order for deterministic output.
	type uploadResult struct {
		key  string
		size int
		err  error
	}

	results := make([]uploadResult, len(items))

	var (
		uploaded int64
		sem      = make(chan struct{}, opts.Concurrency)
		wg       sync.WaitGroup
	)

	for i, item := range items {
		i, item := i, item
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			err := uploadFn(ctx, item.key, item.data)
			results[i] = uploadResult{key: item.key, size: len(item.data), err: err}
			if err == nil {
				atomic.AddInt64(&uploaded, 1)
			}
		}()
	}

	wg.Wait()

	var errs []error
	var uploadedFiles []string
	for _, r := range results {
		if r.err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", r.key, r.err))
		} else {
			fmt.Printf("  %-50s %s\n", r.key, formatSize(r.size))
			uploadedFiles = append(uploadedFiles, r.key)
		}
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("upload errors: %v", errs)
	}

	return &Result{
		FilesUploaded: int(uploaded),
		SiteURL:       s3client.BuildSiteURL(opts.Endpoint, opts.Bucket, opts.Prefix),
		Files:         uploadedFiles,
	}, nil
}

// dryRun simulates a deploy without uploading.
// Prints in the same format as a real deploy, with a [dry] marker.
func dryRun(files []FileEntry, opts Options) (*Result, error) {
	type dryItem struct {
		key  string
		size int
	}
	var items []dryItem

	if opts.Encrypt {
		loginHTML := login.GenerateLoginPage(opts.Locale)
		items = append(items, dryItem{"index.html", len(loginHTML)})

		swJS := login.GenerateServiceWorker()
		items = append(items, dryItem{"sw.js", len(swJS)})

		canary, err := crypto.EncryptCanary(opts.Password)
		if err != nil {
			return nil, fmt.Errorf("creating canary: %w", err)
		}
		items = append(items, dryItem{"_verify.enc", len(canary)})

		for _, f := range files {
			data, err := os.ReadFile(f.AbsPath)
			if err != nil {
				return nil, fmt.Errorf("reading %s: %w", f.RelPath, err)
			}
			encKey := f.RelPath + ".enc"
			// Encrypted size = plaintext + HEADER_LEN(33) + GCM tag(16)
			items = append(items, dryItem{encKey, len(data) + 49})

			if isHTMLFile(f.RelPath) && f.RelPath != "index.html" {
				loader := login.GenerateLoader(encKey)
				items = append(items, dryItem{f.RelPath, len(loader)})
			}
		}
	} else {
		for _, f := range files {
			data, err := os.ReadFile(f.AbsPath)
			if err != nil {
				return nil, fmt.Errorf("reading %s: %w", f.RelPath, err)
			}
			items = append(items, dryItem{f.RelPath, len(data)})
		}
	}

	var resultFiles []string
	for _, item := range items {
		fmt.Printf("  %-50s %s  [dry]\n", item.key, formatSize(item.size))
		resultFiles = append(resultFiles, item.key)
	}

	return &Result{
		FilesUploaded: 0,
		SiteURL:       s3client.BuildSiteURL(opts.Endpoint, opts.Bucket, opts.Prefix),
		Files:         resultFiles,
	}, nil
}

func isHTMLFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".html" || ext == ".htm"
}
