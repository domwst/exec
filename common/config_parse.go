package common

import (
	"encoding/json"
	"exec/tools"
	"io"
	"os"
	"reflect"
)

type stringOrEnv string

func (s stringOrEnv) toString(env map[string]string) string {
	if len(s) == 0 {
		return string(s)
	}
	if s[0] == '$' {
		return env[string(s)[1:]]
	}
	return string(s)
}

func alterStringFields(config any, alterRule func(string) string) {
	configValue := reflect.ValueOf(config).Elem()

	for i := 0; i < configValue.NumField(); i++ {
		fieldValue := configValue.Field(i)

		if fieldValue.Kind() == reflect.String {
			fieldValue.SetString(alterRule(fieldValue.String()))
		}

		if fieldValue.Kind() == reflect.Struct {
			alterStringFields(fieldValue.Addr().Interface(), alterRule)
		}
	}
}

func ParseConfigFileWithRespectToEnv(filename string, env map[string]string, config any) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer func() {
		tools.HandlePanic(file.Close())
	}()
	bytes, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	err = json.Unmarshal(bytes, config)
	if err != nil {
		return err
	}
	alterStringFields(config, func(s string) string {
		return stringOrEnv(s).toString(env)
	})
	return nil
}
