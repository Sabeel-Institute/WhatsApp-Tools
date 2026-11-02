package real

import (
	"context"
	"fmt"
	"log/slog"

	waLog "go.mau.fi/whatsmeow/util/log"
)

// slogAdapter routes whatsmeow's logs into the app's redacting slog logger.
// whatsmeow's debug output names people by their WhatsApp IDs, so the app
// only passes through Info and above, and the file writer redacts numbers.
type slogAdapter struct {
	l   *slog.Logger
	min slog.Level
}

func newLogger(l *slog.Logger, min slog.Level) waLog.Logger {
	return &slogAdapter{l: l.With("component", "whatsmeow"), min: min}
}

func (a *slogAdapter) log(level slog.Level, msg string, args ...any) {
	if level < a.min || !a.l.Enabled(context.Background(), level) {
		return
	}
	a.l.Log(context.Background(), level, fmt.Sprintf(msg, args...))
}

func (a *slogAdapter) Warnf(msg string, args ...any)  { a.log(slog.LevelWarn, msg, args...) }
func (a *slogAdapter) Errorf(msg string, args ...any) { a.log(slog.LevelError, msg, args...) }
func (a *slogAdapter) Infof(msg string, args ...any)  { a.log(slog.LevelInfo, msg, args...) }
func (a *slogAdapter) Debugf(msg string, args ...any) { a.log(slog.LevelDebug, msg, args...) }
func (a *slogAdapter) Sub(module string) waLog.Logger {
	return &slogAdapter{l: a.l.With("module", module), min: a.min}
}
