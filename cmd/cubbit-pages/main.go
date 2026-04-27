package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/marcodellemarche/cubbit-pages/internal/config"
	"github.com/marcodellemarche/cubbit-pages/internal/deploy"
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
	rootCmd.AddCommand(snippetsCmd())
	rootCmd.AddCommand(versionCmd())

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
			fmt.Println("  Il nome del bucket non può essere vuoto.")
			continue
		}

		fmt.Printf("\n  Verifica bucket %q...", bucket)

		switch client.ProbeBucket(ctx, bucket) {
		case s3client.BucketExists:
			fmt.Println()
			fmt.Print("  Bucket già esistente. Usarlo? (Y/n) ")
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
			fmt.Println("\n  ✗ Bucket esistente ma non tuo. Scegli un altro nome.\n")
			continue

		case s3client.BucketNotFound:
			fmt.Printf(" creazione in corso...")
			if err := client.CreateBucket(ctx, bucket, config.DefaultRegion); err != nil {
				es := err.Error()
				if strings.Contains(es, "BucketAlreadyExists") {
					fmt.Println("\n  ✗ Bucket già esistente e non tuo. Scegli un altro nome.\n")
					continue
				}
				if strings.Contains(es, "InvalidBucketName") {
					fmt.Printf("\n  ✗ Nome bucket non valido: %s\n\n", bucket)
					continue
				}
				fmt.Printf("\n  ✗ Errore: %v\n\n", err)
				continue
			}
			fmt.Println(" ✓\n")
			goto bucketOK
		}
	}

bucketOK:
	fc := &config.FileConfig{
		AccessKey: accessKey,
		SecretKey: secretKey,
		Bucket:    bucket,
		Endpoint:  endpoint,
	}
	if err := config.SaveFileConfig(fc); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	configPath, _ := config.ConfigFilePath()
	fmt.Printf("  Config salvata in %s\n", configPath)
	fmt.Println()
	fmt.Println("  Pronto! Prova:")
	fmt.Printf("    cubbit-pages deploy ./mio-sito\n")
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
				pwd, err := readPassword("Password per cifratura: ")
				if err != nil {
					return fmt.Errorf("reading password: %w", err)
				}
				cfg.Password = pwd
			}

			if err := cfg.Resolve(); err != nil {
				return err
			}

			fmt.Printf("\nDeploy su s3://%s/\n", cfg.Bucket)
			if cfg.Encrypt {
				fmt.Println("Modalità: cifrata (AES-256-GCM)")
			} else {
				fmt.Println("Modalità: in chiaro")
			}
			if cfg.DryRun {
				fmt.Println("⚠ Dry run — nessun file verrà caricato")
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
				Concurrency:  cfg.Concurrency,
				Prefix:       cfg.Prefix,
			}

			result, err := deploy.Run(cmd.Context(), opts)
			if err != nil {
				return err
			}

			fmt.Printf("\nDeploy completato: %d file caricati\n", result.FilesUploaded)
			fmt.Printf("URL: %s\n", result.SiteURL)

			if !cfg.PublicBucket && !cfg.DryRun {
				fmt.Println("\nNota: i file sono stati resi pubblici tramite ACL per-oggetto.")
				fmt.Println("Per usare una bucket policy invece, usa --public-bucket e applica:")
				fmt.Printf("  cubbit-pages snippets --bucket %s --type bucket-policy\n", cfg.Bucket)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&cfg.Bucket, "bucket", "b", "", "bucket name (or CUBBIT_BUCKET)")
	cmd.Flags().StringVar(&cfg.AccessKey, "access-key", "", "Cubbit access key (or CUBBIT_ACCESS_KEY)")
	cmd.Flags().StringVar(&cfg.SecretKey, "secret-key", "", "Cubbit secret key (or CUBBIT_SECRET_KEY)")
	cmd.Flags().StringVar(&cfg.Endpoint, "endpoint", "", "S3 endpoint (default: https://s3.cubbit.eu)")
	cmd.Flags().BoolVarP(&cfg.Encrypt, "encrypt", "e", false, "enable AES-256-GCM encryption")
	cmd.Flags().StringVarP(&cfg.Password, "password", "p", "", "encryption password (prompted if --encrypt and not provided)")
	cmd.Flags().BoolVar(&cfg.PublicBucket, "public-bucket", false, "assume public bucket policy (skip per-object ACL)")
	cmd.Flags().BoolVar(&cfg.DryRun, "dry-run", false, "show what would be uploaded without uploading")
	cmd.Flags().IntVar(&cfg.Concurrency, "concurrency", config.DefaultConcurrency, "number of parallel uploads")
	cmd.Flags().StringVar(&cfg.Prefix, "prefix", "", "S3 key prefix for all files")

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
