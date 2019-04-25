package gemolsyr

import (
	"context"
	"fmt"
	"math"
	"testing"
)

type testRule struct{}

func (tt *testRule) Priority() int {
	return 1
}

func (tt *testRule) Matches(predecessor *Module, left []Module, right []Module) bool {
	return predecessor.Letter == 'V'
}

func (tt *testRule) Probability() float64 {
	return 1
}

var pair = []Module{
{
	Letter: 'V',
	Parameters: []float64{1},
},
{
	Letter: 'V',
	Parameters: []float64{1},
},
}

func (tt *testRule) Execute(to []Module, predecessor *Module, env Environment) (int, error) {
	copy(to, pair)
	return 2, nil
}

func (tt *testRule) OutputSize() int {
	return 2
}

var TestParameters = Parameters{
	Axiom: []Module{
		{
			Letter: 'V',
			Parameters: []float64{1},
		},
	},
	Constants: []Letter {
		'C',
	},
	Variables: []Letter {
		'V',
	},
	Rules: []Rule {
		&testRule{},
	},
}

var TestLSystem = New(TestParameters)

func BenchmarkLSystem_Derivate_InputLength(b *testing.B) {
	b.Skip()

	ctx := context.Background()
	for i := uint(0); i <= 15; i++ {
		// Precompute tier
		ls := New(TestParameters)
		ls.DerivateUntil(ctx, i)
		parameters := TestParameters
		parameters.Axiom = ls.Export()

		// Run sub-benchmark
		b.Run(fmt.Sprintf("%d", int(math.Pow(2,float64(i)))), func(b *testing.B) {
			// Prepare tier of length N
			for n := 0; n < b.N; n++ {
				ls := New(parameters)
				err := ls.Derivate(ctx)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkLSystem_Derivate(b *testing.B) {
	ctx := context.Background()

	// Precompute tier
	ls := New(TestParameters)
	ls.DerivateUntil(ctx, 12)
	parameters := TestParameters
	parameters.Axiom = ls.Export()

	for n := 0; n < b.N; n++ {
		ls := New(parameters)
		err := ls.Derivate(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}