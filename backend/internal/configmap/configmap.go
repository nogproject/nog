package configmap

import (
	"encoding/json"
	"errors"
	"fmt"

	pb "github.com/nogproject/nog/backend/internal/nogfsopb"
)

func ParsePb(c *pb.ConfigMap) (map[string]interface{}, error) {
	m := make(map[string]interface{})

	if c == nil {
		return m, nil
	}

	for _, kv := range c.Fields {
		switch v := kv.Val.(type) {
		case *pb.ConfigField_Number:
			var val float64 = v.Number
			m[kv.Key] = val

		case *pb.ConfigField_Text:
			var val string = v.Text
			m[kv.Key] = val

		case *pb.ConfigField_TextList:
			var val []string = v.TextList.Vals
			m[kv.Key] = val

		// Add types here as needed.

		default:
			err := errors.New("unsupported ConfigMap value type")
			return nil, err
		}
	}

	return m, nil
}

func Merge(
	a map[string]interface{}, b map[string]interface{},
) (map[string]interface{}, error) {
	if len(b) == 0 {
		return a, nil
	}

	// Start from copy of a.
	m := make(map[string]interface{})
	for k, v := range a {
		m[k] = v
	}

	for k, v := range b {
		// If not in a, take from b.
		aVal, ok := a[k]
		if !ok {
			m[k] = v
			continue
		}

		// Otherwise, do type-dependent merge.
		switch x := v.(type) {
		case []string:
			aX, ok := aVal.([]string)
			if !ok {
				err := fmt.Errorf(
					"field `%s` type mismatch", k,
				)
				return nil, err
			}
			m[k] = append(aX, x...)

		// Add types here as needed.

		default:
			err := fmt.Errorf(
				"field `%s` unsupported type", k,
			)
			return nil, err
		}
	}

	return m, nil
}

func NewPbFromJsonString(s string) (*pb.ConfigMap, error) {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, err
	}

	fields := make([]*pb.ConfigField, 0)
	for k, v := range m {
		if x, ok := asStringList(v); ok {
			fields = append(fields, &pb.ConfigField{
				Key: k,
				Val: &pb.ConfigField_TextList{
					&pb.StringList{Vals: x},
				},
			})
			continue
		}

		switch x := v.(type) {
		case float64:
			fields = append(fields, &pb.ConfigField{
				Key: k,
				Val: &pb.ConfigField_Number{x},
			})

		case string:
			fields = append(fields, &pb.ConfigField{
				Key: k,
				Val: &pb.ConfigField_Text{x},
			})

		default:
			err := fmt.Errorf("field `%s`: unsupported type", k)
			return nil, err
		}
	}

	return &pb.ConfigMap{Fields: fields}, nil
}

func asStringList(in interface{}) ([]string, bool) {
	lst, ok := in.([]interface{})
	if !ok {
		return nil, false
	}

	out := make([]string, 0, len(lst))
	for _, v := range lst {
		s, ok := v.(string)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}

	return out, true
}
