package updater

import (
	"context"
	"reflect"
	"testing"

	"github.com/google/go-github/v34/github"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestGithubRelease(t *testing.T) {
	ctx := context.Background()
	g := NewGithubUpdater(ctx, logrus.New(), "", "jaredallard", "localizer")

	assert.Nil(t, g.Check(ctx), "validating client work")

	r, err := g.GetLatestVersion(ctx, "v0.9.0", false)
	assert.Nil(t, err, "unable to get latest version")

	if r.GetTagName() == "v0.9.0" {
		t.Error("got invalid version")
	}
}

func TestGithub_SelectAsset(t *testing.T) {
	ctx := context.Background()
	g := NewGithubUpdater(ctx, logrus.New(), "", "jaredallard", "localizer")

	type args struct {
		assets []*github.ReleaseAsset
		name   string
	}
	tests := []struct {
		name    string
		fields  *Github
		args    args
		want    *github.ReleaseAsset
		wantErr bool
	}{
		{
			name:   "should match asset",
			fields: g,
			args: args{
				name: "localizer",
				assets: []*github.ReleaseAsset{
					{
						ID:   github.Int64(28295890),
						Name: github.String("localizer_1.0.0_linux_amd64.tar.gz"),
					},
				},
			},
			want: &github.ReleaseAsset{
				ID:   github.Int64(28295890),
				Name: github.String("localizer_1.0.0_linux_amd64.tar.gz"),
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			g := &Github{
				gc:   tt.fields.gc,
				org:  tt.fields.org,
				repo: tt.fields.repo,
			}
			_, got, err := g.SelectAsset(ctx, tt.args.assets, tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("Github.SelectAsset() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Github.SelectAsset() = %v, want %v", got, tt.want)
			}
		})
	}
}
