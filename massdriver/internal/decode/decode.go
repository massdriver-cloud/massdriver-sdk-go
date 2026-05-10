// Package decode bridges genqlient-generated response structs to the
// hand-written public types exposed by massdriver/platform/*.
//
// We use mapstructure (rather than JSON roundtripping) because the genqlient
// scalar bindings already decode Massdriver's `JSON` and `Map` scalars into
// `map[string]any` — re-marshaling those through encoding/json would re-invoke
// the scalar marshaler and produce the doubly-encoded wire form, not the
// in-memory shape the wrapper expects.
package decode

import (
	"reflect"
	"time"

	"github.com/go-viper/mapstructure/v2"
)

// Decode copies a genqlient-generated struct into a public API struct.
func Decode(input, output any) error {
	cfg := &mapstructure.DecoderConfig{
		Result:     output,
		DecodeHook: timeWrapHook,
	}
	dec, err := mapstructure.NewDecoder(cfg)
	if err != nil {
		return err
	}
	return dec.Decode(input)
}

const timeWrapKey = "__time__"

var (
	timeType    = reflect.TypeOf(time.Time{})
	timePtrType = reflect.TypeOf(&time.Time{})
)

// timeWrapHook preserves time.Time values through mapstructure's internal
// struct→map→struct dance by wrapping them in a single-key map on the way out
// and unwrapping on the way in. Without this, mapstructure treats time.Time as
// a generic struct, flattens it (hitting only unexported fields → empty map),
// and re-inflates the empty map; both passes lose the value.
func timeWrapHook(from, to reflect.Type, data any) (any, error) {
	if from == timeType || from == timePtrType {
		var t time.Time
		switch d := data.(type) {
		case time.Time:
			t = d
		case *time.Time:
			if d != nil {
				t = *d
			}
		default:
			return data, nil
		}
		return map[string]any{timeWrapKey: t.Format(time.RFC3339Nano)}, nil
	}
	if to == timeType {
		if m, ok := data.(map[string]any); ok {
			if s, ok := m[timeWrapKey].(string); ok {
				return time.Parse(time.RFC3339Nano, s)
			}
		}
	}
	return data, nil
}
