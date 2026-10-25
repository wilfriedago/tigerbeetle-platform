package grpcserver

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"google.golang.org/grpc/grpclog"
)

// routeOnce guards grpc-go's logger. grpc-go keeps it in a package-level variable that is not
// safe to set while gRPC runs, so it is set once per process, at start-up, the exception ADR-0021
// makes as ADR-0018 does for Huma.
var routeOnce sync.Once

// RouteLogs sends grpc-go's own log lines (connection and transport events) to logger. Call it
// once at process start, before anything uses gRPC; later calls do nothing. Without it grpc-go
// logs errors to stderr in its own format.
func RouteLogs(logger *slog.Logger) {
	routeOnce.Do(func() { grpclog.SetLoggerV2(slogLogger{logger: logger.With(slog.String("component", "grpc"))}) })
}

// slogLogger adapts slog to grpclog.LoggerV2. grpc-go's info lines are connection chatter, so
// they go out at debug level.
type slogLogger struct {
	logger *slog.Logger
}

func (l slogLogger) log(level slog.Level, msg string) {
	l.logger.Log(context.Background(), level, msg)
}

func (l slogLogger) Info(args ...any) { l.log(slog.LevelDebug, fmt.Sprint(args...)) }

func (l slogLogger) Infoln(args ...any) { l.log(slog.LevelDebug, fmt.Sprint(args...)) }

func (l slogLogger) Infof(format string, args ...any) {
	l.log(slog.LevelDebug, fmt.Sprintf(format, args...))
}
func (l slogLogger) Warning(args ...any)   { l.log(slog.LevelWarn, fmt.Sprint(args...)) }
func (l slogLogger) Warningln(args ...any) { l.log(slog.LevelWarn, fmt.Sprint(args...)) }

func (l slogLogger) Warningf(format string, args ...any) {
	l.log(slog.LevelWarn, fmt.Sprintf(format, args...))
}

func (l slogLogger) Error(args ...any) { l.log(slog.LevelError, fmt.Sprint(args...)) }

func (l slogLogger) Errorln(args ...any) { l.log(slog.LevelError, fmt.Sprint(args...)) }

func (l slogLogger) Errorf(format string, args ...any) {
	l.log(slog.LevelError, fmt.Sprintf(format, args...))
}

// Fatal logs and exits, as grpclog.LoggerV2 requires.
func (l slogLogger) Fatal(args ...any) {
	l.log(slog.LevelError, fmt.Sprint(args...))
	os.Exit(1)
}

func (l slogLogger) Fatalln(args ...any) {
	l.log(slog.LevelError, fmt.Sprint(args...))
	os.Exit(1)
}

func (l slogLogger) Fatalf(format string, args ...any) {
	l.log(slog.LevelError, fmt.Sprintf(format, args...))
	os.Exit(1)
}

// V reports whether verbose logs of level v are on; they are off, as by default in grpc-go.
func (slogLogger) V(v int) bool { return v <= 0 }
