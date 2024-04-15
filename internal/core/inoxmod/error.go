package inoxmod

import (
	"fmt"

	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

type Error struct {
	BaseError      error
	Position       parse.SourcePositionRange
	AdditionalInfo string
}

func (err Error) Error() string {
	return fmt.Sprintf("%s %s", err.Position.String(), err.BaseError)
}

// CombineErrors combines errors into a single error with a multiline message.
func CombineErrors(errs []Error) error {

	if len(errs) == 0 {
		return nil
	}

	goErrors := make([]error, len(errs))
	for i, e := range errs {
		goErrors[i] = fmt.Errorf("%s %w", e.Position.String(), e.BaseError)
	}

	return utils.CombineErrors(goErrors...)
}
