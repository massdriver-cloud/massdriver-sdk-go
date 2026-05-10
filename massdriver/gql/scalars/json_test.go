package scalars_test

import (
	"reflect"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/scalars"
)

func TestMarshalJSON(t *testing.T) {
	data := map[string]any{"foo": "bar"}
	got, _ := scalars.MarshalJSON(data)

	want := `"{\"foo\":\"bar\"}"`

	if string(got) != want {
		t.Errorf("got %s, wanted %s", got, want)
	}
}

func TestMarshalJSON_EmptyMapReturnsNil(t *testing.T) {
	var data map[string]any
	got, err := scalars.MarshalJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("got %s, wanted nil", got)
	}
}

func TestMarshalJSON_ExplicitEmptyMapEncodes(t *testing.T) {
	data := map[string]any{}
	got, err := scalars.MarshalJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != `"{}"` {
		t.Errorf("got %s, wanted \"{}\"", got)
	}
}

func TestUnmarshalJSON(t *testing.T) {
	want := map[string]any{"foo": "bar"}

	data := []byte(`{"foo": "bar"}`)
	got := map[string]any{}

	if err := scalars.UnmarshalJSON(data, &got); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, wanted %v", got, want)
	}
}
