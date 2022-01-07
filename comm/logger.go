package comm

import (
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"os"
	"sync"
)

var (
	loggerInitializer sync.Once
	sugarLogger       *zap.SugaredLogger
)

func GetLogger() *zap.SugaredLogger {
	loggerInitializer.Do(func() {
		initLogger()
	})
	return sugarLogger
}

type ApplyLoggerOption func(l *loggerOption)

func WithLoggerFile(fileName string) ApplyLoggerOption {
	return func(l *loggerOption) {
		if fileName != "" {
			writer, err := os.Create(fileName)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			l.loggerFile = writer
		}
	}
}

func WithLoggerLevel(level string) ApplyLoggerOption {
	return func(l *loggerOption) {
		logLevel := zap.InfoLevel
		switch level {
		case "debug":
			logLevel = zap.DebugLevel
		case "info":
			logLevel = zap.InfoLevel
		case "warn":
			logLevel = zap.WarnLevel
		case "error":
			logLevel = zap.ErrorLevel
		case "panic":
			logLevel = zap.PanicLevel
		case "fatal":
			logLevel = zap.FatalLevel
		default:
		}

		l.level = logLevel
	}
}

type loggerOption struct {
	loggerFile io.Writer
	level      zapcore.Level
}

func initLogger(opts ...ApplyLoggerOption) {
	defaultOpt := &loggerOption{level: zap.InfoLevel}
	for _, opt := range opts {
		opt(defaultOpt)
	}

	writeSyncer := getLogWriter(defaultOpt.loggerFile)
	encoder := getEncoder()
	core := zapcore.NewCore(encoder, writeSyncer, defaultOpt.level)
	zapLogger := zap.New(core)
	sugarLogger = zapLogger.Sugar()
}

func getEncoder() zapcore.Encoder {
	return zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
}

func getLogWriter(writer io.Writer) zapcore.WriteSyncer {
	if writer == nil {
		return zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout))
	}

	return zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), zapcore.AddSync(writer))
}
