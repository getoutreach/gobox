package aws

import (
	"fmt"
	"os"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func Test_assumedToRole(t *testing.T) {
	type args struct {
		assumedRole string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "should properly parse principal_arn",
			args: args{
				assumedRole: "arn:aws:sts::182192988802:assumed-role/okta_eng_readonly_role/jared.allard@outreach.io",
			},
			want: "arn:aws:iam::182192988802:role/okta_eng_readonly_role",
		},
		{
			name: "should ignore invalid input",
			args: args{
				assumedRole: "hello world",
			},
			want: "hello world",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := assumedToRole(tt.args.assumedRole); got != tt.want { //nolint:scopelint
				t.Errorf("assumedToRole() = %v, want %v", got, tt.want) //nolint:scopelint
			}
		})
	}
}

func Test_needsRefresh(t *testing.T) {
	type args struct {
		copts *CredentialOptions
	}
	tests := []struct {
		name                string
		args                args
		wantNeedsNewCreds   bool
		credentialsContents string
		wantReason          string
	}{
		{
			name: "should refresh when selecting a role interactively",
			args: args{copts: &CredentialOptions{
				Role: RoleInteractive,
			}},
			wantNeedsNewCreds: true,
			wantReason:        "Refreshing AWS credentials since we are interactively selecting a role",
		},
		{
			name: "should refresh when file doesn't exist",
			args: args{copts: &CredentialOptions{
				FileName: "i/do/not/exist",
			}},
			wantNeedsNewCreds: true,
			wantReason:        "Credential file failed to load: open i/do/not/exist: no such file or directory",
		},
		{
			name:                "should refresh when file exists but is empty",
			credentialsContents: "[default]",
			wantNeedsNewCreds:   true,
			wantReason:          "No existing credentials",
		},
		{
			name: "should refresh when file has expired credentials",
			//nolint:lll // Why: Test case
			credentialsContents: `[default]
aws_access_key_id        = ACCESSKEY
aws_secret_access_key    = SECRETKEY
x_security_token_expires = 2006-01-02T15:04:05+07:00`,
			wantNeedsNewCreds: true,
			wantReason:        "Credentials are expired",
		},
		{
			name:                "should not refresh when credentials are still valid",
			credentialsContents: "[default]\nx_security_token_expires = " + time.Now().Add(20*time.Minute).Format(time.RFC3339),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.args.copts == nil {
				tt.args.copts = &CredentialOptions{}
			}
			tt.args.copts.Profile = "default"

			// Fake ~/.aws/credentials when specified
			if tt.credentialsContents != "" {
				tmpFile, err := os.CreateTemp("", "test-aws-credentials")
				assert.NilError(t, err, "failed to create temp file")
				defer tmpFile.Close()
				defer os.Remove(tmpFile.Name())

				_, err = tmpFile.WriteString(tt.credentialsContents)
				assert.NilError(t, err, "failed to write credentials contents to temp file")
				tt.args.copts.FileName = tmpFile.Name()

				fmt.Printf("==== Wrote %s as aws credentials fake ====\n", tmpFile.Name())
				fmt.Println(tt.credentialsContents)
				fmt.Println("==== End ====")
			}

			gotNeedsNewCreds, gotReason := needsRefresh(tt.args.copts)
			if gotNeedsNewCreds != tt.wantNeedsNewCreds {
				t.Errorf("needsRefresh() gotNeedsNewCreds = %v, want %v", gotNeedsNewCreds, tt.wantNeedsNewCreds)
			}
			if gotReason != tt.wantReason {
				t.Errorf("needsRefresh() gotReason = %v, want %v", gotReason, tt.wantReason)
			}
		})
	}
}
