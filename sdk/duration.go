package sdk

import (
	"encoding/json"
	"errors"
	"reflect"
	"time"

	"github.com/go-viper/mapstructure/v2"
)

type Duration time.Duration

func (d *Duration) String() string {
	return time.Duration(*d).String()
}

func (d *Duration) Set(value string) (err error) {
	v, err := time.ParseDuration(value)
	if err != nil {
		return
	}
	*d = Duration(v)
	return
}

func (d *Duration) Type() string {
	return "duration"
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d *Duration) UnmarshalJSON(b []byte) (err error) {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case int64:
		*d = Duration(time.Duration(value))
		return
	case float64:
		*d = Duration(time.Duration(value))
		return
	case string:
		duration, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		*d = Duration(duration)
		return nil
	default:
		err = errors.New("invalid duration")
	}
	return
}

func DurationMapstructureHook() mapstructure.DecodeHookFuncType {
	return func(_, targetType reflect.Type, a any) (any, error) {
		if targetType.Kind() != reflect.Int64 {
			return a, nil
		}
		switch value := a.(type) {
		case int64:
			return Duration(value), nil
		case float64:
			return Duration(value), nil
		case string:
			duration, err := time.ParseDuration(value)
			if err != nil {
				return nil, err
			}
			return Duration(duration), nil
		default:
			return a, nil
		}
	}
}
