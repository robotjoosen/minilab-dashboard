package env

import (
	"fmt"
	"strings"
)

const (
	MapConfigEntrySeparator    = ","
	MapConfigKeyValueSeparator = ":"
)

type InvalidKeyValueError struct {
	data string
}

func (err InvalidKeyValueError) Error() string {
	return fmt.Sprintf("invalid key/value pair %s", err.data)
}

// MapConfig parses key/value environment variables format as `foo:bar,lorem:ipsum`
type MapConfig map[string]string

func (c *MapConfig) UnmarshalText(text []byte) error {
	cfg := make(map[string]string)

	kvs := strings.Split(string(text), MapConfigEntrySeparator)
	for _, kvString := range kvs {
		kv := strings.Split(kvString, MapConfigKeyValueSeparator)
		if len(kv) != 2 {
			return &InvalidKeyValueError{data: kvString}
		}

		cfg[kv[0]] = kv[1]
	}

	*c = cfg

	return nil
}

func (c *MapConfig) Mapped() map[string]string {
	m := make(map[string]string)
	for k, v := range *c {
		m[k] = v
	}

	return m
}
