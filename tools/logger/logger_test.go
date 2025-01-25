package main

import (
	"bytes"
	"go/types"
	"testing"

	"gotest.tools/v3/assert"
)

func TestGetOptionalFieldFormat(t *testing.T) {
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
			got := getOptionalFieldFormat(tt.typ)
			if got != tt.expected {
				t.Errorf("getOptionalFieldFormat() =\n%v\nwant:\n%v", got, tt.expected)
			}
		})
	}
}

func TestProcessStruct(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() (*types.Struct, string)
		expected string
	}{
		{
			name: "basic fields",
			setup: func() (*types.Struct, string) {
				fields := []*types.Var{
					types.NewField(0, nil, "Name", types.Typ[types.String], false),
					types.NewField(0, nil, "Age", types.Typ[types.Int], false),
				}
				tags := []string{
					`log:"name"`,
					`log:"age"`,
				}
				return types.NewStruct(fields, tags), "BasicStruct"
			},
			expected: `
func (s *BasicStruct) MarshalLog(addField func(key string, value interface{})) {
	if s == nil {
		return
	}

addField("name", s.Name)
addField("age", s.Age)
}
`,
		},
		{
			name: "time field",
			setup: func() (*types.Struct, string) {
				pkg := types.NewPackage("time", "time")
				timeType := types.NewNamed(types.NewTypeName(0, pkg, "Time", nil), &types.Struct{}, nil)

				fields := []*types.Var{
					types.NewField(0, nil, "CreatedAt", timeType, false),
				}
				tags := []string{`log:"created_at"`}
				return types.NewStruct(fields, tags), "TimeStruct"
			},
			expected: `
func (s *TimeStruct) MarshalLog(addField func(key string, value interface{})) {
	if s == nil {
		return
	}

addField("created_at", s.CreatedAt.UTC().Format(time.RFC3339Nano))
}
`,
		},
		{
			name: "omitempty fields",
			setup: func() (*types.Struct, string) {
				fields := []*types.Var{
					types.NewField(0, nil, "Name", types.Typ[types.String], false),
					types.NewField(0, nil, "AgeP", types.NewPointer(types.Typ[types.Int]), false),
				}
				tags := []string{
					`log:"name,omitempty"`,
					`log:"ageP,omitempty"`,
				}
				return types.NewStruct(fields, tags), "OmitStruct"
			},
			expected: `
func (s *OmitStruct) MarshalLog(addField func(key string, value interface{})) {
	if s == nil {
		return
	}

if s.Name != "" {
	addField("name", s.Name)
}
if s.AgeP != nil {
	addField("ageP", s.AgeP)
}
}
`,
		},
		{
			name: "nested marshaler",
			setup: func() (*types.Struct, string) {
				nestedType := types.NewPointer(types.NewNamed(
					types.NewTypeName(0, nil, "NestedStruct", nil),
					&types.Struct{},
					nil,
				))
				fields := []*types.Var{
					types.NewField(0, nil, "Nested", nestedType, false),
				}
				tags := []string{`log:"."`}
				return types.NewStruct(fields, tags), "ParentStruct"
			},
			expected: `
func (s *ParentStruct) MarshalLog(addField func(key string, value interface{})) {
	if s == nil {
		return
	}

if s.Nested != nil {
	s.Nested.MarshalLog(addField)
}
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, name := tt.setup()
			buf := &bytes.Buffer{}
			processStruct(buf, s, name)
			assert.Equal(t, tt.expected, buf.String())
		})
	}
}

func TestContains(t *testing.T) {
	var uninitializedStrings []string
	var uninitializedIntegers []int
	var uninitializedBooleans []bool
	tests := []struct {
		name           string
		slice          any
		item           any
		underlyingType string
		expected       bool
	}{
		{
			name:           "string slice with matching item",
			slice:          []string{"a", "b", "c"},
			item:           "b",
			underlyingType: "string",
			expected:       true,
		},
		{
			name:           "string slice without matching item",
			slice:          []string{"a", "b", "c"},
			item:           "d",
			underlyingType: "string",
			expected:       false,
		},
		{
			name:           "empty string slice",
			slice:          []string{},
			item:           "a",
			underlyingType: "string",
			expected:       false,
		},
		{
			name:           "uninitialized string slice",
			slice:          uninitializedStrings,
			item:           "a",
			underlyingType: "string",
			expected:       false,
		},
		{
			name:           "int slice with matching item",
			slice:          []int{1, 2, 3},
			item:           2,
			underlyingType: "int",
			expected:       true,
		},
		{
			name:           "uninitialized int slice",
			slice:          uninitializedIntegers,
			item:           4,
			underlyingType: "int",
			expected:       false,
		},
		{
			name:           "bool slice with matching item",
			slice:          []bool{true, false},
			item:           true,
			underlyingType: "bool",
			expected:       true,
		},
		{
			name:           "bool slice without matching item",
			slice:          []bool{false},
			item:           true,
			underlyingType: "bool",
			expected:       false,
		},
		{
			name:           "uninitialized bool slice",
			slice:          uninitializedBooleans,
			item:           false,
			underlyingType: "bool",
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.underlyingType {
			case "string":
				if got := contains(tt.slice.([]string), tt.item.(string)); got != tt.expected {
					t.Errorf("contains() = %v, want %v", got, tt.expected)
				}
			case "int":
				if got := contains(tt.slice.([]int), tt.item.(int)); got != tt.expected {
					t.Errorf("contains() = %v, want %v", got, tt.expected)
				}
			case "bool":
				if got := contains(tt.slice.([]bool), tt.item.(bool)); got != tt.expected {
					t.Errorf("contains() = %v, want %v", got, tt.expected)
				}
			}
		})
	}
}
