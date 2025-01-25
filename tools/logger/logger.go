// Copyright 2022 Outreach Corporation. All Rights Reserved.

// main
//
// The logger cmd can be used to generate log marshalers for structs.
//
// # See generating.md for generating log marshalers
//
// Usage: logger [flag] [files]
//
// By default the output is written to marshalers.go
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/types"
	"io"
	"log"
	"os"
	"reflect"
	"sort"
	"strings"
	"text/template"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/imports"
)

// nolint:gochecknoglobals // Why: flag used in multiple places
var (
	outputFile = flag.String("output", "marshalers.go", "location of generated marshalers")
	header     = `// Code generated by "logger %s"; DO NOT EDIT.

package %s

`
	functionHeaderFormat = `
func (s *{{ .name }}) MarshalLog(addField func(key string, value interface{})) {
	if s == nil {
		return
	}
`
	timeFieldFormat = `
addField("{{.key}}", s.{{.name}}.UTC().Format(time.RFC3339Nano))`
	simpleFieldFormat = `
addField("{{.key}}", s.{{.name}})`
	optionalFieldFormat = `
if s.{{.name}} != %s {
	addField("{{.key}}", s.{{.name}})
}`
	nestedMarshalerFormat = `
s.{{.name}}.MarshalLog(addField)`
	nestedNilableMarshalerFormat = `
if s.{{.name}} != nil {
	s.{{.name}}.MarshalLog(addField)
}`
)

const (
	annotationOmitEmpty = "omitempty"
)

func main() {
	flag.Usage = usage

	flag.Parse()
	args := flag.Args()

	// use current directory if no file names are provided
	if len(args) == 0 {
		args = []string{"."}
	}

	mode := packages.NeedName | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps
	cfg := &packages.Config{Mode: mode, Tests: false}
	pkgs, err := packages.Load(cfg, args...)
	if err != nil || len(pkgs) != 1 {
		log.Fatalf("generation failed %v", err)
	}
	scanPackage(pkgs[0])
}

func scanPackage(pkg *packages.Package) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, header, strings.Join(os.Args[1:], " "), pkg.Name)

	names, structs := filterStructs(pkg)
	for kk := range names {
		processStruct(&buf, structs[kk], names[kk])
	}

	// run the equivalent of goimports -w
	result, err := imports.Process(*outputFile, buf.Bytes(), nil)
	if err != nil {
		log.Fatal(err)
	}

	// write to stdout
	if err := os.WriteFile(*outputFile, result, 0o600); err != nil {
		log.Fatal(err)
	}
}

func filterStructs(pkg *packages.Package) ([]string, []*types.Struct) {
	names := []string{}
	structs := map[string]*types.Struct{}

	// look for structs which have the `log:".."` tag specified
	for _, def := range pkg.TypesInfo.Defs {
		typeName, ok := def.(*types.TypeName)
		if !ok {
			continue
		}

		s, ok := typeName.Type().Underlying().(*types.Struct)
		if !ok {
			continue
		}

		for kk := 0; kk < s.NumFields(); kk++ {
			if _, ok := reflect.StructTag(s.Tag(kk)).Lookup("log"); ok {
				structs[typeName.Name()] = s
				names = append(names, typeName.Name())
				break
			}
		}
	}

	sort.Strings(names)
	result := make([]*types.Struct, len(names))
	for kk, name := range names {
		result[kk] = structs[name]
	}
	return names, result
}

func processStruct(w io.Writer, s *types.Struct, name string) {
	write(w, functionHeaderFormat, map[string]string{"name": name})
	for kk := 0; kk < s.NumFields(); kk++ {
		field, ok := reflect.StructTag(s.Tag(kk)).Lookup("log")
		if !ok {
			continue
		}

		var annotations []string
		fieldParts := strings.SplitN(field, ",", 2)
		field = fieldParts[0]
		if len(fieldParts) > 1 {
			annotations = fieldParts[1:]
		}
		args := map[string]string{"key": field, "name": s.Field(kk).Name()}
		switch {
		case s.Field(kk).Type().String() == "time.Time":
			write(w, timeFieldFormat, args)
		case field == "." && isNilable(s.Field(kk).Type()):
			write(w, nestedNilableMarshalerFormat, args)
		case field == ".":
			write(w, nestedMarshalerFormat, args)
		case contains(annotations, annotationOmitEmpty):
			write(w, getOptionalFieldFormat(s.Field(kk).Type()), args)
		default:
			write(w, simpleFieldFormat, args)
		}
	}
	fmt.Fprintf(w, "\n}\n")
}

func isNilable(t types.Type) bool {
	_, isInterface := t.Underlying().(*types.Interface)
	_, isPointer := t.Underlying().(*types.Pointer)
	return isInterface || isPointer
}

func write(w io.Writer, s string, args map[string]string) {
	err := template.Must(template.New("tpl").Parse(s)).Execute(w, args)
	if err != nil {
		panic(err)
	}
}

// usage is printed out automatically by flags when needed
func usage() {
	fmt.Fprintf(os.Stderr, "Usage of logger:\n")
	fmt.Fprintf(os.Stderr, "\tlogger [go files or directory]\n")
	fmt.Fprintf(os.Stderr, "Flags:\n")
	flag.PrintDefaults()
}

func getOptionalFieldFormat(p types.Type) string {
	var defaultValue string
	switch p.Underlying().String() {
	case "string":
		defaultValue = `""`
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64":
		defaultValue = "0"
	case "float32", "float64":
		defaultValue = "0.0"
	case "bool":
		defaultValue = "false"
	default:
		defaultValue = "nil"
	}

	return fmt.Sprintf(optionalFieldFormat, defaultValue)
}

func contains[T comparable](slice []T, item T) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
