package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/marcodellemarche/cubbit-pages/internal/config"
	"github.com/marcodellemarche/cubbit-pages/internal/deploy"
	"github.com/marcodellemarche/cubbit-pages/internal/login"
	"github.com/marcodellemarche/cubbit-pages/internal/snippets"
	s3client "github.com/marcodellemarche/cubbit-pages/internal/s3"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "cubbit-pages",
		Short: "Deploy static sites to Cubbit S3 with optional AES-256-GCM encryption",
	}

	rootCmd.AddCommand(setupCmd())
	rootCmd.AddCommand(deployCmd())
	rootCmd.AddCommand(listCmd())
	rootCmd.AddCommand(deleteCmd())
	rootCmd.AddCommand(openCmd())
	rootCmd.AddCommand(statusCmd())
	rootCmd.AddCommand(snippetsCmd())
	rootCmd.AddCommand(versionCmd())
	rootCmd.AddCommand(updateCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func setupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Interactive setup wizard — saves credentials to ~/.cubbit/pages/config.yaml",
		RunE:  runSetup,
	}
}

func runSetup(cmd *cobra.Command, args []string) error {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println()
	fmt.Println("Cubbit Pages Setup")
	fmt.Println(strings.Repeat("─", 18))
	fmt.Println()

	fmt.Print("? Access Key: ")
	if !scanner.Scan() {
		return fmt.Errorf("aborted")
	}
	accessKey := strings.TrimSpace(scanner.Text())
	if accessKey == "" {
		return fmt.Errorf("access key is required")
	}

	var (
		secretKey string
		err       error
	)
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		secretKey, err = readPassword("? Secret Key: ")
		if err != nil {
			return fmt.Errorf("reading secret key: %w", err)
		}
	} else {
		fmt.Print("? Secret Key: ")
		if !scanner.Scan() {
			return fmt.Errorf("aborted")
		}
		secretKey = strings.TrimSpace(scanner.Text())
	}
	if secretKey == "" {
		return fmt.Errorf("secret key is required")
	}

	fmt.Printf("? Endpoint [%s]: ", config.DefaultEndpoint)
	if !scanner.Scan() {
		return fmt.Errorf("aborted")
	}
	endpoint := strings.TrimSpace(scanner.Text())
	if endpoint == "" {
		endpoint = config.DefaultEndpoint
	}

	// Create a throwaway client (no bucket yet) to probe/create bucket
	client, err := s3client.NewClient(endpoint, accessKey, secretKey, config.DefaultRegion, "probe")
	if err != nil {
		return fmt.Errorf("invalid credentials: %w", err)
	}

	ctx := context.Background()
	var bucket string
	for {
		fmt.Print("? Bucket: ")
		if !scanner.Scan() {
			return fmt.Errorf("aborted")
		}
		bucket = strings.TrimSpace(scanner.Text())
		if bucket == "" {
			fmt.Println("  Bucket name cannot be empty.")
			continue
		}

		fmt.Printf("\n  Checking bucket %q...", bucket)

		switch client.ProbeBucket(ctx, bucket) {
		case s3client.BucketExists:
			fmt.Println()
			fmt.Print("  Bucket already exists. Use it? (Y/n) ")
			if !scanner.Scan() {
				return fmt.Errorf("aborted")
			}
			answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
			fmt.Println()
			if answer == "" || answer == "y" || answer == "yes" {
				goto bucketOK
			}
			continue

		case s3client.BucketForbidden:
			fmt.Print("\n  ✗ Bucket exists but is not yours. Choose a different name.\n\n")
			continue

		case s3client.BucketNotFound:
			fmt.Printf(" creating...")
			if err := client.CreateBucket(ctx, bucket, config.DefaultRegion); err != nil {
				es := err.Error()
				if strings.Contains(es, "BucketAlreadyExists") {
					fmt.Print("\n  ✗ Bucket already exists and is not yours. Choose a different name.\n\n")
					continue
				}
				if strings.Contains(es, "InvalidBucketName") {
					fmt.Printf("\n  ✗ Invalid bucket name: %s\n\n", bucket)
					continue
				}
				fmt.Printf("\n  ✗ Error: %v\n\n", err)
				continue
			}
			fmt.Print(" ✓\n\n")
			goto bucketOK
		}
	}

bucketOK:
	fmt.Printf("? Login page locale [en] (%s): ", strings.Join(login.KnownLocales(), "/"))
	if !scanner.Scan() {
		return fmt.Errorf("aborted")
	}
	locale := strings.TrimSpace(scanner.Text())
	if locale == "" {
		locale = "en"
	}
	if !login.IsKnownLocale(locale) {
		fmt.Printf("  Unknown locale %q — using \"en\". Available: %s\n", locale, strings.Join(login.KnownLocales(), ", "))
		locale = "en"
	}
	fmt.Println()

	fc := &config.FileConfig{
		AccessKey: accessKey,
		SecretKey: secretKey,
		Bucket:    bucket,
		Endpoint:  endpoint,
		Locale:    locale,
	}
	if err := config.SaveFileConfig(fc); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	configPath, _ := config.ConfigFilePath()
	fmt.Printf("  Config saved to %s\n", configPath)

	fmt.Print("\n  Verifying connection... ")
	verifyClient, err := s3client.NewClient(endpoint, accessKey, secretKey, config.DefaultRegion, bucket)
	if err == nil {
		verifyCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		err = verifyClient.HeadBucket(verifyCtx)
	}
	if err != nil {
		fmt.Printf("✗\n  Warning: connection test failed — double-check your credentials.\n")
	} else {
		fmt.Println("✓")
	}

	fmt.Println()
	fmt.Println("  Done! Try:")
	fmt.Printf("    cubbit-pages deploy ./my-site\n")
	fmt.Println()

	return nil
}

func deployCmd() *cobra.Command {
	cfg := &config.Config{}

	cmd := &cobra.Command{
		Use:   "deploy <directory>",
		Short: "Deploy a static site to Cubbit S3",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.SourceDir = args[0]

			// Check source dir exists
			info, err := os.Stat(cfg.SourceDir)
			if err != nil {
				return fmt.Errorf("source directory %q: %w", cfg.SourceDir, err)
			}
			if !info.IsDir() {
				return fmt.Errorf("%q is not a directory", cfg.SourceDir)
			}

			// If encrypt is set but no password, ask interactively
			if cfg.Encrypt && cfg.Password == "" {
				pwd, err := readPassword("Encryption password: ")
				if err != nil {
					return fmt.Errorf("reading password: %w", err)
				}
				cfg.Password = pwd
			}

			if cfg.Locale != "" && !login.IsKnownLocale(cfg.Locale) {
				return fmt.Errorf("unknown locale %q (available: %v)", cfg.Locale, login.KnownLocales())
			}

			if err := cfg.Resolve(); err != nil {
				return err
			}

			fmt.Printf("\nDeploying to s3://%s/\n", cfg.Bucket)
			if cfg.Encrypt {
				fmt.Println("Mode: encrypted (AES-256-GCM)")
			} else {
				fmt.Println("Mode: plaintext")
			}
			if cfg.DryRun {
				fmt.Println("⚠ Dry run — no files will be uploaded")
			}
			fmt.Println()

			opts := deploy.Options{
				SourceDir:    cfg.SourceDir,
				Bucket:       cfg.Bucket,
				Endpoint:     cfg.Endpoint,
				AccessKey:    cfg.AccessKey,
				SecretKey:    cfg.SecretKey,
				Region:       cfg.Region,
				Encrypt:      cfg.Encrypt,
				Password:     cfg.Password,
				PublicBucket: cfg.PublicBucket,
				DryRun:       cfg.DryRun,
				Clean:        cfg.Clean,
				Concurrency:  cfg.Concurrency,
				Prefix:       cfg.Prefix,
				Locale:       cfg.Locale,
				Version:      Version,
			}

			result, err := deploy.Run(cmd.Context(), opts)
			if err != nil {
				return err
			}

			fmt.Printf("\nDeploy complete: %d file(s) uploaded\n", result.FilesUploaded)
			if result.FilesRemoved > 0 {
				for _, k := range result.RemovedFiles {
					fmt.Printf("  [clean] %s\n", k)
				}
				fmt.Printf("Cleaned:  %d stale file(s) removed\n", result.FilesRemoved)
			}
			fmt.Printf("URL: %s\n", result.SiteURL)

			// Persist last deploy metadata (create config file if it doesn't exist yet).
			if !cfg.DryRun {
				fc, _ := config.LoadFileConfig()
				if fc == nil {
					fc = &config.FileConfig{}
				}
				fc.LastDeploy = &config.LastDeploy{
					Bucket:    cfg.Bucket,
					Prefix:    cfg.Prefix,
					URL:       result.SiteURL,
					Files:     result.FilesUploaded,
					Encrypted: cfg.Encrypt,
					Date:      time.Now().UTC(),
				}
				_ = config.SaveFileConfig(fc)
			}

			if !cfg.PublicBucket && !cfg.DryRun {
				fmt.Println("\nNote: files were made public via per-object ACL.")
				fmt.Println("To use a bucket policy instead, use --public-bucket and apply:")
				fmt.Printf("  cubbit-pages snippets --bucket %s --type bucket-policy\n", cfg.Bucket)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&cfg.Bucket, "bucket", "b", "", "bucket name (or CUBBIT_BUCKET)")
	cmd.Flags().StringVar(&cfg.AccessKey, "access-key", "", "Cubbit access key (or CUBBIT_ACCESS_KEY)")
	cmd.Flags().StringVar(&cfg.SecretKey, "secret-key", "", "Cubbit secret key (or CUBBIT_SECRET_KEY)")
	cmd.Flags().StringVar(&cfg.Endpoint, "endpoint", "", "S3 endpoint (default: https://s3.cubbit.eu)")
	cmd.Flags().StringVar(&cfg.Region, "region", "", "AWS/S3 region (default: eu-west-1)")
	cmd.Flags().BoolVarP(&cfg.Encrypt, "encrypt", "e", false, "enable AES-256-GCM encryption")
	cmd.Flags().StringVarP(&cfg.Password, "password", "p", "", "encryption password (prompted if --encrypt and not provided)")
	cmd.Flags().BoolVar(&cfg.PublicBucket, "public-bucket", false, "assume public bucket policy (skip per-object ACL)")
	cmd.Flags().BoolVar(&cfg.DryRun, "dry-run", false, "show what would be uploaded without uploading")
	cmd.Flags().BoolVar(&cfg.Clean, "clean", true, "delete S3 files not present in source directory (use --clean=false to disable)")
	cmd.Flags().IntVar(&cfg.Concurrency, "concurrency", config.DefaultConcurrency, "number of parallel uploads")
	cmd.Flags().StringVar(&cfg.Prefix, "prefix", "", "S3 key prefix for all files")
	cmd.Flags().StringVar(&cfg.Locale, "locale", "", fmt.Sprintf("login page locale (%s)", strings.Join(login.KnownLocales(), ", ")))

	return cmd
}

func snippetsCmd() *cobra.Command {
	var bucket string
	var snippetType string

	cmd := &cobra.Command{
		Use:   "snippets",
		Short: "Show configuration snippets for bucket setup",
		RunE: func(cmd *cobra.Command, args []string) error {
			if bucket == "" {
				bucket = os.Getenv("CUBBIT_BUCKET")
			}
			if bucket == "" {
				return fmt.Errorf("bucket is required (--bucket or CUBBIT_BUCKET)")
			}

			switch snippetType {
			case "bucket-policy":
				fmt.Println(snippets.BucketPolicyCLI(bucket))
			case "cors":
				fmt.Println(snippets.CORSCLI(bucket))
			case "iam":
				fmt.Println(snippets.IAMPolicy(bucket))
			case "lifecycle":
				fmt.Println(snippets.LifecycleCLI(bucket, 30))
			case "all", "":
				fmt.Println(snippets.AllSnippets(bucket))
			default:
				return fmt.Errorf("unknown snippet type: %s (use: bucket-policy, cors, iam, lifecycle, all)", snippetType)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&bucket, "bucket", "b", "", "bucket name")
	cmd.Flags().StringVar(&snippetType, "type", "all", "snippet type: bucket-policy, cors, iam, lifecycle, all")

	return cmd
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("cubbit-pages %s\n", Version)
			fmt.Printf("  commit:  %s\n", Commit)
			fmt.Printf("  built:   %s\n", BuildDate)
		},
	}
}

func statusCmd() *cobra.Command {
	cfg := &config.Config{}
	var deep bool
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show current config and last deploy",
		RunE: func(cmd *cobra.Command, args []string) error {
			fc, err := config.LoadFileConfig()
			if err != nil {
				return fmt.Errorf("reading config: %w", err)
			}

			var inventory []s3client.DeployInfo
			if deep {
				if err := cfg.Resolve(); err != nil {
					return fmt.Errorf("--deep requires credentials: %w", err)
				}
				client, err := s3client.NewClient(cfg.Endpoint, cfg.AccessKey, cfg.SecretKey, cfg.Region, cfg.Bucket)
				if err != nil {
					return err
				}
				inventory, err = client.DiscoverDeploys(cmd.Context(), cfg.Endpoint)
				if err != nil {
					return fmt.Errorf("scanning bucket: %w", err)
				}
			}

			if asJSON {
				return printStatusJSON(fc, inventory)
			}

			configPath, _ := config.ConfigFilePath()
			fmt.Println()
			fmt.Printf("  Config (%s)\n", configPath)
			fmt.Println("  " + strings.Repeat("─", 44))

			if fc == nil {
				fmt.Println("  No config file found. Run `cubbit-pages setup` to get started.")
				fmt.Println()
			} else {
				endpoint := fc.Endpoint
				if endpoint == "" {
					endpoint = config.DefaultEndpoint
				}
				locale := fc.Locale
				if locale == "" {
					locale = "en"
				}
				fmt.Printf("  %-12s %s\n", "Bucket:", fc.Bucket)
				fmt.Printf("  %-12s %s\n", "Endpoint:", endpoint)
				fmt.Printf("  %-12s %s\n", "Locale:", locale)

				if fc.LastDeploy != nil {
					ld := fc.LastDeploy
					mode := "plaintext"
					if ld.Encrypted {
						mode = "encrypted (AES-256-GCM)"
					}
					prefix := ld.Prefix
					if prefix == "" {
						prefix = "(root)"
					}
					fmt.Println()
					fmt.Println("  Last deploy")
					fmt.Println("  " + strings.Repeat("─", 44))
					fmt.Printf("  %-12s %s\n", "Bucket:", ld.Bucket)
					fmt.Printf("  %-12s %s\n", "Prefix:", prefix)
					fmt.Printf("  %-12s %d\n", "Files:", ld.Files)
					fmt.Printf("  %-12s %s\n", "Mode:", mode)
					fmt.Printf("  %-12s %s\n", "Date:", ld.Date.Local().Format("2006-01-02 15:04"))
					fmt.Printf("  %-12s %s\n", "URL:", ld.URL)
				} else {
					fmt.Println()
					fmt.Println("  No deploy recorded yet. Run `cubbit-pages deploy` to get started.")
				}
				fmt.Println()
			}

			if !deep {
				return nil
			}

			fmt.Printf("  Bucket inventory: %s", cfg.Bucket)
			if len(inventory) == 0 {
				fmt.Println(" (no deploys found)")
				fmt.Println()
				return nil
			}
			fmt.Printf(" (%d deploy)\n", len(inventory))
			fmt.Println("  " + strings.Repeat("─", 44))
			fmt.Println()

			for i, d := range inventory {
				pfxDisplay := d.Prefix
				if pfxDisplay == "" {
					pfxDisplay = "(root)"
				}
				mode := "plaintext"
				if d.Encrypted {
					mode = "encrypted"
				}
				ts := ""
				if !d.Timestamp.IsZero() {
					ts = d.Timestamp.Local().Format("2006-01-02 15:04")
				}
				meta := ""
				if !d.HasMetadata {
					meta = "  (no metadata)"
				}
				fmt.Printf("  #%-2d %-20s %3d files  %8s  %-9s  %s%s\n",
					i+1, pfxDisplay, d.FileCount, formatSize(d.TotalSize), mode, ts, meta)
				fmt.Printf("      %s\n", d.URL)
				fmt.Println()
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&deep, "deep", false, "query S3 for full deploy inventory (requires credentials)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output as JSON (machine-readable)")
	cmd.Flags().StringVarP(&cfg.Bucket, "bucket", "b", "", "bucket name (or CUBBIT_BUCKET)")
	cmd.Flags().StringVar(&cfg.AccessKey, "access-key", "", "Cubbit access key (or CUBBIT_ACCESS_KEY)")
	cmd.Flags().StringVar(&cfg.SecretKey, "secret-key", "", "Cubbit secret key (or CUBBIT_SECRET_KEY)")
	cmd.Flags().StringVar(&cfg.Endpoint, "endpoint", "", "S3 endpoint (default: https://s3.cubbit.eu)")
	cmd.Flags().StringVar(&cfg.Region, "region", "", "AWS/S3 region (default: eu-west-1)")

	return cmd
}

type statusJSON struct {
	Config     *statusJSONConfig  `json:"config,omitempty"`
	LastDeploy *config.LastDeploy `json:"last_deploy,omitempty"`
	Inventory  []s3client.DeployInfo `json:"inventory,omitempty"`
}

type statusJSONConfig struct {
	Bucket   string `json:"bucket"`
	Endpoint string `json:"endpoint"`
	Locale   string `json:"locale"`
}

func printStatusJSON(fc *config.FileConfig, inventory []s3client.DeployInfo) error {
	out := statusJSON{Inventory: inventory}
	if fc != nil {
		endpoint := fc.Endpoint
		if endpoint == "" {
			endpoint = config.DefaultEndpoint
		}
		locale := fc.Locale
		if locale == "" {
			locale = "en"
		}
		out.Config = &statusJSONConfig{
			Bucket:   fc.Bucket,
			Endpoint: endpoint,
			Locale:   locale,
		}
		out.LastDeploy = fc.LastDeploy
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func openCmd() *cobra.Command {
	cfg := &config.Config{}

	cmd := &cobra.Command{
		Use:   "open",
		Short: "Open the deployed site in the default browser",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.ResolveOpen()
			if cfg.Bucket == "" {
				return fmt.Errorf("bucket is required (--bucket or CUBBIT_BUCKET)")
			}
			siteURL := cfg.SiteURL()
			fmt.Printf("Opening %s\n", siteURL)
			return openBrowser(siteURL)
		},
	}

	cmd.Flags().StringVarP(&cfg.Bucket, "bucket", "b", "", "bucket name (or CUBBIT_BUCKET)")
	cmd.Flags().StringVar(&cfg.Endpoint, "endpoint", "", "S3 endpoint (default: https://s3.cubbit.eu)")
	cmd.Flags().StringVar(&cfg.Prefix, "prefix", "", "S3 key prefix")

	return cmd
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func listCmd() *cobra.Command {
	cfg := &config.Config{}
	var prefix string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List files in a Cubbit S3 bucket",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cfg.Resolve(); err != nil {
				return err
			}

			prefix = strings.Trim(prefix, "/")

			client, err := s3client.NewClient(cfg.Endpoint, cfg.AccessKey, cfg.SecretKey, cfg.Region, cfg.Bucket)
			if err != nil {
				return err
			}

			objects, err := client.ListObjects(cmd.Context(), prefix)
			if err != nil {
				return err
			}

			if len(objects) == 0 {
				fmt.Println("No files found.")
				return nil
			}

			fmt.Printf("%-60s  %8s  %s\n", "KEY", "SIZE", "LAST MODIFIED")
			fmt.Println(strings.Repeat("─", 88))
			for _, obj := range objects {
				fmt.Printf("%-60s  %8s  %s\n", obj.Key, formatSize(obj.Size), obj.LastModified.Format("2006-01-02 15:04"))
			}
			fmt.Printf("\n%d file(s)\n", len(objects))
			return nil
		},
	}

	cmd.Flags().StringVarP(&cfg.Bucket, "bucket", "b", "", "bucket name (or CUBBIT_BUCKET)")
	cmd.Flags().StringVar(&cfg.AccessKey, "access-key", "", "Cubbit access key (or CUBBIT_ACCESS_KEY)")
	cmd.Flags().StringVar(&cfg.SecretKey, "secret-key", "", "Cubbit secret key (or CUBBIT_SECRET_KEY)")
	cmd.Flags().StringVar(&cfg.Endpoint, "endpoint", "", "S3 endpoint (default: https://s3.cubbit.eu)")
	cmd.Flags().StringVar(&cfg.Region, "region", "", "AWS/S3 region (default: eu-west-1)")
	cmd.Flags().StringVar(&prefix, "prefix", "", "filter by S3 key prefix")

	return cmd
}

func deleteCmd() *cobra.Command {
	cfg := &config.Config{}
	var prefix string
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete files from a Cubbit S3 bucket",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cfg.Resolve(); err != nil {
				return err
			}

			prefix = strings.Trim(prefix, "/")

			client, err := s3client.NewClient(cfg.Endpoint, cfg.AccessKey, cfg.SecretKey, cfg.Region, cfg.Bucket)
			if err != nil {
				return err
			}

			objects, err := client.ListObjects(cmd.Context(), prefix)
			if err != nil {
				return err
			}

			if len(objects) == 0 {
				fmt.Println("No files found.")
				return nil
			}

			if prefix == "" {
				fmt.Fprintf(os.Stderr, "WARNING: no --prefix specified — ALL files in s3://%s/ will be deleted.\n\n", cfg.Bucket)
			}
			fmt.Printf("Files to delete from s3://%s/:\n\n", cfg.Bucket)
			for _, obj := range objects {
				fmt.Printf("  %s (%s)\n", obj.Key, formatSize(obj.Size))
			}
			fmt.Printf("\n%d file(s) will be permanently deleted.\n\n", len(objects))

			if !yes {
				fmt.Print("Confirm? (y/N) ")
				scanner := bufio.NewScanner(os.Stdin)
				if !scanner.Scan() {
					return fmt.Errorf("aborted")
				}
				answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
				if answer != "y" && answer != "yes" {
					fmt.Fprintln(os.Stderr, "Aborted.")
					os.Exit(1)
				}
			}

			keys := make([]string, len(objects))
			for i, obj := range objects {
				keys[i] = obj.Key
			}

			if err := client.DeleteObjects(cmd.Context(), keys); err != nil {
				return err
			}

			fmt.Printf("Deleted %d file(s).\n", len(keys))
			return nil
		},
	}

	cmd.Flags().StringVarP(&cfg.Bucket, "bucket", "b", "", "bucket name (or CUBBIT_BUCKET)")
	cmd.Flags().StringVar(&cfg.AccessKey, "access-key", "", "Cubbit access key (or CUBBIT_ACCESS_KEY)")
	cmd.Flags().StringVar(&cfg.SecretKey, "secret-key", "", "Cubbit secret key (or CUBBIT_SECRET_KEY)")
	cmd.Flags().StringVar(&cfg.Endpoint, "endpoint", "", "S3 endpoint (default: https://s3.cubbit.eu)")
	cmd.Flags().StringVar(&cfg.Region, "region", "", "AWS/S3 region (default: eu-west-1)")
	cmd.Flags().StringVar(&prefix, "prefix", "", "delete only files with this S3 key prefix")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompt")

	return cmd
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func updateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update cubbit-pages to the latest release",
		RunE:  runUpdate,
	}
}

func runUpdate(cmd *cobra.Command, args []string) error {
	const repo = "marcodellemarche/cubbit-pages"
	const apiURL = "https://api.github.com/repos/" + repo + "/releases/latest"

	fmt.Printf("Checking latest release...\n")

	req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, apiURL, nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("User-Agent", "cubbit-pages/"+Version)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetching release info: %w", err)
	}
	defer resp.Body.Close()

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("parsing release info: %w", err)
	}

	latest := release.TagName
	if latest == "" {
		return fmt.Errorf("could not determine latest version")
	}

	if Version != "dev" && Version == latest {
		fmt.Printf("Already up to date (%s).\n", Version)
		return nil
	}

	fmt.Printf("Updating %s → %s\n", Version, latest)

	goos := runtime.GOOS
	goarch := runtime.GOARCH
	filename := fmt.Sprintf("cubbit-pages-%s-%s", goos, goarch)
	if goos == "windows" {
		filename += ".exe"
	}
	downloadURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, latest, filename)

	fmt.Printf("Downloading %s...\n", filename)

	dlReq, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("building download request: %w", err)
	}
	dlResp, err := http.DefaultClient.Do(dlReq)
	if err != nil {
		return fmt.Errorf("downloading binary: %w", err)
	}
	defer dlResp.Body.Close()
	if dlResp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d (unsupported platform?)", dlResp.StatusCode)
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding current binary: %w", err)
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return fmt.Errorf("resolving symlink: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "cubbit-pages-update-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, dlResp.Body); err != nil {
		tmpFile.Close()
		return fmt.Errorf("writing binary: %w", err)
	}
	tmpFile.Close()

	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("setting permissions: %w", err)
	}

	// Try atomic rename first; falls back to copy if tmp and exe are on different filesystems.
	if err := os.Rename(tmpPath, exe); err != nil {
		if err := replaceByCopy(tmpPath, exe); err != nil {
			return fmt.Errorf("replacing binary (try: sudo cubbit-pages update): %w", err)
		}
	}

	fmt.Printf("✓ Updated to %s\n", latest)
	return nil
}

func replaceByCopy(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func readPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		pwd, err := term.ReadPassword(fd)
		fmt.Println()
		if err != nil {
			return "", err
		}
		return string(pwd), nil
	}
	// Non-interactive: read from stdin
	var pwd string
	if _, err := fmt.Scanln(&pwd); err != nil {
		return "", err
	}
	return pwd, nil
}
