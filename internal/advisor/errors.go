package advisor

import "fmt"

type badRequestError struct {
	msg string
}

func (e badRequestError) Error() string { return e.msg }

func errBadRequest(msg string) error {
	return badRequestError{msg: msg}
}

func IsBadRequest(err error) bool {
	_, ok := err.(badRequestError)
	return ok
}

func fmtError(wrap string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", wrap, err)
}
