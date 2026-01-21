package logging

import (
	"context"
	"os"

	"simple_answer_llm/internal/config"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

func Module() fx.Option {
	return fx.Module(
		"logging",
		fx.Provide(func(cfg config.Config) (*os.File, error) {
			return OpenLogFile(cfg.LogFile)
		}),
		fx.Decorate(func(base *zap.Logger, cfg config.Config, file *os.File) *zap.Logger {
			return AttachFileLogger(base, file, cfg.Debug)
		}),
		fx.Invoke(func(lc fx.Lifecycle, file *os.File) {
			if file == nil {
				return
			}
			lc.Append(fx.Hook{
				OnStop: func(_ context.Context) error {
					return file.Close()
				},
			})
		}),
	)
}
