package dataset

import "errors"

var (
	ErrDuplicateData       = errors.New("didn't expect data duplication")
	ErrNoValidElements     = errors.New("source contained no valid elements")
	ErrUnexpectedIPVersion = errors.New("unexpected IP version")
	ErrInvalidPrefix       = errors.New("invalid prefix")
	ErrInvalidAddr         = errors.New("invalid addr")
	ErrInvalidASN          = errors.New("invalid ASN")
)
