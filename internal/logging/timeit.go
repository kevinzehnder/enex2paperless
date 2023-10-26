package logging

import (
	"fmt"
	"log/slog"
	"time"
)

// timeit measures execution time and prints it to stdout
func Timeit(start time.Time) {
	end := time.Now()
	runtime := end.Sub(start)
	slog.Info(fmt.Sprintf("runtime: %s", runtime))
}
