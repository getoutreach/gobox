// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Implements helpers for working with Github
// on local machines.

// Package github includes helper functions for standardized ways
// of interacting with Github across machines.
package github

import (
	"context"

	"github.com/google/go-github/v43/github"
	"golang.org/x/oauth2"
)

// NewClient returns a new Github client using credentials from
// GetToken().
func NewClient() (*github.Client, error) {
	token, err := GetToken()
	if err != nil {
		return nil, err
	}

	return github.NewClient(oauth2.NewClient(context.Background(),
		oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: string(token)},
		)),
	), nil
}
