package errs

import (
	"errors"
	"io"
)

func WrapDeferedClose(c io.Closer, err *error) {
	_ = errors.Join(*err, c.Close())
}
