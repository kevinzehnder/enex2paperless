package main

import (
	"bufio"
	"enex2paperless/internal/config"
	"enex2paperless/pkg/enex"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/spf13/cobra"
)

func main() {
	// logging
	// opts := &slog.HandlerOptions{
	// 	Level: slog.LevelDebug,
	// }
	// logger := slog.New(logging.NewHandler(opts))
	// slog.SetDefault(logger)

	// configuration
	_, err := config.GetConfig()
	if err != nil {
		slog.Error("couldn't read config", "error", err)
		os.Exit(1)
	}

	var rootCmd = &cobra.Command{
		Use:   "myEnexParser [file path]",
		Short: "ENEX to Paperless-NGX parser",
		Long: `An ENEX file parser for Paperless-NGX.
                github.com/kevinzehnder/myEnexParser`,
		Args: cobra.MinimumNArgs(1),
		Run:  importENEX,
	}

	// Adding flags
	var howMany int
	rootCmd.PersistentFlags().IntVarP(&howMany, "concurrent", "c", 1, "Number of concurrent consumers")

	rootCmd.Execute()
}

func importENEX(cmd *cobra.Command, args []string) {
	filePath := args[0]
	// validate file path exists

	// Access the value of howMany
	howMany, err := cmd.Flags().GetInt("concurrent")
	if err != nil {
		slog.Error("failed to read flag", "error", err)
		os.Exit(1)
	}

	inputFile := enex.NewEnexFile()
	noteChannel := make(chan enex.Note)
	var failedNotes []enex.Note

	// Failure Catcher
	failedNoteChannel := make(chan enex.Note)
	failedNoteSignal := make(chan bool)
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
			err := inputFile.UploadFromNoteChannel(noteChannel, failedNoteChannel)
			// inputFile.PrintNoteInfo(noteChannel)
			if err != nil {
				slog.Error("failed to upload resources", "error", err)
				os.Exit(1)
			}
			wg.Done()
		}()
	}
	slog.Debug("waiting for WaitGroup")
	wg.Wait()
	close(failedNoteChannel)

	// wait for FailedNoteCatcher
	slog.Debug("waiting for FailedNoteCatcher")
	<-failedNoteSignal

	// log results
	slog.Info("ENEX processing done",
		slog.Int("numberOfNotes", inputFile.NumNotes),
		slog.Int("totalUploads", inputFile.Uploads),
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
			err = inputFile.UploadFromNoteChannel(retryChannel, failedNoteChannel)
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
