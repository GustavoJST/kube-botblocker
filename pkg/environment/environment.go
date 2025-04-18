package environment

import (
	"github.com/caarlos0/env/v11"
)

type OperatorEnv struct {
	OperatorNamespace    string `env:"OPERATOR_NAMESPACE,required"`
	CurrentNamespaceOnly bool   `env:"CURRENT_NAMESPACE_ONLY" envDefault:"false"`
}

func GetOperatorEnv() (*OperatorEnv, error) {
	var cfg OperatorEnv
	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
