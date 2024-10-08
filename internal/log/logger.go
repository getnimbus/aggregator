package log

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger interface {
	Debug(msg string, ctx ...interface{})
	Info(msg string, ctx ...interface{})
	Warn(msg string, ctx ...interface{})
	Error(msg string, ctx ...interface{})
	Crit(msg string, ctx ...interface{})
	Trace(msg string, ctx ...interface{})
	NewLogger(ctx ...interface{}) Logger
	SetLevel(lvl zapcore.Level) Logger
	SetLevelString(lvlString string) Logger
	NewContextLogger(c context.Context, ctx ...interface{}) (context.Context, Logger)
}

type loggerCtx struct {
}

type logger struct {
	*zap.Logger
}

func (l *logger) NewLogger(ctx ...interface{}) Logger {
	newLogger := l.With(convertCtxToZapFields(ctx)...)
	return &logger{Logger: newLogger}
}

func (l *logger) NewContextLogger(c context.Context, ctx ...interface{}) (context.Context, Logger) {
	nl := l.NewLogger(ctx...)
	return WithContext(c, nl), nl
}

func (l *logger) SetLevel(lvl zapcore.Level) Logger {
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(lvl)
	newLogger, _ := config.Build()
	l.Logger = newLogger
	return l
}

func (l *logger) SetLevelString(lvlString string) Logger {
	var lvl zapcore.Level
	if err := lvl.UnmarshalText([]byte(lvlString)); err != nil {
		return l
	}
	return l.SetLevel(lvl)
}

func (l *logger) Debug(msg string, ctx ...interface{}) {
	l.Logger.Debug(msg, convertCtxToZapFields(ctx)...)
}

func (l *logger) Info(msg string, ctx ...interface{}) {
	l.Logger.Info(msg, convertCtxToZapFields(ctx)...)
}

func (l *logger) Warn(msg string, ctx ...interface{}) {
	l.Logger.Warn(msg, convertCtxToZapFields(ctx)...)
}

func (l *logger) Error(msg string, ctx ...interface{}) {
	l.Logger.Error(msg, convertCtxToZapFields(ctx)...)
}

func (l *logger) Crit(msg string, ctx ...interface{}) {
	l.Logger.Error(msg, convertCtxToZapFields(ctx)...)
}

func (l *logger) Trace(msg string, ctx ...interface{}) {
	l.Logger.Debug(msg, convertCtxToZapFields(ctx)...)
}

var root *logger
var moduleLogs sync.Map

func init() {
	config := zap.NewProductionConfig()
	newLogger, _ := config.Build()
	root = &logger{Logger: newLogger}
	root.SetLevelString("debug")
}

func Module(module string) Logger {
	if module == "root" {
		return Root()
	}
	logI, ok := moduleLogs.Load(module)
	if !ok {
		log := newModule(module)
		moduleLogs.Store(module, log)
		return log
	}
	log, ok := logI.(Logger)
	if !ok {
		log = newModule(module)
		moduleLogs.Store(module, log)
		return log
	}
	return log
}

func newModule(module string) Logger {
	log := Root().NewLogger(zap.String("module", module))
	return log
}

// New returns a new logger with the given context.
func New(ctx ...interface{}) Logger {
	return root.NewLogger(ctx...)
}

// Root returns the root logger
func Root() Logger {
	return root
}

// Debug is a convenient alias for Root().Debug
func Debug(msg string, ctx ...interface{}) {
	root.Debug(msg, ctx...)
}

// Info is a convenient alias for Root().Info
func Info(msg string, ctx ...interface{}) {
	root.Info(msg, ctx...)
}

// Warn is a convenient alias for Root().Warn
func Warn(msg string, ctx ...interface{}) {
	root.Warn(msg, ctx...)
}

// Error is a convenient alias for Root().Error
func Error(msg string, ctx ...interface{}) {
	root.Error(msg, ctx...)
}

// Crit is a convenient alias for Root().Crit
func Crit(msg string, ctx ...interface{}) {
	root.Crit(msg, ctx...)
}

// Trace is a convenient alias for Root().Trace
func Trace(msg string, ctx ...interface{}) {
	root.Trace(msg, ctx...)
}

func StackError(msg string, err error) {
	root.Error(fmt.Sprintf("%s   => error : %+v", msg, err))
}

func SetLevel(lvl zapcore.Level) Logger {
	return Root().SetLevel(lvl)
}

func SetLevelString(lvlString string) Logger {
	return Root().SetLevelString(lvlString)
}

// WithContext context log
func WithContext(ctx context.Context, l Logger) context.Context {
	if l == nil {
		l = Root()
	}
	k := loggerCtx{}
	ctx = context.WithValue(ctx, k, l)
	return ctx
}

// Helper function to convert context params (ctx ...interface{}) to zap fields
func convertCtxToZapFields(ctx []interface{}) []zap.Field {
	var fields []zap.Field
	for i := 0; i < len(ctx); i += 2 {
		if i+1 < len(ctx) {
			key, okKey := ctx[i].(string)
			value := ctx[i+1]
			if okKey {
				fields = append(fields, zap.Any(key, value))
			}
		}
	}
	return fields
}
