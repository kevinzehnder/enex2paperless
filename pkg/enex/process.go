package enex

import (
	"fmt"
	"log/slog"
	"os"
	"sync"
)

// ProcessOptions configures the processing behavior
type ProcessOptions struct {
	// ConcurrentWorkers specifies how many concurrent upload workers to use
	ConcurrentWorkers int

	// OutputFolder specifies where to save files locally (empty string for Paperless upload)
	OutputFolder string

	// RetryPromptFunc is called when there are failed notes, allowing the caller
	// to decide whether to retry. Return true to retry, false to stop.
	// If nil, retries are automatically attempted without prompting.
	RetryPromptFunc func(failedCount int) bool
}

// ProcessResult contains the results of processing
type ProcessResult struct {
	// NotesProcessed is the total number of notes processed
	NotesProcessed int

	// FilesUploaded is the total number of files successfully uploaded
	FilesUploaded int

	// FailedNotes contains any notes that failed processing after all retries
	FailedNotes []Note
}

// Process orchestrates the complete ENEX processing workflow:
// - Reads notes from the file
// - Uploads files with concurrent workers
// - Handles failures and retries
// - Returns results and any remaining failures
func (e *EnexFile) Process(opts ProcessOptions) (*ProcessResult, error) {
	// Validate we have a file to process
	if e.FilePath == "" {
		return nil, fmt.Errorf("no file path provided")
	}

	_, err := e.Fs.Stat(e.FilePath)
	if err != nil {
		return nil, fmt.Errorf("cannot access file %s: %w", e.FilePath, err)
	}

	// Set defaults
	if opts.ConcurrentWorkers <= 0 {
		opts.ConcurrentWorkers = 1
	}

	// Failure Catcher
	var failedNotes []Note
	go func() {
		e.FailedNoteCatcher(&failedNotes)
		e.FailedNoteSignal <- true
	}()

	// Producer: read from file and feed notes to channel
	go func() {
		err := e.ReadFromFile()
		if err != nil {
			slog.Error("failed to read from file", "error", err)
			// critical error, cant read file -> exit
			os.Exit(1)
		}
	}()

	// Consumers: spawn concurrent upload workers
	var wg sync.WaitGroup
	wg.Add(opts.ConcurrentWorkers)

	for i := 0; i < opts.ConcurrentWorkers; i++ {
		go func(workerID int) {
			err := e.UploadFromNoteChannel(opts.OutputFolder)
			if err != nil {
				slog.Error("worker failed to upload resources",
					"workerID", workerID,
					"error", err)
			}
			wg.Done()
		}(i)
	}

	slog.Debug("waiting for upload workers to complete")
	wg.Wait()

	// Close failedNoteChannel when consumers are done
	close(e.FailedNoteChannel)

	// Wait for FailedNoteCatcher to finish
	slog.Debug("waiting for FailedNoteCatcher")
	<-e.FailedNoteSignal

	// Log initial results
	notesProcessed := int(e.NumNotes.Load())
	filesUploaded := int(e.Uploads.Load())

	slog.Info("ENEX processing complete",
		slog.Int("notesProcessed", notesProcessed),
		slog.Int("filesUploaded", filesUploaded),
	)

	// Retry loop for failed notes
	for {
		// If no failed notes, we're done
		if len(failedNotes) == 0 {
			break
		}

		slog.Warn("notes failed to process",
			slog.Int("failedCount", len(failedNotes)),
		)

		// Check if we should retry
		shouldRetry := true
		if opts.RetryPromptFunc != nil {
			shouldRetry = opts.RetryPromptFunc(len(failedNotes))
		}

		if !shouldRetry {
			// User chose not to retry, break out
			break
		}

		slog.Info("retrying failed notes",
			slog.Int("retryCount", len(failedNotes)),
		)

		// Prepare for retry
		failedThisCycle := []Note{}

		// Create a fresh EnexFile for the retry (no file path since we're feeding notes)
		retryFile := NewEnexFile("", e.config)

		// Start failure catcher for this retry
		go func() {
			retryFile.FailedNoteCatcher(&failedThisCycle)
			retryFile.FailedNoteSignal <- true
		}()

		// Feed the failed notes into the retry channel
		go retryFile.RetryFeeder(&failedNotes)

		// Start a single worker for retry
		wg.Add(1)
		go func() {
			err := retryFile.UploadFromNoteChannel(opts.OutputFolder)
			if err != nil {
				slog.Error("retry worker failed", "error", err)
			}
			wg.Done()
		}()

		// Wait for retry to complete
		wg.Wait()

		// Close the retry file's failed note channel
		close(retryFile.FailedNoteChannel)

		// Wait for the failure catcher to finish
		<-retryFile.FailedNoteSignal

		// Update metrics with retry results
		filesUploaded += int(retryFile.Uploads.Load())

		// Move notes that failed this cycle into failedNotes for next iteration
		failedNotes = failedThisCycle
	}

	// Final results
	result := &ProcessResult{
		NotesProcessed: notesProcessed,
		FilesUploaded:  filesUploaded,
		FailedNotes:    failedNotes,
	}

	if len(failedNotes) > 0 {
		return result, fmt.Errorf("%d notes failed to process", len(failedNotes))
	}

	slog.Info("all notes processed successfully")
	return result, nil
}
