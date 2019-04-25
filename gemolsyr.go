package gemolsyr

import (
	"context"
	"math/rand"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
)

const DefaultSubsectionMinimumSize = 64

var DefaultMaxWorkers = uint32(runtime.NumCPU())

type LSystem struct {
	Parameters  Parameters
	env Environment

	currentTier uint

	rng  *rand.Rand
	tier []Module

	mu sync.Mutex

	subsectionMinimumSize uint32
	maxWorkers uint32
}

func New(parameters Parameters) LSystem {
	// Prepare RNG
	randomNumberGenerator := rand.New(rand.NewSource(parameters.Seed))

	// Prepare tier list
	return LSystem{
		Parameters:  parameters,
		currentTier: 0,
		rng:         randomNumberGenerator,
		tier:        parameters.Axiom,
		subsectionMinimumSize: DefaultSubsectionMinimumSize,
		maxWorkers: DefaultMaxWorkers,
	}
}

func (ls LSystem) SetSubsectionMinimumSize(size uint) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	ls.subsectionMinimumSize = uint32(size)
}

func (ls LSystem) SubsectionMinimumSize() uint {
	return uint(atomic.LoadUint32(&(ls.subsectionMinimumSize)))
}

// prepareRules associates each existing tier to a rule to be executed
func (ls LSystem) calculateRules(rules []Rule, input []Module) {
	// This stores the "matching" rules for any letter. This is reused in all iterations.
	matching := make([]Rule, 0, len(ls.Parameters.Rules))

	// Iterate through the elements of the tier to select the rules to be used for each Module
	for i, mod := range input {
		// Store the matching
		for _, r := range ls.Parameters.Rules {
			if r.Matches(&mod, input[:i], input[i+1:]) {
				matching = append(matching, r)
			}
		}

		// If there's more than one, check their priority and only keep the ones sharing the highest priority
		if len(matching) > 1 {
			// Compute maximum priority & the amount with the max priority
			maxPriority := 0
			amountWithMaxPriority := 0
			for _, r := range matching {
				p := r.Priority()

				// If we encounter a new max, we reset the amount of rules with max priority
				// If we encounter a rule with the same priority as max, we increment the amount with max priority
				if p > maxPriority {
					maxPriority = p
					amountWithMaxPriority = 1
				} else if p == maxPriority {
					amountWithMaxPriority++
				}
			}

			// Replace in-place the rules with only the ones with the maximum priority
			for oldIndex, newIndex := 0, 0; oldIndex < len(matching); oldIndex++ {
				examined := matching[oldIndex]
				if examined.Priority() == maxPriority {
					matching[newIndex] = examined
					newIndex++
				}
			}
			matching = matching[:amountWithMaxPriority]
		}

		// If there's still more than one, we execute the stochastic case, else we store
		if len(matching) > 1 {
			// First sum up the probabilities in order to check that it comes up under 1
			// If it doesn't, scale them up/down to 1
			var s float64
			for _, r := range matching {
				s += r.Probability()
			}
			scalingFactor := 1/s

			// Order the rules by their probabilities, ascending
			sort.SliceStable(matching, func(i, j int) bool {
				// We don't apply the scaling factor here as it's not necessary (linear operation)
				return matching[i].Probability() < matching[j].Probability()
			})

			// Then roll a random number
			n := ls.rng.Float64()
			cum := float64(0)
			for _, matchingRule := range matching {
				cum += scalingFactor * matchingRule.Probability()
				if n < cum {
					// Finally select the rule
					rules[i] = matchingRule
					break
				}
			}
		} else if len(matching) == 1{
			rules[i] = matching[0]
		} // Else, no matching rule means it won't be applied

		// Memclear matching (this should be optimised by the compiler to a single memclear)
		// I don't think it's worth it to not execute it on last iteration
		matching = matching[:cap(matching)] // Open it up to the full capacity
		for j := range matching { // Memclear
			matching[j] = nil
		}
		matching = matching[:0] // Close it all up all over again
	}
}

func (ls LSystem) calculateOutputSize(rules []Rule) int {
	var val int
	for _, r := range rules {
		if r != nil {
			val += r.OutputSize()
		}
	}
	return val
}

// Execute a rewrite
func (ls LSystem) rewrite(output []Module, input []Module , rules []Rule) error {
	// Apply the rules for each element
	outputCursor := 0
	env := wrapEnvironment(ls.env) // Reuse the same
	for inputCursor, inputModule := range input {
		rule := rules[inputCursor]

		env.prev = inputModule.Parameters

		// If there is a rule to apply
		if rule != nil {
			n, err := rule.Execute(output[outputCursor:], &inputModule, env)
			if err != nil {
				return err
			}

			outputCursor += n
		}
	}
	return nil
}

// Calculate number of splits for a given maximum of workers and minimum of subsection size
func (ls LSystem) splits() (splits uint32, size uint64, rem uint32) {
	l := uint64(len(ls.tier))

	if v := uint32(l/uint64(ls.subsectionMinimumSize)); v == 0 {
		splits = 1
	} else if v < ls.maxWorkers {
		splits = v
	} else {
		splits = ls.maxWorkers
	}

	return splits, l/uint64(splits), uint32(l)%splits
}

/*
Derivate runs runs one iteration of the l-system algorithm, heavily inspired from https://publik.tuwien.ac.at/files/PubDat_181216.pdf

	0. Split the input array into n, and launch n threads
	1 (T). Get rules to be applied to each module, applying context sensitive
	2 (T). Calculate required output size, based upon production, for each module and add it to a shared atomic variable
	3. Create a common output array
	4.(T). Rewrite
 */
func (ls *LSystem) Derivate(ctx context.Context) error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	// 0. Calculate amount of splits
	splits, size, rem := ls.splits()

	// Worker definitions
	rules := make([]Rule, len(ls.tier))
	sectionOutputSizes := make([]int, splits)
	outputSliceChan := make([]chan []Module, splits)
	for i := range outputSliceChan {
		outputSliceChan[i] = make(chan []Module, 1)
	}
	wg := sync.WaitGroup{}
	wg.Add(int(splits))
	for i,cursor := uint32(0), uint64(0); i < splits; i++ {
		// Calculate this subsection's size (can be +1 for the rem first elements)
		thisSize := size
		if i < rem {
			thisSize++
		}

		// Launch the worker
		go func(workerNumber uint32, cursor uint64) {
			inputSlice := ls.tier[cursor:cursor+thisSize]
			sectionRules := rules[cursor:cursor+thisSize]

			// Calculate rules
			ls.calculateRules(sectionRules, inputSlice)

			// Once we're done, we can calculate the output size
			sectionOutputSize := ls.calculateOutputSize(sectionRules)

			// Add to common value
			sectionOutputSizes[workerNumber] = sectionOutputSize

			// We're done here for this section
			wg.Done()

			// Now we wait for the output array creation, the one on which we'll write
			outputSlice :=  <- outputSliceChan[workerNumber]

			// Rewrite on the output slice
			err := ls.rewrite(outputSlice, inputSlice, sectionRules)
			if err != nil{
				panic("Error in rewriting")
			}

			// We're done here
			wg.Done()
		}(i, cursor)

		// Add to the cursor
		cursor += thisSize
	}

	// Wait for output size calculation
	wg.Wait()

	// Re-add values
	wg.Add(int(splits))

	// Add up all output sizes and create output slice
	var outputSize int
	for _, s := range sectionOutputSizes {
		outputSize += s
	}
	output := make([]Module, outputSize)

	// Distribute output slice (reslices)
	cursor := int(0)
	for i:=0; i < len(outputSliceChan); i++ {
		outputSliceChan[i] <- output[cursor: cursor + sectionOutputSizes[i]]
		cursor += sectionOutputSizes[i]
	}

	// Wait a last time
	wg.Wait()

	// Replace the tier
	ls.tier = output

	// Tier generated, ready to increment tier number
	ls.currentTier += 1

	return nil
}

// DerivateUntil runs iterations until a given number of tiers is achieved
func (ls *LSystem) DerivateUntil(ctx context.Context, maxTiers uint) error {
	for ls.currentTier <= maxTiers {
		err := ls.Derivate(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ls LSystem) Export() []Module {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	return ls.tier
}

func (ls LSystem) CurrentTier() uint {
	return ls.currentTier
}
