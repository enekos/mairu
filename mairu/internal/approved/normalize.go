package approved

import (
	"encoding/json"
	"reflect"
	"strings"
)

func normalizeJSON(v any) ([]byte, error) {
	fixed := fixNils(reflect.ValueOf(v))
	raw, err := json.MarshalIndent(fixed, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

// fixNils recursively replaces nil slices with empty slices
// and nil maps with empty maps using reflection, before marshaling.
func fixNils(v reflect.Value) any {
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			return nil
		}
		return fixNils(v.Elem())
	case reflect.Struct:
		out := make(map[string]any)
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			field := t.Field(i)
			if !field.IsExported() {
				continue
			}
			name := field.Tag.Get("json")
			if name == "" || name == "-" {
				if name == "-" {
					continue
				}
				name = field.Name
			}
			// Strip json tag options (e.g., "name,omitempty")
			if idx := strings.Index(name, ","); idx >= 0 {
				name = name[:idx]
			}
			out[name] = fixNils(v.Field(i))
		}
		return out
	case reflect.Slice:
		if v.IsNil() {
			return []any{}
		}
		out := make([]any, v.Len())
		for i := 0; i < v.Len(); i++ {
			out[i] = fixNils(v.Index(i))
		}
		return out
	case reflect.Map:
		if v.IsNil() {
			return map[string]any{}
		}
		out := make(map[string]any, v.Len())
		for _, key := range v.MapKeys() {
			out[key.String()] = fixNils(v.MapIndex(key))
		}
		return out
	default:
		if v.IsValid() {
			return v.Interface()
		}
		return nil
	}
}

// removeFields removes the named keys from a JSON tree at any depth.
func removeFields(v any, fields map[string]bool) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, child := range val {
			if fields[k] {
				continue
			}
			out[k] = removeFields(child, fields)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, child := range val {
			out[i] = removeFields(child, fields)
		}
		return out
	default:
		return v
	}
}
