// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains the persistent storage format
// for logfiles.

package logfile

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func Test_read(t *testing.T) {
	type args struct {
		r io.Reader
	}
	tests := []struct {
		name    string
		args    args
		want    []Entry
		wantErr bool
	}{
		{
			name: "should read basic frames",
			args: args{
				// abc, def (w/ diff 1)
				r: bytes.NewBufferString(`{"t":1,"d":1,"b":"YWJj"}` + "\n" + `{"t":1,"d":1,"b":"ZGVm"}` + "\n"),
			},
			want: []Entry{
				NewFrameEntry(1, []byte("abc")),
				NewFrameEntry(1, []byte("def")),
			},
		},
		{
			name: "should read metadata",
			args: args{
				r: bytes.NewBufferString(`{"t":0,"started_at":"2022-10-13T00:00:00Z","command":"player","args":["arg1","arg2"]}` + "\n"),
			},
			want: []Entry{
				NewMetadataEntry(time.Date(2022, 10, 13, 0, 0, 0, 0, time.UTC), "player", []string{"arg1", "arg2"}),
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
			if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(Entry{})); diff != "" {
				t.Errorf("read mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
