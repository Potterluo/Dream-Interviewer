//go:build desktop

package main

// Desktop bootstrap: where the data lives, where the log goes, and how a fatal
// error is reported.
//
// WHY THIS EXISTS. cmd/desktop is built with `-H windowsgui`, which means the
// process has NO console: anything written to stdout/stderr goes nowhere. The
// old code did `slog.Error("boot", ...)` and `os.Exit(1)`, so a boot failure
// was completely invisible — double-clicking the exe did nothing at all, with
// no window, no message, and no file to inspect.
//
// It also hard-overrode the data dir with %APPDATA%\... and ignored
// APP_DATA_DIR, so if that one path was not writable there was no way to make
// the app work. That is not hypothetical: %APPDATA% can be unavailable (roaming
// profile disabled, folder redirected to OneDrive, blocked by EDR/AV, or an
// inherited sandbox) and the app simply refused to start.

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

const appDirName = "github.com/Potterluo/dream-interviewer"

// desktopLogPath is the file the log was redirected to ("" when unavailable),
// so a fatal dialog can tell the user where to look.
var desktopLogPath string

// openDesktopLog points the default logger at a file, because stdout does not
// exist in a windowsgui build. It tries, in order: next to the executable, then
// %TEMP%. If both fail it leaves logging on stdout (harmless, just invisible).
func openDesktopLog() {
	writers := []io.Writer{os.Stdout}

	var candidates []string
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "dream-interviewer-desktop.log"))
	}
	candidates = append(candidates, filepath.Join(os.TempDir(), "dream-interviewer-desktop.log"))

	for _, path := range candidates {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			continue
		}
		desktopLogPath = path
		writers = append(writers, f)
		break
	}

	// MultiWriter so `go run ./cmd/desktop` still prints to the terminal.
	slog.SetDefault(slog.New(slog.NewTextHandler(io.MultiWriter(writers...),
		&slog.HandlerOptions{Level: slog.LevelInfo})))
}

// candidateDataDirs lists where the app is willing to keep its data, in order
// of preference.
func candidateDataDirs() []string {
	var out []string
	// An explicit override wins, and is the escape hatch when the standard
	// locations are not usable.
	if v := os.Getenv("APP_DATA_DIR"); v != "" {
		out = append(out, v)
	}
	if dir, err := os.UserConfigDir(); err == nil { // %APPDATA% on Windows
		out = append(out, filepath.Join(dir, appDirName))
	}
	if dir, err := os.UserCacheDir(); err == nil { // %LOCALAPPDATA% on Windows
		out = append(out, filepath.Join(dir, appDirName))
	}
	if exe, err := os.Executable(); err == nil {
		// Last resort: portable-style next to the binary. Also the case that
		// makes a downloaded exe work when the profile is read-only.
		out = append(out, filepath.Join(filepath.Dir(exe), "data"))
	}
	return out
}

// pickWritableDataDir returns the first candidate it can actually write to,
// along with the reasons the earlier ones were rejected.
//
// "Can create the directory" is not enough: a directory can exist and still
// reject file creation (read-only ACL, full disk, sandbox), which is exactly
// what produced SQLITE_CANTOPEN / "readonly database" later on. So it probes
// with a real file.
func pickWritableDataDir() (string, []string) {
	var problems []string
	for _, dir := range candidateDataDirs() {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			problems = append(problems, fmt.Sprintf("%s — %v", dir, err))
			continue
		}
		probe := filepath.Join(dir, ".write-probe")
		f, err := os.Create(probe)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s — %v", dir, err))
			continue
		}
		_ = f.Close()
		_ = os.Remove(probe)
		return dir, problems
	}
	return "", problems
}

// fatalDesktop reports an unrecoverable startup error where the user can
// actually see it, then exits.
func fatalDesktop(summary string, err error) {
	msg := summary
	if err != nil {
		msg += ": " + err.Error()
	}
	if problems := lastDataDirProblems; len(problems) > 0 {
		msg += "\n\n尝试过的数据目录：\n"
		for _, p := range problems {
			msg += "· " + p + "\n"
		}
	}
	if desktopLogPath != "" {
		msg += "\n完整日志：" + desktopLogPath
	} else {
		msg += "\n\n（无法写入日志文件）"
	}
	slog.Error(summary, "err", err)
	showError("Dream Interviewer 无法启动", msg)
	os.Exit(1)
}

// lastDataDirProblems records why earlier data dirs were rejected, so the fatal
// dialog can show all of them at once instead of only the last.
var lastDataDirProblems []string
