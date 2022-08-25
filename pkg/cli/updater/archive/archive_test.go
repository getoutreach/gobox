// Copyright 2022 Outreach Corporation. All Rights Reserved.

package archive

import (
	"bytes"
	"context"
	_ "embed"
	"io"
	"reflect"
	"testing"
)

//go:embed testdata/tar.tar
var testdataTar []byte

//go:embed testdata/tar.tar.bz2
var testdataTarBz2 []byte

//go:embed testdata/tar.tar.gz
var testdataTarGz []byte

//go:embed testdata/tar.tar.xz
var testdataTarXz []byte

//go:embed testdata/zip.zip
var testdataZip []byte

//go:embed testdata/tar_with_multiple_files.tar
var testdataTarWithMultipleFiles []byte

//go:embed testdata/tar_with_a_directory.tar
var testdataTarWithADir []byte

func TestExtract(t *testing.T) {
	basicHeader := &Header{
		Name: "abc.txt",
		Mode: 33279,
		Size: 4,
		Type: HeaderTypeFile,
	}

	type args struct {
		ctx         context.Context
		archiveName string
		r           io.Reader
		optFns      []ExtractOptionFunc
	}
	tests := []struct {
		name    string
		args    args
		want    string
		want1   *Header
		wantErr bool
	}{
		{
			name: "should support find file by name",
			args: args{
				ctx:         context.Background(),
				archiveName: "tar.tar",
				r:           bytes.NewReader(testdataTar),
				optFns: []ExtractOptionFunc{
					WithFilePath("abc.txt"),
				},
			},
			want:    "xyz\n",
			want1:   basicHeader,
			wantErr: false,
		},
		{
			name: "should support find file with function",
			args: args{
				ctx:         context.Background(),
				archiveName: "tar.tar",
				r:           bytes.NewReader(testdataTar),
				optFns: []ExtractOptionFunc{
					WithFilePathSelector(func(s string) bool {
						return s == "abc.txt"
					}),
				},
			},
			want:    "xyz\n",
			want1:   basicHeader,
			wantErr: false,
		},
		{
			name: "should return error when file not found",
			args: args{
				ctx:         context.Background(),
				archiveName: "tar.tar",
				r:           bytes.NewReader(testdataTar),
				optFns: []ExtractOptionFunc{
					WithFilePath("abcdef.txt"),
				},
			},
			want:    "",
			want1:   nil,
			wantErr: true,
		},
		{
			name: "should return correct file",
			args: args{
				ctx:         context.Background(),
				archiveName: "tar_with_multiple_files.tar",
				r:           bytes.NewReader(testdataTarWithMultipleFiles),
				optFns: []ExtractOptionFunc{
					WithFilePath("dce.txt"),
				},
			},
			want: "zxy\n",
			want1: &Header{
				Name: "dce.txt",
				Mode: 33279,
				Size: 4,
				Type: HeaderTypeFile,
			},
			wantErr: false,
		},
		{
			name: "should return file at directory",
			args: args{
				ctx:         context.Background(),
				archiveName: "tar_with_dir.tar",
				r:           bytes.NewReader(testdataTarWithADir),
				optFns: []ExtractOptionFunc{
					WithFilePath("xyz/a_path.txt"),
				},
			},
			want: "dddd\n",
			want1: &Header{
				Name: "xyz/a_path.txt",
				Mode: 33279,
				Size: 5,
				Type: HeaderTypeFile,
			},
			wantErr: false,
		},
		{
			name: "should return error when unsupported archive type",
			args: args{
				ctx:         context.Background(),
				archiveName: "7z.7z",
				r:           nil,
				optFns:      []ExtractOptionFunc{},
			},
			want:    "",
			want1:   nil,
			wantErr: true,
		},
		{
			name: "should return error when filepath or selector is nil",
			args: args{
				ctx:         context.Background(),
				archiveName: "7z.7z",
				r:           nil,
				optFns:      []ExtractOptionFunc{},
			},
			want:    "",
			want1:   nil,
			wantErr: true,
		},
		{
			name: "should support support tar bz2",
			args: args{
				ctx:         context.Background(),
				archiveName: "tar.tar.bz2",
				r:           bytes.NewReader(testdataTarBz2),
				optFns: []ExtractOptionFunc{
					WithFilePath("abc.txt"),
				},
			},
			want:    "xyz\n",
			want1:   basicHeader,
			wantErr: false,
		},
		{
			name: "should support support tar gz",
			args: args{
				ctx:         context.Background(),
				archiveName: "tar.tar.gz",
				r:           bytes.NewReader(testdataTarGz),
				optFns: []ExtractOptionFunc{
					WithFilePath("abc.txt"),
				},
			},
			want:    "xyz\n",
			want1:   basicHeader,
			wantErr: false,
		},
		{
			name: "should support support tar xz",
			args: args{
				ctx:         context.Background(),
				archiveName: "tar.tar.xz",
				r:           bytes.NewReader(testdataTarXz),
				optFns: []ExtractOptionFunc{
					WithFilePath("abc.txt"),
				},
			},
			want:    "xyz\n",
			want1:   basicHeader,
			wantErr: false,
		},
		{
			name: "should support support zip",
			args: args{
				ctx:         context.Background(),
				archiveName: "zip.zip",
				r:           bytes.NewReader(testdataZip),
				optFns: []ExtractOptionFunc{
					WithFilePath("abc.txt"),
				},
			},
			want: "xyz\n",
			want1: &Header{
				Name: "abc.txt",
				// Zip doesn't support full UNIX modes
				Mode: 420,
				Size: 4,
				Type: HeaderTypeFile,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := Extract(tt.args.ctx, tt.args.archiveName, tt.args.r, tt.args.optFns...)
			if (err != nil) != tt.wantErr {
				t.Errorf("Extract() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			var strGot string
			if got != nil {
				byt, err := io.ReadAll(got)
				if err != nil {
					t.Errorf("Extract() got ReadAll error = %v", err)
					return
				}
				if err := got.Close(); err != nil {
					t.Errorf("Extract() got Close error = %v", err)
					return
				}

				strGot = string(byt)
			}

			if !reflect.DeepEqual(strGot, tt.want) {
				t.Errorf("Extract() got_readcloser (string) = %v, want %v", strGot, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("Extract() got_header = %v, want %v", got1, tt.want1)
			}
		})
	}
}
