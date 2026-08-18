package env

import (
	"errors"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

func Load[T any](scope map[string]any, transformers ...func(*T)) (env T, err error) {
	viper.AddConfigPath(".")
	viper.SetConfigName(".env")
	viper.SetConfigType("env")

	viper.AutomaticEnv()

	for key, value := range scope {
		viper.SetDefault(key, value)
	}

	err = viper.ReadInConfig()
	if err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			return env, err
		}
	}

	err = viper.Unmarshal(&env,
		viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
			mapstructure.TextUnmarshallerHookFunc(),
		)),
		func(config *mapstructure.DecoderConfig) {
			config.IgnoreUntaggedFields = true
		},
	)

	for _, transformer := range transformers {
		transformer(&env)
	}

	return env, err
}
