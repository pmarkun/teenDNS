package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/pmarkun/teendns/classifier"
)

type options struct {
	taxonomy     string
	guidance     string
	jevURL       string
	jevModel     string
	jevToken     string
	threshold    float64
	allowPrivate bool
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: teendns-classify classify [flags] URL | serve [flags]")
	}
	switch os.Args[1] {
	case "classify":
		runClassify(os.Args[2:])
	case "serve":
		runServe(os.Args[2:])
	default:
		log.Fatalf("unknown command %q", os.Args[1])
	}
}

func commonFlags(flags *flag.FlagSet) *options {
	defaults := &options{}
	flags.StringVar(&defaults.taxonomy, "taxonomy", "criteria/age-rating-v1.json", "age-rating taxonomy JSON")
	flags.StringVar(&defaults.guidance, "guidance", "criteria/priority-guidance-12-13-v1.json", "classifier guidance JSON")
	flags.StringVar(&defaults.jevURL, "jev-url", envOr("SIMPLE_JEV_URL", "http://127.0.0.1:8000/v1/classifier"), "Simple JEV classifier endpoint")
	flags.StringVar(&defaults.jevModel, "jev-model", envOr("SIMPLE_JEV_MODEL", "Qwen/Qwen3.5-0.8B"), "model name exposed by Simple JEV")
	flags.StringVar(&defaults.jevToken, "jev-token", os.Getenv("SIMPLE_JEV_TOKEN"), "optional Simple JEV bearer token")
	flags.Float64Var(&defaults.threshold, "threshold", envFloat("TEENDNS_CLASSIFIER_THRESHOLD", 0.75), "minimum criterion score")
	flags.BoolVar(&defaults.allowPrivate, "allow-private", false, "allow fetching private addresses for local development")
	return defaults
}

func runClassify(arguments []string) {
	flags := flag.NewFlagSet("classify", flag.ExitOnError)
	options := commonFlags(flags)
	_ = flags.Parse(arguments)
	if flags.NArg() != 1 {
		log.Fatal("usage: teendns-classify classify [flags] URL")
	}
	service, cleanup, err := build(*options)
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	result, err := service.ClassifyURL(ctx, flags.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		log.Fatal(err)
	}
}

func runServe(arguments []string) {
	flags := flag.NewFlagSet("serve", flag.ExitOnError)
	options := commonFlags(flags)
	listen := flags.String("listen", envOr("TEENDNS_CLASSIFIER_LISTEN", "127.0.0.1:8090"), "HTTP listen address")
	_ = flags.Parse(arguments)
	service, cleanup, err := build(*options)
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()
	server := &http.Server{
		Addr:              *listen,
		Handler:           classifier.Handler(service),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	go func() {
		<-ctx.Done()
		shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownContext)
	}()
	log.Printf("teenDNS classifier listening on http://%s", *listen)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func build(options options) (*classifier.Classifier, func(), error) {
	taxonomy, err := os.Open(options.taxonomy)
	if err != nil {
		return nil, func() {}, fmt.Errorf("open taxonomy: %w", err)
	}
	guidance, err := os.Open(options.guidance)
	if err != nil {
		taxonomy.Close()
		return nil, func() {}, fmt.Errorf("open guidance: %w", err)
	}
	closeFiles := func() {
		_ = taxonomy.Close()
		_ = guidance.Close()
	}
	catalog, err := classifier.LoadCatalog(taxonomy, guidance)
	closeFiles()
	if err != nil {
		return nil, func() {}, err
	}
	engine := &classifier.SimpleJEV{
		Endpoint: options.jevURL,
		Model:    options.jevModel,
		Token:    options.jevToken,
	}
	return &classifier.Classifier{
		Catalog: catalog, Extractor: classifier.NewExtractor(options.allowPrivate),
		Engine: engine, Threshold: options.threshold,
	}, func() {}, nil
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envFloat(name string, fallback float64) float64 {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		log.Fatalf("invalid %s: %v", name, err)
	}
	return parsed
}
