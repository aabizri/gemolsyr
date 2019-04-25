package lsif

import (
	"github.com/Knetic/govaluate"
	"github.com/pkg/errors"
	"github.com/aabizri/gemolsyr"
	"strconv"
)

type expressionFunction func(environment gemolsyr.Environment) float64

type wrappedVariablesForExpression struct {
	gemolsyr.Environment
}

func (wvfp wrappedVariablesForExpression) Get(name string) (interface{}, error) {
	val, err := wvfp.Get(name)
	if err != nil {
		return nil, errors.Errorf("Couldn't find %s", name)
	}
	return val, nil
}

func parseExpression(asString string) (expressionFunction, error) {
	// Check if possible to simplify if it just a scalar
	if scalar, err := strconv.ParseFloat(asString, 64); err != nil {
		return func(_ gemolsyr.Environment) float64 {
			return scalar
		}, nil
	}


	evaluable, err := govaluate.NewEvaluableExpression(asString)
	if err != nil {
		return nil, errors.Wrapf(err, "Error while parsing expression")
	}

	// Parse expressions
	return func(variablesForExpression gemolsyr.Environment) float64 {
		wrapped := wrappedVariablesForExpression{variablesForExpression}

		resAsInterface, err := evaluable.Eval(wrapped)
		if err != nil {
			panic("Error while evaluating " + err.Error())
		}

		resAsFloat, ok := resAsInterface.(float64)
		if !ok {
			panic("Failure while casting")
		}

		return resAsFloat
	}, nil
}
