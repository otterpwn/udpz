package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"udpz/pkg/scan"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

var (
	// Scan options
	hostConcurrency uint = 10
	portConcurrency uint = 50
	timeoutMs       uint = 3000
	retransmissions uint = 2

	// DNS options
	scanAllAddresses bool = true

	// Logging options
	quiet  bool = false // Disable info logging output (non-errors)
	silent bool = false // Disable logging entirely

	info  bool = true // Default log level
	debug bool = false
	trace bool = false

	// Output options
	outputPath   string
	logPath      string
	outputFormat string = "auto"
	logFormat    string = "auto"
	outputAppend bool   = true

	// Proxy options
	socks5Address  string
	socks5User     string
	socks5Password string
	socks5Timeout  uint = 3000

	// Constraints
	supportedLogFormats = map[string]bool{
		"json": true, "jsonl": true,
		"pretty": true,
		"auto":   true,
	}
	supportedOutputFormats = map[string]bool{
		"text": true, "txt": true,
		"yaml": true, "yml": true,
		"json": true, "jsonl": true,
		"csv":    true,
		"tsv":    true,
		"pretty": true,
		"auto":   true,
	}
)

func init() {

	rootCmd.Flags().SortFlags = false
	rootCmd.InitDefaultVersionFlag()
	rootCmd.InitDefaultCompletionCmd()

	// Output
	rootCmd.Flags().StringVarP(&outputPath, "output", "o", outputPath, "Save results to file")
	rootCmd.Flags().StringVarP(&logPath, "log", "O", logPath, "Output log messages to file")
	rootCmd.Flags().BoolVarP(&outputAppend, "append", "a", outputAppend, "Append results to output file")
	rootCmd.Flags().StringVarP(&outputFormat, "format", "f", outputFormat, "Output format [text, pretty, csv, tsv, json, yaml, auto]")
	rootCmd.Flags().StringVarP(&logFormat, "log-format", "L", logFormat, `Output log format [pretty, json, auto]`)

	// Performance
	rootCmd.Flags().UintVarP(&hostConcurrency, "host-tasks", "c", hostConcurrency, "Maximum Number of hosts to scan concurrently")
	rootCmd.Flags().UintVarP(&portConcurrency, "port-tasks", "p", portConcurrency, "Number of Concurrent scan tasks per host")
	rootCmd.Flags().UintVarP(&retransmissions, "retries", "r", retransmissions, "Number of probe retransmissions per probe")
	rootCmd.Flags().UintVarP(&timeoutMs, "timeout", "t", timeoutMs, "UDP Probe timeout in milliseconds")

	// DNS
	rootCmd.Flags().BoolVarP(&scanAllAddresses, "all", "A", scanAllAddresses, "Scan all resolved addresses instead of just the first")

	/*
		TODO
		// Proxy
		rootCmd.Flags().StringVarP(&socks5Address, "socks", "S", socks5Address, "SOCKS5 proxy address as HOST:PORT")
		rootCmd.Flags().StringVar(&socks5User, "socks-user", socks5User, "SOCKS5 proxy username")
		rootCmd.Flags().StringVar(&socks5Password, "socks-pass", socks5Password, "SOCKS5 proxy password")
		rootCmd.Flags().UintVar(&socks5Timeout, "socks-timeout", socks5Timeout, "SOCKS5 proxy timeout")
	*/

	// Logging
	rootCmd.Flags().BoolVarP(&debug, "debug", "D", debug, "Enable debug logging (Very noisy!)")
	rootCmd.Flags().BoolVarP(&trace, "trace", "T", trace, "Enable trace logging (Very noisy!)")
	rootCmd.Flags().BoolVarP(&quiet, "quiet", "q", quiet, "Disable info logging")
	rootCmd.Flags().BoolVarP(&silent, "silent", "s", silent, "Disable ALL logging")
}

var rootCmd = &cobra.Command{
	Use:     "udpz [flags] [targets ...]",
	Short:   "Speedy probe-oriented UDP port scanner",
	Version: "0.0.1-beta",
	Long: `
  ┳┳  ┳┓  ┏┓  ┏┓
  ┃┃━━┃┃━━┃┃━━┏┛
  ┗┛  ┻┛  ┣┛  ┗┛

  Author: Bryan McNulty (@bryanmcnulty)
  Source: https://github.com/FalconOps-Cybersecurity/udpz`,

	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, targets []string) (err error) {

		var outputFile *os.File
		var log zerolog.Logger
		var logFile *os.File
		var outputFlags int = os.O_WRONLY | os.O_CREATE
		var logFileFlags int = os.O_WRONLY | os.O_CREATE | os.O_APPEND

		outputFormat = strings.ToLower(outputFormat)

		if sup, ok := supportedOutputFormats[outputFormat]; !ok || !sup {
			return errors.New("invalid output format: " + outputFormat)
		}
		if sup, ok := supportedLogFormats[logFormat]; !ok || !sup {
			return errors.New("invalid log format: " + logFormat)
		}
		if portConcurrency < 1 || hostConcurrency < 1 {
			return errors.New("concurrency value must be > 0")
		}
		if timeoutMs < 1 {
			return errors.New("timeout value must be > 0")
		}
		if outputAppend {
			outputFlags |= os.O_APPEND
		}

		if silent {
			zerolog.SetGlobalLevel(zerolog.Disabled)
		} else if quiet {
			zerolog.SetGlobalLevel(zerolog.ErrorLevel)
		} else if trace {
			zerolog.SetGlobalLevel(zerolog.TraceLevel)
		} else if debug {
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
		} else if info {
			zerolog.SetGlobalLevel(zerolog.InfoLevel)
		}

		zerolog.TimeFieldFormat = zerolog.TimeFormatUnixNano

		if logPath == "" {
			log = zerolog.New(os.Stderr).
				With().
				Timestamp().
				Caller().
				Logger()
			if logFormat == "auto" || logFormat == "pretty" {
				log = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
			}
		} else if logFile, err = os.OpenFile(logPath, logFileFlags, 0o644); err == nil {

			defer logFile.Close()
			log = zerolog.New(logFile).
				With().
				Timestamp().
				Caller().
				Logger()
			if logFormat == "pretty" {
				log = log.Output(zerolog.ConsoleWriter{Out: logFile})
			}

		} else {
			log = zerolog.New(os.Stderr).
				With().
				Timestamp().
				Caller().
				Logger()
			if logFormat == "auto" || logFormat == "pretty" {
				log = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
			}
			log.Error().
				AnErr("error", err).
				Str("log_path", logPath).
				Msg("Could not open log file for writing")
		}

		var scanner scan.UdpProbeScanner

		if scanner, err = scan.NewUdpProbeScanner(
			log,
			scanAllAddresses,
			hostConcurrency,
			portConcurrency,
			retransmissions,
			time.Duration(timeoutMs)*time.Millisecond,
			socks5Address,
			socks5User,
			socks5Password,
			int(socks5Timeout)); err != nil {

			log.Fatal().
				Err(err).
				Msg("Failed to initialize scanner")
		}

		var scanStartTime, scanEndTime time.Time

		log.Info().
			Msg("Starting scanner")

		scanStartTime = time.Now()
		scanner.Scan(targets)
		scanEndTime = time.Now()

		log.Info().
			Time("start", scanStartTime).
			Time("end", scanEndTime).
			TimeDiff("duration", scanEndTime, scanStartTime).
			Msg("Scan complete")

		if scanner.Length() > 0 {

			if outputPath == "" {
				outputFile = os.Stdout
				if outputFormat == "auto" {
					outputFormat = "pretty"
				}

			} else if outputFile, err = os.OpenFile(outputPath, outputFlags, 0o644); err == nil {
				if outputFormat == "auto" {
					outputFormat = "json"
				}
				defer outputFile.Close()

			} else {
				log.Error().
					AnErr("error", err).
					Str("outputPath", outputPath).
					Msg("Could not open output file for writing")
				outputFile = os.Stdout
				if outputFormat == "auto" {
					outputFormat = "pretty"
				}
			}
			if outputFormat == "json" || outputFormat == "jsonl" {
				log.Info().
					Str("format", "json")
				scanner.SaveJson(outputFile)
			} else if outputFormat == "yml" || outputFormat == "yaml" {
				scanner.SaveYAML(outputFile)
			} else {
				scanner.SaveTable(outputFormat, outputFile)
			}
		}
		return
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
