// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Implements helpers for working with Github
// on local machines.

// Package github includes helper functions for standardized ways
// of interacting with Github across machines.
package github

import (
	"context"
	"net/http"

	"github.com/google/go-github/v57/github"
	"golang.org/x/oauth2"
)

// NewClient returns a new Github client using credentials from
// GetToken().
func NewClient(optFns ...Option) (*github.Client, error) {
	opts := &Options{}
	opts.apply(optFns...)

	token, err := GetToken()
	if err != nil {
		if opts.AllowUnauthenticated {
			opts.Logger.Warn("unable to get token, falling back to unauthenticated client")
			return github.NewClient(http.DefaultClient), nil
		}

		return nil, err
	}

	return github.NewClient(oauth2.NewClient(context.Background(),
		oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: string(token)},
		)),
	), nil
}
