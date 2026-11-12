package log

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Options struct {
	File                        string
	Console                     bool
	Level                       string
	MaxSize, MaxBackups, MaxAge int
}

func NewLogger(appName string, opts Options) (*slog.Logger, func(), error) {
	level := zap.InfoLevel
	if opts.Level != "" {
		if err := level.UnmarshalText([]byte(opts.Level)); err != nil {
			return nil, nil, errors.New("invalid zap.level")
		}
	}
	if opts.MaxSize < 0 || opts.MaxBackups < 0 || opts.MaxAge < 0 {
		return nil, nil, errors.New("zap rotation limits must be non-negative")
	}
	var writers []zapcore.WriteSyncer
	var rolling *lumberjack.Logger
	if opts.File != "" {
		if err := os.MkdirAll(filepath.Dir(opts.File), 0o700); err != nil {
			return nil, nil, err
		}
		file, err := os.OpenFile(opts.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, nil, err
		}
		if err := file.Close(); err != nil {
			return nil, nil, err
		}
		rolling = &lumberjack.Logger{
			Filename: opts.File, MaxSize: opts.MaxSize, MaxBackups: opts.MaxBackups,
			MaxAge: opts.MaxAge, Compress: true, LocalTime: true,
		}
		writers = append(writers, zapcore.AddSync(rolling))
	}
	if opts.Console {
		writers = append(writers, zapcore.AddSync(os.Stdout))
	}
	if len(writers) == 0 {
		return nil, nil, errors.New("configure zap.file or enable zap.console")
	}

	encoder := zapcore.EncoderConfig{
		TimeKey: "TS", LevelKey: "L", MessageKey: "M", CallerKey: "LFILE",
		NameKey: "logger", StacktraceKey: "stack", LineEnding: zapcore.DefaultLineEnding,
		EncodeTime: zapcore.ISO8601TimeEncoder, EncodeLevel: zapcore.LowercaseLevelEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder, EncodeCaller: zapcore.FullCallerEncoder,
	}
	core := zapcore.NewCore(zapcore.NewJSONEncoder(encoder),
		zapcore.Lock(zapcore.NewMultiWriteSyncer(writers...)), zap.NewAtomicLevelAt(level))
	host, _ := os.Hostname()
	handler := zapslog.NewHandler(core, zapslog.WithCaller(true))
	logger := slog.New(traceHandler{handler}).With("LAPP", appName, "HOST", host)
	return logger, func() {
		if rolling != nil {
			_ = rolling.Close()
		}
	}, nil
}

type traceHandler struct{ slog.Handler }

func (h traceHandler) Handle(ctx context.Context, record slog.Record) error {
	span := trace.SpanContextFromContext(ctx)
	if span.IsValid() {
		record.AddAttrs(slog.String("TRACE_ID", span.TraceID().String()), slog.String("SPAN_ID", span.SpanID().String()))
	}
	return h.Handler.Handle(ctx, record)
}

func (h traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return traceHandler{h.Handler.WithAttrs(attrs)}
}

func (h traceHandler) WithGroup(name string) slog.Handler {
	return traceHandler{h.Handler.WithGroup(name)}
}
