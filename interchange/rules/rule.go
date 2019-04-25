package rules

import "github.com/aabizri/gemolsyr"

var ensureInterfaceCompliance gemolsyr.Rule = &GeneralRule{}

type ExecutionFunction func(output []gemolsyr.Module, predecessor *gemolsyr.Module, variables gemolsyr.Environment) (int, error)

// A GeneralRule supports
// - Classic
// - Stochastic
// - Context-Sensitive
// Rules
type GeneralRule struct {
	On        gemolsyr.Letter
	WithLeft  []gemolsyr.Letter
	WithRight []gemolsyr.Letter
	Do        ExecutionFunction
	Size int

	// Encoded in 1-Probability
	OneMinusProbability float64
}

func (r *GeneralRule) Priority() int {
	// If it is context-sensitive, return 1
	if r.ContextSensitive() {
		return 1
	}

	return 0
}

func (r *GeneralRule) Matches(predecessor *gemolsyr.Module, left []gemolsyr.Module, right []gemolsyr.Module) bool {
	// Check that the predecessor matches
	if predecessor.Letter != r.On {
		return false
	}

	// If there is no context-sensitiveness, we're done
	if !r.ContextSensitive() {
		return true
	}

	// Check that the context matches
	// First check that the given left & right are greater than the required context by the rule
	if len(left) < len(r.WithLeft) || len(right) < len(r.WithRight) {
		return false
	}

	// Check left going from right-to-left
	for i := 0; i < len(r.WithLeft); i++ {
		if left[len(left)-1-i].Letter != r.WithLeft[len(r.WithLeft)-1-i] {
			return false
		}
	}

	// Check right going from left-to-right
	for i := 0; i < len(r.WithRight); i++ {
		if left[i].Letter != r.WithLeft[i] {
			return false
		}
	}

	return true
}

func (r *GeneralRule) Probability() float64 {
	return 1 - r.OneMinusProbability
}

func (r *GeneralRule) Execute(output []gemolsyr.Module, predecessor *gemolsyr.Module, env gemolsyr.Environment) (int, error) {
	return r.Do(output, predecessor, env)
}

func (r *GeneralRule) OutputSize() int {
	return r.Size
}

func (r *GeneralRule) ContextSensitive() bool {
	return (r.WithLeft != nil && len(r.WithLeft) > 0) || (r.WithRight != nil && len(r.WithRight) > 0)
}

func NewRuleClassic(on gemolsyr.Letter, rewrite []gemolsyr.Module) *GeneralRule {
	return NewRuleNonParametric(on, rewrite, nil, nil, 1)
}

func NewRuleStochastic(on gemolsyr.Letter, rewrite []gemolsyr.Module, probability float64) *GeneralRule {
	return NewRuleNonParametric(on, rewrite, nil, nil, probability)
}

func NewRuleContextSensitive(on gemolsyr.Letter, rewrite []gemolsyr.Module, left []gemolsyr.Letter, right []gemolsyr.Letter) *GeneralRule {
	return NewRuleNonParametric(on, rewrite, left, right, 1)
}

func NewRuleNonParametric(on gemolsyr.Letter, rewrite []gemolsyr.Module, left []gemolsyr.Letter, right []gemolsyr.Letter, probability float64) *GeneralRule {
	f := func(output []gemolsyr.Module,_ *gemolsyr.Module, _ gemolsyr.Environment) (int, error) {
		n := copy(output, rewrite)
		return n, nil
	}
	return NewRule(on, f, len(rewrite), left, right, probability)
}

func NewRule(on gemolsyr.Letter, do ExecutionFunction, size int, left []gemolsyr.Letter, right []gemolsyr.Letter, probability float64) *GeneralRule {
	return &GeneralRule{
		On:                  on,
		Do:                  do,
		Size: size,
		WithLeft:            left,
		WithRight:           right,
		OneMinusProbability: 1 - probability,
	}
}
