package cli

import "go.uber.org/fx"

func Module() fx.Option {
	return fx.Module(
		"cli",
		fx.Provide(NewRunner),
	)
}
