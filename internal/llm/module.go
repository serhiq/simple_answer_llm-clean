package llm

import "go.uber.org/fx"

func Module() fx.Option {
	return fx.Module(
		"llm",
		fx.Provide(NewClient),
	)
}
