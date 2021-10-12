// Package sshconfig implements a small ssh config parser
// based on the output of `ssh -G`.
package sshconfig

import (
	"context"
	"os"
	"testing"
)

func TestGet(t *testing.T) {
	type args struct {
		ctx   context.Context
		host  string
		field string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "basic operation",
			args: args{
				ctx:   context.TODO(),
				host:  "github.com",
				field: "identityfile",
			},
			want:    "hello-world",
			wantErr: false,
		},
		// This works but include can only check ~/.ssh/ without an abs
		// path. Not writing a generator right now, but in the future it'd be
		// nice to test that.
		// {
		// 	name: "supports include",
		// 	args: args{
		// 		ctx:   context.TODO(),
		// 		host:  "included.github.com",
		// 		field: "identityfile",
		// 	},
		// 	want:    "hello-world-2",
		// 	wantErr: false,
		// },
		{
			name: "supports match",
			args: args{
				ctx:   context.TODO(),
				host:  "hello",
				field: "identityfile",
			},
			want:    "echo-hello",
			wantErr: false,
		},
	}

	os.Setenv("SSH_CONFIG_PATH", "ssh.config")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Get(tt.args.ctx, tt.args.host, tt.args.field)
			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Get() = %v, want %v", got, tt.want)
			}
		})
	}
}
