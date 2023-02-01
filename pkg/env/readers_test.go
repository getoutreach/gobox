// Copyright 2022 Outreach Corporation. All Rights Reserved.

//go:build or_dev || or_test || or_e2e
// +build or_dev or_test or_e2e

// Description: Provides configuration readers for various environments

package env

import (
	"reflect"
	"testing"
)

const example string = `
BentoNamespace: bento1a
HTTPPort: 81
GRPCPort: 5001
`

func TestFakeTestConfigHandler(t *testing.T) {
	type args struct {
		fName string
		ptr   interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    func()
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FakeTestConfigHandler(tt.args.fName, tt.args.ptr)
			if (err != nil) != tt.wantErr {
				t.Errorf("FakeTestConfigHandler() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FakeTestConfigHandler() = %v, want %v", got, tt.want)
			}
		})
	}
}
