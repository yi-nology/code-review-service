package postgres

import (
	"errors"

	"github.com/yi-nology/code-review-service/internal/repository/repoerr"

	gormDB "gorm.io/gorm"
)

func wrapNotFound(err error) error {
	if errors.Is(err, gormDB.ErrRecordNotFound) {
		return repoerr.ErrNotFound
	}
	return err
}
