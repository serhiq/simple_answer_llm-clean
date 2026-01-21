package internal

import (
	"context"

	"simple_answer_llm/internal/cli"
	"simple_answer_llm/internal/config"
	"simple_answer_llm/internal/evotor"
	"simple_answer_llm/internal/llm"
	"simple_answer_llm/internal/logging"

	"github.com/go-core-fx/logger"
	"go.uber.org/fx"
)

func Run() error {
	var runner *cli.Runner

	app := fx.New(
		logger.Module(),
		logger.WithFxDefaultLogger(),
		config.Module(),
		logging.Module(),
		evotor.Module(),
		llm.Module(),
		cli.Module(),
		fx.Populate(&runner),
	)

	ctx := context.Background()
	if err := app.Start(ctx); err != nil {
		return err
	}
	defer func() {
		_ = app.Stop(ctx)
	}()

	return runner.Execute()
}
