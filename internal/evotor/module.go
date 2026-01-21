package evotor

import "go.uber.org/fx"

func Module() fx.Option {
	return fx.Module(
		"evotor",
		fx.Provide(NewClient),
	)
}
