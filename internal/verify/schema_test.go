package verify_test

import (
	"encoding/json"
	"github.com/akkaraponph/itervyn/internal/config"
	"github.com/akkaraponph/itervyn/internal/verify"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

// This test guards published field/required/nullability drift. Behavioral
// constraints have independent config, policy, process and integrity tests.
func TestPublishedContractFields(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value any
	}{{"config", config.Config{}}, {"result", verify.Result{}}, {"diagnostic", verify.Diagnostic{}}} {
		b, e := os.ReadFile("../../schemas/" + tc.name + "-v1alpha1.schema.json")
		if e != nil {
			t.Fatal(e)
		}
		var schema map[string]any
		if e = json.Unmarshal(b, &schema); e != nil {
			t.Fatal(e)
		}
		compareShape(t, reflect.TypeOf(tc.value), schema, schema, tc.name)
	}
}
func compareShape(t *testing.T, typ reflect.Type, node, root map[string]any, path string) {
	t.Helper()
	if name, ok := node["$ref"].(string); ok {
		node = root["$defs"].(map[string]any)[strings.TrimPrefix(name, "#/$defs/")].(map[string]any)
	}
	if typ.Kind() == reflect.Pointer {
		if node["type"] == "null" {
			return
		}
		choices, ok := node["anyOf"].([]any)
		if !ok {
			t.Fatal(path, "nullable schema missing")
		}
		for _, x := range choices {
			m := x.(map[string]any)
			if m["type"] != "null" {
				compareShape(t, typ.Elem(), m, root, path)
				return
			}
		}
		return
	}
	if typ == reflect.TypeFor[time.Time]() {
		return
	}
	if typ.Kind() == reflect.Struct {
		props := node["properties"].(map[string]any)
		required := map[string]bool{}
		for _, k := range node["required"].([]any) {
			required[k.(string)] = true
		}
		count := 0
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			key := strings.Split(f.Tag.Get("json"), ",")[0]
			if key == "-" {
				continue
			}
			count++
			child, ok := props[key].(map[string]any)
			if !ok {
				t.Fatal(path, "missing schema field", key)
			}
			if required[key] != (f.Tag.Get("optional") != "true") {
				t.Fatal(path, "required mismatch", key)
			}
			compareShape(t, f.Type, child, root, path+"."+key)
		}
		if count != len(props) {
			t.Fatal(path, "extra schema field")
		}
	}
	if typ.Kind() == reflect.Slice {
		compareShape(t, typ.Elem(), node["items"].(map[string]any), root, path+"[]")
	}
}
