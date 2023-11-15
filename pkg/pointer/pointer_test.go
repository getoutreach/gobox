package pointer

import (
	"testing"
)

type customStruct struct {
	name string
}

func TestPtrStruct(t *testing.T) {
	oPtr := ToPtr(customStruct{name: "value"})
	if oPtr == nil {
		t.Error("expected a non-nil pointer")
	}
	s := *oPtr
	if s.name != "value" {
		t.Errorf("expected %s, but received %s", "value", s.name)
	}
	oValue := ToValue(oPtr)
	if oValue.name != "value" {
		t.Errorf("expected %s, but received %s", "value", oValue.name)
	}
}

func TestSlicePtrStruct(t *testing.T) {
	value := []customStruct{{name: "one"}, {name: "two"}}
	oSlice := ToSlicePtr(value)
	if len(oSlice) != len(value) {
		t.Errorf("expected slice of size %d, but got %d", len(value), len(oSlice))
	}
	for i, v := range value {
		if oSlice[i] == nil {
			t.Error("unexpected nil value in slice")
		}
		s := *oSlice[i]
		if s.name != v.name {
			t.Errorf("expected value of size %s, but got %s", v.name, s.name)
		}
	}
	oSliceValue := ToSliceValue(oSlice)
	if len(oSliceValue) != len(value) {
		t.Errorf("expected slice of size %d, but got %d", len(value), len(oSliceValue))
	}
	for i, v := range value {
		if oSliceValue[i].name != v.name {
			t.Errorf("expected value of size %s, but got %s", v.name, oSliceValue[i].name)
		}
	}
}

func TestMapPtrStruct(t *testing.T) {
	value := map[string]customStruct{
		"1":  {name: "One"},
		"2":  {name: "Two"},
		"10": {name: "Three??"},
	}
	oMap := ToMapPtr(value)
	if len(oMap) != len(value) {
		t.Errorf("expected slice of size %d, but got %d", len(value), len(oMap))
	}
	for i, v := range value {
		if oMap[i] == nil {
			t.Error("unexpected nil value in slice")
		}
		s := *oMap[i]
		if s.name != v.name {
			t.Errorf("expected value of size %s, but got %s", v.name, s.name)
		}
	}
	oMapValue := ToMapValue(oMap)
	if len(oMapValue) != len(value) {
		t.Errorf("expected slice of size %d, but got %d", len(value), len(oMapValue))
	}
	for i, v := range value {
		if oMapValue[i].name != v.name {
			t.Errorf("expected value of size %s, but got %s", v.name, oMapValue[i].name)
		}
	}
}

func TestPtrString(t *testing.T) {
	oPtr := ToPtr("value")
	if oPtr == nil {
		t.Error("expected a non-nil pointer")
	}
	if *oPtr != "value" {
		t.Errorf("expected %s, but received %s", "value", *oPtr)
	}
	oValue := ToValue(oPtr)
	if oValue != "value" {
		t.Errorf("expected %s, but received %s", "value", oValue)
	}
}

func TestSlicePtrString(t *testing.T) {
	value := []string{"a", "b", "c"}
	oSlice := ToSlicePtr(value)
	if len(oSlice) != len(value) {
		t.Errorf("expected slice of size %d, but got %d", len(value), len(oSlice))
	}
	for i, v := range value {
		if oSlice[i] == nil {
			t.Error("unexpected nil value in slice")
		}
		if *oSlice[i] != v {
			t.Errorf("expected value of size %s, but got %s", v, *oSlice[i])
		}
	}
	oSliceValue := ToSliceValue(oSlice)
	if len(oSliceValue) != len(value) {
		t.Errorf("expected slice of size %d, but got %d", len(value), len(oSliceValue))
	}
	for i, v := range value {
		if oSliceValue[i] != v {
			t.Errorf("expected value of size %s, but got %s", v, oSliceValue[i])
		}
	}
}

func TestMapPtrString(t *testing.T) {
	value := map[int]string{
		1:  "a",
		2:  "b",
		10: "c",
	}
	oMap := ToMapPtr(value)
	if len(oMap) != len(value) {
		t.Errorf("expected slice of size %d, but got %d", len(value), len(oMap))
	}
	for i, v := range value {
		if oMap[i] == nil {
			t.Error("unexpected nil value in slice")
		}
		if *oMap[i] != v {
			t.Errorf("expected value of size %s, but got %s", v, *oMap[i])
		}
	}
	oMapValue := ToMapValue(oMap)
	if len(oMapValue) != len(value) {
		t.Errorf("expected slice of size %d, but got %d", len(value), len(oMapValue))
	}
	for i, v := range value {
		if oMapValue[i] != v {
			t.Errorf("expected value of size %s, but got %s", v, oMapValue[i])
		}
	}
}

func TestPtrInterface(t *testing.T) {
	var value interface{} = "some interface"
	oPtr := ToPtr(value)
	if oPtr == nil {
		t.Error("expected a non-nil pointer")
	}
	if *oPtr != value {
		t.Errorf("expected %s, but received %s", "value", *oPtr)
	}
	oValue := ToValue(oPtr)
	if oValue != value {
		t.Errorf("expected %s, but received %s", "value", oValue)
	}
}

func TestSlicePtrInterface(t *testing.T) {
	value := []interface{}{"a", 2, true}
	oSlice := ToSlicePtr(value)
	if len(oSlice) != len(value) {
		t.Errorf("expected slice of size %d, but got %d", len(value), len(oSlice))
	}
	for i, v := range value {
		if oSlice[i] == nil {
			t.Error("unexpected nil value in slice")
		}
		if *oSlice[i] != v {
			t.Errorf("expected value of size %s, but got %s", v, *oSlice[i])
		}
	}
	oSliceValue := ToSliceValue(oSlice)
	if len(oSliceValue) != len(value) {
		t.Errorf("expected slice of size %d, but got %d", len(value), len(oSliceValue))
	}
	for i, v := range value {
		if oSliceValue[i] != v {
			t.Errorf("expected value of size %s, but got %s", v, oSliceValue[i])
		}
	}
}

func TestMapPtrInterface(t *testing.T) {
	value := map[int]interface{}{
		1:  "a",
		2:  50,
		10: true,
	}
	oMap := ToMapPtr(value)
	if len(oMap) != len(value) {
		t.Errorf("expected slice of size %d, but got %d", len(value), len(oMap))
	}
	for i, v := range value {
		if oMap[i] == nil {
			t.Error("unexpected nil value in slice")
		}
		if *oMap[i] != v {
			t.Errorf("expected value of size %s, but got %s", v, *oMap[i])
		}
	}
	oMapValue := ToMapValue(oMap)
	if len(oMapValue) != len(value) {
		t.Errorf("expected slice of size %d, but got %d", len(value), len(oMapValue))
	}
	for i, v := range value {
		if oMapValue[i] != v {
			t.Errorf("expected value of size %s, but got %s", v, oMapValue[i])
		}
	}
}
