package gemolsyr

import (
	"errors"
	"strconv"
	"strings"
)

const PrevPrefix = "prev_"

type Environment interface {
	Get(v string) (float64, error)
}

type wrappedEnvironment struct {
	Inner Environment

	prev []float64
}

func (wenv *wrappedEnvironment) Get(v string) (float64, error) {
	if strings.HasPrefix(v, PrevPrefix) {
		n, err := strconv.Atoi(v[len(PrevPrefix):])
		if err != nil {
			return 0, err
		}

		if n < 0 || n >= len(wenv.prev) {
			return 0, errors.New("call to unexistent previous variable")
		}

		return wenv.prev[n], nil
	} else if wenv.Inner != nil {
		return wenv.Inner.Get(v)
	} else {
		return 0, errors.New("call to undefined variable as there is no environment defined")
	}
}

func wrapEnvironment(inner Environment) *wrappedEnvironment {
	return &wrappedEnvironment{inner, nil}
}