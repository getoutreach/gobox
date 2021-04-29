package orerr

import (
	"errors"
	"fmt"
)

func ExampleRetryable() {
	process := func(attempt int) error {
		err := errors.New("something went wrong")
		if attempt < 2 {
			return Retryable(err)
		}

		return err
	}

	for i := 0; ; i++ {
		if err := process(i); err != nil {
			if IsRetryable(err) {
				fmt.Println("error is retryable")
				continue
			}

			fmt.Println("error is not retryable:", err)
			break
		}
	}

	// Output:
	// error is retryable
	// error is retryable
	// error is not retryable: something went wrong
}
