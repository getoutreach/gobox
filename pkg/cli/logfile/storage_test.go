// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains the persistent storage format
// for logfiles.

package logfile

import (
	"bytes"
	"io"
	"reflect"
	"testing"
)

func Test_read(t *testing.T) {
	type args struct {
		r io.Reader
	}
	tests := []struct {
		name    string
		args    args
		want    []FrameOrMetadata
		wantErr bool
	}{
		{
			name: "should read basic frames",
			args: args{
				// abc, def (w/ diff 1)
				r: bytes.NewBufferString(`{"d":1,"b":"YWJj"}` + "\n" + `{"d":1,"b":"ZGVm"}` + "\n"),
			},
			want: []FrameOrMetadata{
				{
					Frame: &Frame{
						Delay: 1,
						Bytes: []byte("abc"),
					},
				},
				{
					Frame: &Frame{
						Delay: 1,
						Bytes: []byte("def"),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := read(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("read() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("read() = %v, want %v", got, tt.want)
			}
		})
	}
}
