package orerr_test

import (
	"errors"
	"fmt"

	"github.com/getoutreach/gobox/pkg/orerr"
)

const ErrUsernameTaken orerr.SentinelError = "username already taken"

func createUser(username string) error {
	return ErrUsernameTaken
}

func ExampleSentinelError() {
	if err := createUser("joe"); err != nil {
		if errors.Is(err, ErrUsernameTaken) {
			fmt.Println("User 'joe' already exists")
			return
		}

		panic(err)
	}

	// Output: User 'joe' already exists
}
