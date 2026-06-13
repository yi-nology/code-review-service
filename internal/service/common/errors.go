package common

import (
	mw "github.com/oekit/middleware"
)

var (
	ErrNotFound   = mw.NewNotFoundError
	ErrBadRequest = mw.NewBadRequestError
	ErrConflict   = func(msg string) *mw.BizError { return mw.NewBizError(409, msg) }
	ErrInternal   = func(msg string) *mw.BizError { return mw.NewBizError(500, msg) }
)
