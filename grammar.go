package gemolsyr

import "strconv"

type Letter rune

type Module struct {
	Letter     Letter
	Parameters []float64
}

// Module stringifier
func (m Module) String() string {
	out := string(m.Letter)
	if m.Parameters == nil || len(m.Parameters) == 0 {
		return out
	}

	out += "("
	for i, param := range m.Parameters {
		out += strconv.FormatFloat(param, byte('f'), -1, 64)
		if i+1 != len(m.Parameters) {
			out += ", "
		}
	}
	out += ")"
	return out
}

type Parameters struct {
	Axiom     []Module
	Constants []Letter
	Variables []Letter
	Rules     []Rule
	Seed      int64
}

type Rule interface {
	// In case of multiple-match, how much is the priority of that rule, higher takes precedence
	Priority() int

	// Whether it matches the context
	Matches(predecessor *Module, left []Module, right []Module) bool

	// The probability of it, compared to all same-priority matches
	Probability() float64

	// Execute it
	Execute(to []Module, predecessor *Module, env Environment) (int, error)

	// Return output size
	OutputSize() int
}
