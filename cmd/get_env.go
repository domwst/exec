package cmd

import (
	"errors"
	"exec/common"
	"github.com/hashicorp/go-envparse"
	"os"
)

func loadDotEnv(dotEnvPath string) (map[string]string, error) {
	if dotEnvPath == "-" {
		return make(map[string]string), nil
	}
	file, err := os.Open(dotEnvPath)
	if errors.Is(err, os.ErrNotExist) {
		return make(map[string]string), nil
	}
	return envparse.Parse(file)
}

func GetEnv(dotEnvPath string) map[string]string {
	base, err := loadDotEnv(dotEnvPath)
	common.HandlePanic(err)
	for k, v := range ParseEnvironment(os.Environ()) {
		base[k] = v
	}
	return base
}
