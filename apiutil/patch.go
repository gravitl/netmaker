package apiutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
)

type Patch[T any] struct {
	fields map[string]json.RawMessage
}

type options struct {
	includeFields []string
	excludeFields []string
}

type ApplyOption func(*options)

func IncludeFields(fields ...string) ApplyOption {
	return func(o *options) {
		o.includeFields = fields
	}
}

func ExcludeFields(fields ...string) ApplyOption {
	return func(o *options) {
		o.excludeFields = fields
	}
}

type FieldDiff struct {
	Old any
	New any
}

func NewPatch[T any](r io.Reader) (*Patch[T], error) {
	p := &Patch[T]{fields: make(map[string]json.RawMessage)}
	if err := json.NewDecoder(r).Decode(&p.fields); err != nil {
		return nil, err
	}
	return p, nil
}

func NewPatchFromBytes[T any](body []byte) (*Patch[T], error) {
	return NewPatch[T](bytes.NewReader(body))
}

type ApplyResult[T any] struct {
	Original      *T
	Patched       *T
	UpdatedFields []string
}

func (p *Patch[T]) Apply(target *T, opts ...ApplyOption) (ApplyResult[T], error) {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	effective := buildEffectiveFields(p.fields, o.includeFields, o.excludeFields)

	cloned, err := deepClone(target)
	if err != nil {
		return ApplyResult[T]{}, fmt.Errorf("clone failed: %w", err)
	}

	v := reflect.ValueOf(cloned).Elem()
	t := v.Type()
	origV := reflect.ValueOf(target).Elem()

	var updatedFields []string

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonKey := jsonFieldName(field)
		if jsonKey == "" || jsonKey == "-" {
			continue
		}

		if _, allowed := effective[jsonKey]; !allowed {
			if _, inPatch := p.fields[jsonKey]; inPatch {
				return ApplyResult[T]{}, fmt.Errorf("field %q is not patchable in this context", jsonKey)
			}
			continue
		}

		raw, exists := p.fields[jsonKey]
		if !exists {
			continue
		}

		fieldVal := v.Field(i)
		origVal := origV.Field(i)

		if string(raw) == "null" {
			fieldVal.Set(reflect.Zero(field.Type))
		} else {
			ptr := reflect.New(field.Type)
			if err := json.Unmarshal(raw, ptr.Interface()); err != nil {
				return ApplyResult[T]{}, fmt.Errorf("field %s: %w", jsonKey, err)
			}
			fieldVal.Set(ptr.Elem())
		}

		if !reflect.DeepEqual(origVal.Interface(), fieldVal.Interface()) {
			updatedFields = append(updatedFields, jsonKey)
		}
	}

	return ApplyResult[T]{
		Original:      target,
		Patched:       cloned,
		UpdatedFields: updatedFields,
	}, nil
}

func (r ApplyResult[T]) Diff() map[string]FieldDiff {
	result := map[string]FieldDiff{}

	ov := reflect.ValueOf(r.Original).Elem()
	nv := reflect.ValueOf(r.Patched).Elem()
	rt := ov.Type()

	fieldIndex := map[string]int{}
	for i := 0; i < rt.NumField(); i++ {
		tag := jsonFieldName(rt.Field(i))
		if tag != "" && tag != "-" {
			fieldIndex[tag] = i
		}
	}

	for _, field := range r.UpdatedFields {
		idx, ok := fieldIndex[field]
		if !ok {
			continue
		}
		result[field] = FieldDiff{
			Old: ov.Field(idx).Interface(),
			New: nv.Field(idx).Interface(),
		}
	}

	return result
}

func deepClone[T any](v *T) (*T, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var clone T
	if err := json.Unmarshal(b, &clone); err != nil {
		return nil, err
	}
	return &clone, nil
}

func buildEffectiveFields(
	raw map[string]json.RawMessage,
	include []string,
	exclude []string,
) map[string]struct{} {
	effective := map[string]struct{}{}

	if len(include) > 0 {
		for _, f := range include {
			effective[f] = struct{}{}
		}
	} else {
		for k := range raw {
			effective[k] = struct{}{}
		}
	}

	for _, f := range exclude {
		delete(effective, f)
	}

	return effective
}

func jsonFieldName(f reflect.StructField) string {
	tag := f.Tag.Get("json")
	if tag == "" {
		return strings.ToLower(f.Name)
	}
	return strings.Split(tag, ",")[0]
}
