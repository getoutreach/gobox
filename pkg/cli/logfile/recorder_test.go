// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file implements the serialization logic for
// logs.

package logfile

import (
	"bytes"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	jsoniter "github.com/json-iterator/go"
)

func Test_recorder(t *testing.T) {
	startedAt := time.Now()

	tests := []struct {
		name    string
		data    []string
		want    string
		wantErr bool
	}{
		{
			name: "should write frames",
			data: []string{"abc", "def"},
			want: `{"d":1,"b":"YWJj"}` + "\n" + `{"d":1,"b":"ZGVm"}` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			r := &recorder{
				enc:       jsoniter.NewEncoder(buf),
				startedAt: startedAt,
				lastWrite: startedAt,
				fixedDiff: 1,
			}

			for _, s := range tt.data {
				r.Write([]byte(s))
			}

			contents := buf.String()
			if diff := cmp.Diff(contents, tt.want); diff != "" {
				t.Errorf("recorder.Write() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
