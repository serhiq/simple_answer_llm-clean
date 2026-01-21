package logging

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func OpenLogFile(logFile string) (*os.File, error) {
	if logFile == "" {
		return nil, nil
	}

	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}

	return file, nil
}

func AttachFileLogger(base *zap.Logger, file *os.File, debug bool) *zap.Logger {
	if file == nil {
		return base
	}

	encCfg := zap.NewProductionEncoderConfig()
	level := zap.InfoLevel
	if debug {
		level = zap.DebugLevel
	}

	fileCore := zapcore.NewCore(zapcore.NewJSONEncoder(encCfg), zapcore.AddSync(file), level)
	return base.WithOptions(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		return zapcore.NewTee(core, fileCore)
	}))
}
