package lsif

import (
	"github.com/aabizri/gemolsyr"
	"github.com/aabizri/gemolsyr/interchange/rules"
	"github.com/pkg/errors"
	"strconv"
)

func (format *Format) Import() (gemolsyr.Parameters, error) {
	// Build a param name to position map for each variable
	variableParamNameToPositionMap := make(map[rune]map[rune]uint8, len(format.Variables))
	for definedVariableName, definedVariable := range format.Variables {
		variableParamNameToPositionMap[definedVariableName] = definedVariable.ParameterNameToPositionMap()
	}

	// moduleList & adapt axioms
	axioms := make([]gemolsyr.Module, len(format.Axiom))
	for i, m := range format.Axiom {
		parameters := make([]float64, len(m.Parameters))
		for paramName, paramExpr := range m.Parameters {
			paramPos := int(variableParamNameToPositionMap[m.Letter][paramName])
			paramValue, err := strconv.ParseFloat(paramExpr, 64)
			if err != nil {
				return gemolsyr.Parameters{}, errors.Wrapf(err, "Error while parsing axiom, position % letter %s parameter %s value %s", i, m.Letter, paramName, paramExpr)
			}
			parameters[paramPos] = paramValue
		}

		axioms[i] = gemolsyr.Module{
			Letter:     gemolsyr.Letter(m.Letter),
			Parameters: parameters,
		}
	}
	parameters := gemolsyr.Parameters{
		Axiom: axioms,
	}

	// Build the rules
	builtRules := make([]gemolsyr.Rule, len(format.Rules))
	for ri, definedRule := range format.Rules {
		// For each rule, parse each created module parameters expression
		rewritten := make([]map[rune]func(environment gemolsyr.Environment) float64, len(definedRule.Rewrite))
		for i, rewriteModule := range definedRule.Rewrite {
			parameters := make(map[rune]func(environment gemolsyr.Environment) float64, len(rewriteModule.Parameters))
			for parameterName, parameterExpression := range rewriteModule.Parameters {
				f, err := parseExpression(parameterExpression)
				if err != nil {
					return gemolsyr.Parameters{}, err
				}
				parameters[parameterName] = f
			}
			rewritten[i] = parameters
		}

		// Create the overall rewriting function
		f := func(out []gemolsyr.Module, predecessor *gemolsyr.Module, env gemolsyr.Environment) (int, error) {
			n := 0
			for _, paramNameToFuncMap := range rewritten {
				// For each parameter name, transform it to its position
				mod := &gemolsyr.Module{
					Letter: gemolsyr.Letter(definedRule.Rewrite[n].Letter),
				}

				parameters := make([]float64, len(paramNameToFuncMap))
				for paramName, paramFunc := range paramNameToFuncMap {
					paramPosition := int(variableParamNameToPositionMap[definedRule.Rewrite[n].Letter][paramName])

					parameters[paramPosition] = paramFunc(env)
				}
				mod.Parameters = parameters

				out[n] = *mod
				n++
			}

			return n, nil
		}

		// Create the rule
		builtRules[ri] = rules.NewRule(
			gemolsyr.Letter(definedRule.From),
			f,
			len(definedRule.Rewrite),
			nil,
			nil,
			1,
		)
	}

	parameters.Rules = builtRules
	return parameters, nil
}
