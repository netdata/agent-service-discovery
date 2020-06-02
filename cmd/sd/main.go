package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/netdata/sd/manager"
	"github.com/netdata/sd/manager/config/provider/file"
	"github.com/netdata/sd/manager/config/provider/kubernetes"
	"github.com/netdata/sd/pkg/log"

	"github.com/jessevdk/go-flags"
	"github.com/rs/zerolog"
)

type options struct {
	ConfigFile string `long:"config-file" description:"Configuration file path"`
	ConfigMap  string `long:"config-map" description:"Configuration ConfigMap (name:key)"`
	Debug      bool   `short:"d" long:"debug" description:"Debug mode"`
}

var logger = log.New("main")

func main() {
	opts := parseCLI()
	applyFromEnv(&opts)

	if err := validateOptions(opts); err != nil {
		logger.Fatal().Err(err).Msg("failed to validate cli options")
	}

	if opts.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	provider, err := newConfigProvider(opts)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create config provider")
	}

	mgr := manager.New(provider)
	run(mgr)
}

func run(mgr *manager.Manager) {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	wg.Add(1)
	go func() { defer wg.Done(); mgr.Run(ctx) }()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	sig := <-ch
	logger.Info().Msgf("received %s signal (%d). Terminating...", sig, sig)
	cancel()
	wg.Wait()
}

func parseCLI() options {
	var opts options
	parser := flags.NewParser(&opts, flags.Default)
	parser.Name = "sd"
	parser.Usage = "[OPTION]..."

	if _, err := parser.ParseArgs(os.Args); err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			os.Exit(1)
		}
	}
	return opts
}

func applyFromEnv(opts *options) {
	if v, ok := os.LookupEnv("NETDATA_SD_CONFIG_FILE"); ok && opts.ConfigFile == "" {
		opts.ConfigFile = v
	}
	if v, ok := os.LookupEnv("NETDATA_SD_CONFIG_MAP"); ok && opts.ConfigMap == "" {
		opts.ConfigMap = v
	}
}

func validateOptions(opts options) error {
	if opts.ConfigFile == "" && opts.ConfigMap == "" {
		return errors.New("configuration source not set")
	}
	return nil
}

func newConfigProvider(opts options) (manager.ConfigProvider, error) {
	if opts.ConfigFile != "" {
		return file.NewProvider([]string{opts.ConfigFile}), nil
	}

	parts := strings.Split(strings.TrimSpace(opts.ConfigMap), ":")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("config-map parameter bad syntax ('%s')", opts.ConfigMap)
	}
	provider, err := kubernetes.NewProvider(kubernetes.Config{
		Namespace: os.Getenv("MY_POD_NAMESPACE"),
		ConfigMap: parts[0],
		Key:       parts[1],
	})
	if err != nil {
		return nil, err
	}
	return provider, nil
}
