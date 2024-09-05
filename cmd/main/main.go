package main

import (
	"bufio"
	"enex2paperless/internal/config"
	"enex2paperless/internal/logging"
	"enex2paperless/pkg/enex"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/spf13/cobra"
)

func main() {
	// define root command
	rootCmd := &cobra.Command{
		Use:   "enex2paperless [file path]",
		Short: "ENEX to Paperless-NGX parser",
		Long:  `An ENEX file parser for Paperless-NGX. https://github.com/kevinzehnder/enex2paperless`,
		Args:  cobra.MinimumNArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			// this block will execute after flag parsing and before the main Run

			// configure SLOG with the determined log level from verbose flag
			verbose, err := cmd.Flags().GetBool("verbose") // Ensure to get the flag value correctly
			if err != nil {
				fmt.Println("Error retrieving verbose flag:", err)
				os.Exit(1)
			}

			// set log level
			var logLevel slog.Level
			if verbose {
				logLevel = slog.LevelDebug
			} else {
				logLevel = slog.LevelInfo
			}

			// nocolor option
			nocolor, err := cmd.Flags().GetBool("nocolor")
			if err != nil {
				fmt.Println("Error retrieving nocolor flag:", err)
				os.Exit(1)
			}

			opts := &slog.HandlerOptions{
				Level: logLevel,
			}
			// use default slog TextHandler
			// logger := slog.New(slog.NewTextHandler(os.Stdout, opts))

			// use custom slog Handler
			logger := slog.New(logging.NewHandler(opts, nocolor))
			slog.SetDefault(logger)

			// handle configuration
			settings, err := config.GetConfig()
			if err != nil {
				slog.Error("couldn't read config", "error", err)
				os.Exit(1)
			}
			slog.Debug(fmt.Sprintf("configuration: %v", settings))

			// add to configuration
			outputfolder, err := cmd.Flags().GetString("outputfolder")
			if err != nil {
				fmt.Println("Error retrieving outputfolder flag:", err)
				os.Exit(1)
			}

			if outputfolder != "" {
				config.SetOutputFolder(outputfolder)
			}
		},

		// run main function
		Run: importENEX,
	}

	// add flags
	var howMany int
	rootCmd.PersistentFlags().IntVarP(&howMany, "concurrent", "c", 1, "Number of concurrent consumers")

	var verbose bool
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")

	var nocolor bool
	rootCmd.PersistentFlags().BoolVarP(&nocolor, "nocolor", "n", false, "Disable colored output")

	var outputfolder string
	rootCmd.PersistentFlags().StringVarP(&outputfolder, "outputfolder", "o", "", "Output attachements to this folder, NOT paperless.")

	// run root command
	err := rootCmd.Execute()
	if err != nil {
		fmt.Println("Error executing command:", err)
		os.Exit(1)
	}
}

func importENEX(cmd *cobra.Command, args []string) {
	slog.Debug("starting importENEX")
	settings, _ := config.GetConfig()

	if settings.OutputFolder != "" {
		slog.Info(fmt.Sprintf("Output to local storage is enabled. Target is: %v", settings.OutputFolder))
	}

	// determine how many concurrent uploaders we want
	howMany, err := cmd.Flags().GetInt("concurrent")
	if err != nil {
		slog.Error("failed to read flag", "error", err)
		os.Exit(1)
	}

	// prepare input file
	filePath := args[0]
	inputFile := enex.NewEnexFile()

	// prepare channels
	noteChannel := make(chan enex.Note)
	failedNoteChannel := make(chan enex.Note)
	failedNoteSignal := make(chan bool)

	// Failure Catcher
	var failedNotes []enex.Note
	go func() {
		enex.FailedNoteCatcher(failedNoteChannel, &failedNotes)
		failedNoteSignal <- true
	}()

	// Producer
	go func() {
		err := inputFile.ReadFromFile(filePath, noteChannel)
		if err != nil {
			slog.Error("failed to read from file", "error", err)
			os.Exit(1)
		}
	}()

	// Consumers
	var wg sync.WaitGroup
	wg.Add(howMany)

	for i := 0; i < howMany; i++ {
		go func() {
			err := inputFile.UploadFromNoteChannel(noteChannel, failedNoteChannel, settings.OutputFolder)
			// inputFile.PrintNoteInfo(noteChannel)
			if err != nil {
				slog.Error("failed to upload resources", "error", err)
				os.Exit(1)
			}

			wg.Done()
		}()
	}
	slog.Debug("waiting for Consumers (WaitGroup)")
	wg.Wait()

	// close failedNoteChannel when consumers are done
	close(failedNoteChannel)

	// wait for FailedNoteCatcher
	slog.Debug("waiting for FailedNoteCatcher")
	<-failedNoteSignal

	// log results
	slog.Info("ENEX processing done",
		slog.Int("numberOfNotes", int(inputFile.NumNotes.Load())),
		slog.Int("totalFiles", int(inputFile.Uploads.Load())),
	)

	for {
		// if we still have failedNotes in this iteration, keep going
		if len(failedNotes) == 0 {
			break
		}

		slog.Warn("there have been errors, starting retry cycle", "errors", len(failedNotes))
		PressKeyToContinue()

		// all failed notes are now in failedNotes slice
		// push notes that failed this Cycle into failedThisCycle slice
		failedThisCycle := []enex.Note{}

		// reset failedNoteChannel
		failedNoteChannel = make(chan enex.Note)

		// this feeds the failedNotes slice into the failedNoteChannel
		go func() {
			enex.FailedNoteCatcher(failedNoteChannel, &failedThisCycle)
			failedNoteSignal <- true
		}()

		// this feeds the failedNotes into the Retry Channel
		retryChannel := make(chan enex.Note)
		go enex.RetryFeeder(&failedNotes, retryChannel)

		// this works on the retry channel
		wg.Add(1)
		go func() {
			err = inputFile.UploadFromNoteChannel(retryChannel, failedNoteChannel, settings.OutputFolder)
			// inputFile.PrintNoteInfo(noteChannel)
			if err != nil {
				slog.Error("failed to upload resources", "error", err)
				os.Exit(1)
			}
			wg.Done()
		}()
		wg.Wait()

		// when the uploader is done, we can close the failedNoteChannel
		// to signal to the FailedNote Catcher that it can stop
		close(failedNoteChannel)

		// then we wait for the FailedNoteCatcher to stop
		<-failedNoteSignal

		// we move the notes that failed this cycle into the failedNotes variable
		failedNotes = failedThisCycle
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
