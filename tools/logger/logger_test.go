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

func TestProcessStructWithTimeField(t *testing.T) {
	var buf bytes.Buffer
	timeType := types.NewNamed(
		types.NewTypeName(0, types.NewPackage("time", "time"), "Time", nil),
		types.NewStruct(nil, nil),
		nil,
	)

	s := types.NewStruct([]*types.Var{
		types.NewVar(0, nil, "CreatedAt", timeType),
	}, []string{
		`log:"created_at"`,
	})

	processStruct(&buf, s, "TimeStruct")

	expected := `
func (s *TimeStruct) MarshalLog(addField func(key string, value interface{})) {
	if s == nil {
		return
	}

addField("created_at", s.CreatedAt.UTC().Format(time.RFC3339Nano))
}
`
	assert.Equal(t, expected, buf.String())
}

func TestProcessStructWithNestedMarshaler(t *testing.T) {
	var buf bytes.Buffer
	nestedType := types.NewNamed(
		types.NewTypeName(0, nil, "NestedStruct", nil),
		types.NewStruct(nil, nil),
		nil,
	)
	s := types.NewStruct([]*types.Var{
		types.NewVar(0, nil, "Child", types.NewPointer(nestedType)),
	}, []string{
		`log:"."`,
	})

	processStruct(&buf, s, "ParentStruct")

	expected := `
func (s *ParentStruct) MarshalLog(addField func(key string, value interface{})) {
	if s == nil {
		return
	}

if s.Child != nil {
	s.Child.MarshalLog(addField)
}
}
`
	assert.Equal(t, expected, buf.String())
}

func TestProcessStructWithOmitemptyField(t *testing.T) {
	var buf bytes.Buffer
	s := types.NewStruct([]*types.Var{
		types.NewVar(0, nil, "Status", types.Typ[types.String]),
	}, []string{
		`log:"status,omitempty"`,
	})

	processStruct(&buf, s, "OptionalStruct")

	expected := `
func (s *OptionalStruct) MarshalLog(addField func(key string, value interface{})) {
	if s == nil {
		return
	}

if s.Status != "" {
	addField("status", s.Status)
}
}
`
	assert.Equal(t, expected, buf.String())
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
