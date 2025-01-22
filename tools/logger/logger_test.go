package main

import (
	"go/types"
	"testing"
)

func TestGetSimpleOptionalFieldFormat(t *testing.T) {
	newNamedType := func(name string, underlying types.Type) types.Type {
		return types.NewNamed(
			types.NewTypeName(0, nil, name, nil),
			underlying,
			nil,
		)
	}

	tests := []struct {
		name     string
		typ      types.Type
		expected string
	}{
		{
			name:     "string type",
			typ:      types.Typ[types.String],
			expected: "\nif s.{{.name}} != \"\" {\n\taddField(\"{{.key}}\", s.{{.name}})\n}",
		},
		{
			name:     "int type",
			typ:      types.Typ[types.Int],
			expected: "\nif s.{{.name}} != 0 {\n\taddField(\"{{.key}}\", s.{{.name}})\n}",
		},
		{
			name:     "float64 type",
			typ:      types.Typ[types.Float64],
			expected: "\nif s.{{.name}} != 0.0 {\n\taddField(\"{{.key}}\", s.{{.name}})\n}",
		},
		{
			name:     "bool type",
			typ:      types.Typ[types.Bool],
			expected: "\nif s.{{.name}} != false {\n\taddField(\"{{.key}}\", s.{{.name}})\n}",
		},
		{
			name:     "custom string type (type Token string)",
			typ:      newNamedType("Token", types.Typ[types.String]),
			expected: "\nif s.{{.name}} != \"\" {\n\taddField(\"{{.key}}\", s.{{.name}})\n}",
		},
		{
			name:     "custom int type (type Digits int)",
			typ:      newNamedType("Digits", types.Typ[types.Int]),
			expected: "\nif s.{{.name}} != 0 {\n\taddField(\"{{.key}}\", s.{{.name}})\n}",
		},
		{
			name:     "custom float type (type Decimal float64)",
			typ:      newNamedType("Decimal", types.Typ[types.Float64]),
			expected: "\nif s.{{.name}} != 0.0 {\n\taddField(\"{{.key}}\", s.{{.name}})\n}",
		},
		{
			name:     "custom bool type (type Truther bool)",
			typ:      newNamedType("Truther", types.Typ[types.Bool]),
			expected: "\nif s.{{.name}} != false {\n\taddField(\"{{.key}}\", s.{{.name}})\n}",
		},
		{
			name:     "pointer type",
			typ:      types.NewPointer(types.Typ[types.Int]),
			expected: "\nif s.{{.name}} != nil {\n\taddField(\"{{.key}}\", s.{{.name}})\n}",
		},
		{
			name:     "pointer to custom type",
			typ:      types.NewPointer(newNamedType("Nullable", types.Typ[types.String])),
			expected: "\nif s.{{.name}} != nil {\n\taddField(\"{{.key}}\", s.{{.name}})\n}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getSimpleOptionalFieldFormat(tt.typ)
			if got != tt.expected {
				t.Errorf("getSimpleOptionalFieldFormat() =\n%v\nwant:\n%v", got, tt.expected)
			}
		})
	}
}
