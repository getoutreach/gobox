// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: dummy file to make gobox importable by Windows.

package logfile

import (
	"github.com/pkg/errors"
)

func Hook() error {
	return errors.New("not implemented")
}
