package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"
)

// DecodeStrict accepts exactly one UTF-8 JSON value. Struct fields are required
// unless tagged optional:"true"; only pointer types permit explicit null.
// Exact JSON field spelling, recursive duplicate detection and bounded nesting
// avoid encoding/json's case-insensitive and last-key-wins behavior.
func DecodeStrict(b []byte, dst any) error {
	if !utf8.Valid(b) {
		return errors.New("invalid UTF-8")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	v, err := value(d, 0)
	if err != nil {
		return err
	}
	if _, err = d.Token(); err != io.EOF {
		return errors.New("trailing JSON value")
	}
	t := reflect.TypeOf(dst)
	if t.Kind() != reflect.Pointer {
		return errors.New("destination must be pointer")
	}
	if err = shape(v, t.Elem(), "$"); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
func value(d *json.Decoder, depth int) (any, error) {
	if depth > 128 {
		return nil, errors.New("JSON nesting limit")
	}
	t, err := d.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := t.(json.Delim); ok {
		switch delim {
		case '{':
			m := map[string]any{}
			for d.More() {
				k, e := d.Token()
				if e != nil {
					return nil, e
				}
				s, ok := k.(string)
				if !ok {
					return nil, errors.New("invalid key")
				}
				if _, ok = m[s]; ok {
					return nil, fmt.Errorf("duplicate key %q", s)
				}
				v, e := value(d, depth+1)
				if e != nil {
					return nil, e
				}
				m[s] = v
			}
			_, err = d.Token()
			return m, err
		case '[':
			a := []any{}
			for d.More() {
				v, e := value(d, depth+1)
				if e != nil {
					return nil, e
				}
				a = append(a, v)
			}
			_, err = d.Token()
			return a, err
		}
		return nil, errors.New("unexpected delimiter")
	}
	return t, nil
}

var rawType = reflect.TypeFor[json.RawMessage]()
var timeType = reflect.TypeFor[time.Time]()

func shape(v any, t reflect.Type, p string) error {
	bad := func() error { return fmt.Errorf("invalid or null value at %s", p) }
	if t.Kind() == reflect.Pointer {
		if v == nil {
			return nil
		}
		return shape(v, t.Elem(), p)
	}
	if t == rawType {
		if v == nil {
			return bad()
		}
		return nil
	}
	if v == nil {
		return bad()
	}
	if t == timeType {
		s, ok := v.(string)
		if !ok {
			return bad()
		}
		_, err := time.Parse(time.RFC3339Nano, s)
		return err
	}
	switch t.Kind() {
	case reflect.Struct:
		m, ok := v.(map[string]any)
		if !ok {
			return bad()
		}
		known := map[string]bool{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			k := strings.Split(f.Tag.Get("json"), ",")[0]
			if k == "-" {
				continue
			}
			if k == "" {
				k = f.Name
			}
			known[k] = true
			x, present := m[k]
			if !present {
				if f.Tag.Get("optional") == "true" {
					continue
				}
				return fmt.Errorf("missing %s.%s", p, k)
			}
			if err := shape(x, f.Type, p+"."+k); err != nil {
				return err
			}
		}
		for k := range m {
			if !known[k] {
				return fmt.Errorf("unknown %s.%s", p, k)
			}
		}
	case reflect.Slice:
		a, ok := v.([]any)
		if !ok {
			return bad()
		}
		for i, x := range a {
			if err := shape(x, t.Elem(), fmt.Sprintf("%s[%d]", p, i)); err != nil {
				return err
			}
		}
	case reflect.String:
		if _, ok := v.(string); !ok {
			return bad()
		}
	case reflect.Bool:
		if _, ok := v.(bool); !ok {
			return bad()
		}
	case reflect.Int, reflect.Int64:
		n, ok := v.(json.Number)
		if !ok {
			return bad()
		}
		if _, err := n.Int64(); err != nil {
			return bad()
		}
	default:
		return fmt.Errorf("unsupported JSON type %s", t)
	}
	return nil
}
