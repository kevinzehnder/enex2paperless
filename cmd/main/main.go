package main

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"enex2paperless/internal/config"
	"enex2paperless/internal/logging"
	"enex2paperless/pkg/enex"

	"github.com/spf13/cobra"
)

// CLI flag variables
var (
	howMany          int
	verbose          bool
	nocolor          bool
	outputfolder     string
	tags             []string
	useFilenameAsTag bool
)

func main() {
	// define root command
	rootCmd := &cobra.Command{
		Use:   "enex2paperless [file path]",
		Short: "ENEX to Paperless-NGX parser",
		Long:  `An ENEX file parser for Paperless-NGX. https://github.com/kevinzehnder/enex2paperless`,
		Args:  cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// validate concurrent workers
			if howMany < 1 {
				return fmt.Errorf("concurrent workers must be at least 1, got %d", howMany)
			}

			// validate output folder if specified
			if outputfolder != "" {
				info, err := os.Stat(outputfolder)
				if err != nil {
					if os.IsNotExist(err) {
						return fmt.Errorf("output folder does not exist: %s", outputfolder)
					}
					return fmt.Errorf("cannot access output folder: %w", err)
				}
				if !info.IsDir() {
					return fmt.Errorf("output folder is not a directory: %s", outputfolder)
				}
			}

			// validate input file exists
			if _, err := os.Stat(args[0]); err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("input file does not exist: %s", args[0])
				}
				return fmt.Errorf("cannot access input file: %w", err)
			}

			// set log level based on verbose flag
			var logLevel slog.Level
			if verbose {
				logLevel = slog.LevelDebug
			} else {
				logLevel = slog.LevelInfo
			}

			opts := &slog.HandlerOptions{
				Level: logLevel,
			}

			// use custom slog Handler
			logger := slog.New(logging.NewHandler(opts, nocolor))
			slog.SetDefault(logger)

			return nil
		},

		// run main function
		Run: importENEX,
	}

	// add flags
	rootCmd.PersistentFlags().IntVarP(&howMany, "concurrent", "c", 1, "Number of concurrent consumers")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	rootCmd.PersistentFlags().BoolVarP(&nocolor, "nocolor", "n", false, "Disable colored output")
	rootCmd.PersistentFlags().StringVarP(&outputfolder, "outputfolder", "o", "", "Output attachements to this folder, NOT paperless.")
	rootCmd.PersistentFlags().StringSliceVarP(&tags, "tags", "t", nil, "Additional tags to add to all documents.")
	rootCmd.PersistentFlags().BoolVarP(&useFilenameAsTag, "use-filename-tag", "T", false, "Add the ENEX filename as tag to all documents.")

	// run root command
	err := rootCmd.Execute()
	if err != nil {
		// cobra prints error message, we just handle exit code
		os.Exit(1)
	}
}

func importENEX(cmd *cobra.Command, args []string) {
	slog.Debug("starting importENEX")
	settings, _ := config.GetConfig()

	// Apply flag overrides to config
	if outputfolder != "" {
		settings.OutputFolder = outputfolder
	}

	if useFilenameAsTag {
		baseName := filepath.Base(args[0])
		tagName := strings.TrimSuffix(baseName, filepath.Ext(baseName))
		tags = append(tags, tagName)
	}
	if len(tags) > 0 {
		settings.AdditionalTags = tags
	}

	if settings.OutputFolder != "" {
		slog.Info(fmt.Sprintf("Output to local storage is enabled. Target is: %v", settings.OutputFolder))
	}

	// Prepare input file with initialized channels
	filePath := args[0]
	inputFile := enex.NewEnexFile(filePath, settings)

	// Process the ENEX file with retry prompts
	result, err := inputFile.Process(enex.ProcessOptions{
		ConcurrentWorkers: howMany,
		OutputFolder:      settings.OutputFolder,
		RetryPromptFunc: func(failedCount int) bool {
			// Prompt user whether to retry failed notes
			slog.Warn("there have been errors, starting retry cycle", "errors", failedCount)
			PressKeyToContinue()
			return true
		},
	})

	if err != nil {
		slog.Error("processing completed with errors", "error", err)
		if len(result.FailedNotes) > 0 {
			slog.Error("some notes could not be processed", "failedCount", len(result.FailedNotes))
		}
		os.Exit(1)
	}

	slog.Info("all notes processed successfully")
}

func PressKeyToContinue() {
	fmt.Println("Press 'x' to exit or any other key to continue.")
	for {
		key := getUserInput()
		if key == "x" {
			fmt.Println("Exiting...")
			os.Exit(1)
		} else {
			return
		}
	}
}

func getUserInput() string {
	reader := bufio.NewReader(os.Stdin)
	char, _, err := reader.ReadRune()
	if err != nil {
		fmt.Println("Error reading input:", err)
		return ""
	}

	return string(char)
}
