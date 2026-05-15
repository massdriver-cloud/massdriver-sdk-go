package scalars

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// MarshalConditions encodes a [types.PolicyConditions] into the GraphQL
// `Conditions` scalar's wire form — a JSON-encoded string that itself
// contains the bare JSON object. A nil map becomes the literal `"*"`.
//
// Genqlient routes the `Conditions` scalar through this function;
// application code that wants the plain JSON object form should call
// [json.Marshal] on the value directly.
//
// Takes a pointer because genqlient passes `&src` for scalar marshaler
// inputs; nil pointer is treated the same as a nil map.
func MarshalConditions(c *types.PolicyConditions) ([]byte, error) {
	if c == nil || *c == nil {
		return []byte(`"*"`), nil
	}
	inner, err := json.Marshal(*c)
	if err != nil {
		return nil, err
	}
	return json.Marshal(string(inner))
}

// UnmarshalConditions decodes the GraphQL `Conditions` scalar. The
// platform is asymmetric — inputs require the JSON-encoded-string wire
// form, but read responses come back as bare JSON objects. We accept
// both shapes here so callers see one consistent Go value.
func UnmarshalConditions(data []byte, v *types.PolicyConditions) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*v = nil
		return nil
	}

	body := trimmed
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return fmt.Errorf("conditions: %w", err)
		}
		if s == "*" {
			*v = nil
			return nil
		}
		body = []byte(s)
	}

	return json.Unmarshal(body, v)
}
