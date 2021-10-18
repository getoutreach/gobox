// Package exec implements os/exec stdlib helpers
package exec

import (
	"os"
	"path/filepath"
	"testing"
)

func getCwd() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return dir
}

func TestResolveExecuable(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "should lookup path",
			args: args{
				path: "true",
			},
			want: filepath.Join(string(filepath.Separator), "usr", "bin", "true"),
		},
		{
			name: "should return abs path",
			args: args{
				path: "/hello/world/devenv",
			},
			want: filepath.Join(string(filepath.Separator), "hello", "world", "devenv"),
		},
		{
			name: "should clean abs path",
			args: args{
				path: "/hello/world/../devenv",
			},
			want: filepath.Join(string(filepath.Separator), "hello", "devenv"),
		},
		{
			name: "should make abs path",
			args: args{
				path: "./devenv",
			},
			want: filepath.Join(getCwd(), "devenv"),
		},
		{
			name: "should make abs path 2",
			args: args{
				path: "./hello/world/devenv",
			},
			want: filepath.Join(getCwd(), "hello", "world", "devenv"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveExecuable(tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveExecuable() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ResolveExecuable() = %v, want %v", got, tt.want)
			}
		})
	}
}
