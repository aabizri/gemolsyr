package main

import (
	"context"
	"fmt"
	"github.com/aabizri/gemolsyr"
	"github.com/aabizri/gemolsyr/interchange/lsif"
	"io"
	"log"
	"os"
	"reflect"
)

const (
	workersMax         = 1
	sequencerQueueSize = 5
	orderInQueueSize   = 5
	orderOutQueueSize  = 0
	outQueueSize       = 5
)

func main() {
	listen(os.Stdout, os.Stdin, os.Stderr)
}

func listen(w io.Writer, r io.Reader, ew io.Writer) {
	in, out := buildPipeline()

	// Signal that the pipeline is empty
	closed := make(chan struct{})
	go func() {
		seq := -1
		for {
			ls, ok := <-out
			if !ok {
				closed <- struct{}{}
				return
			}

			seq++
			fmt.Fprintf(ew, "Sequence %d read\n", seq)
			_, err := fmt.Fprintf(w, "%s\n", ls.Export())
			if err != nil {
				panic("Error in writing to out")
			}
		}
	}()

	lsifDecoder := lsif.NewDecoder(r)
	for {
		format, err := lsifDecoder.Decode()
		if err == io.EOF {
			close(in)
			break
		} else if err != nil {
			log.Fatalf("Error while decoding lsif: %v\n", err)
		}

		parameters, err := format.Import()
		if err != nil {
			log.Fatalf("Error while importing format: %v\n", err)
		}

		ls := gemolsyr.New(parameters)
		in <- ls
	}

	<-closed
}

func buildPipeline() (in chan<- gemolsyr.LSystem, out <-chan gemolsyr.LSystem) {
	sequencerQueue := make(chan gemolsyr.LSystem, sequencerQueueSize)
	orderInQueue := make(chan *order, orderInQueueSize)
	outQueue := make(chan gemolsyr.LSystem, outQueueSize)
	orderOutQueues := make([]<-chan *order, workersMax)

	go sequence(sequencerQueue, orderInQueue)
	for i := range orderOutQueues {
		q := make(chan *order, orderOutQueueSize)
		go run(orderInQueue, q)
		orderOutQueues[i] = q
	}
	go resolve(orderOutQueues, outQueue)

	return sequencerQueue, outQueue
}

type order struct {
	ls  gemolsyr.LSystem
	seq int
}

func sequence(in <-chan gemolsyr.LSystem, orderInQueue chan<- *order) {
	seq := 0
	for ls := range in {
		orderInQueue <- &order{
			ls,
			seq,
		}
		seq++
	}
	close(orderInQueue)
}

func run(orderInQueue <-chan *order, orderOutQueue chan<- *order) {
	for {
		o, ok := <-orderInQueue
		if !ok {
			close(orderOutQueue)
			return
		}

		o.ls.DerivateUntil(context.Background(), 15)
		orderOutQueue <- o
	}
}

// resolve resolves the inputs to their correct sequence
// If this implementation doesn't work, we could launch a goroutine per queue,
// that would use a local buffer and send itself the next value to the out channel.
// Removing the need for a weird select (see scratch ?)
// TODO: PROFILE AND OPTIMISE
func resolve(orderOutQueues []<-chan *order, gemolsyrOutQueue chan<- gemolsyr.LSystem) {
	seq := -1

	// Buffer is only of one per queue
	// The idea is: we read one per queue, if it is the next in the sequence,
	// it is outputted. If it is in advance, that queue's spot in the buffer is taken,
	// as such it won't be considered for select on the next iteration. In the worst case,
	// all slots are taken but one: the currently considered queue's output. Thus, only that one will
	// be selected on.
	// The buffer small size is not a problem, as the real buffering system is the buffered channel.
	buffer := make([]*order, len(orderOutQueues))

	// The mask marks an order out queue as being closed, so that they are disregarded for
	// queue selection
	mask := make([]bool, len(orderOutQueues))

	// Function to empty the buffer if possible
	var checkBuffer func()
	checkBuffer = func() {
		for i, buffered := range buffer {
			if buffered != nil && buffered.seq == seq+1 {
				gemolsyrOutQueue <- buffered.ls
				seq++

				buffer[i] = nil
				checkBuffer()
			}
		}
	}

	// Create one SelectCase per orderOutQueues
	selectCases := make([]reflect.SelectCase, len(orderOutQueues))
	for i := 0; i < len(orderOutQueues); i++ {
		ooq := orderOutQueues[i]
		sq := reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ooq),
		}
		selectCases[i] = sq
	}

	// Channel case subselection
	subSelectCases := make([]reflect.SelectCase, 0, len(orderOutQueues))

	// Map of subselected channel index to order queue index
	subSelectCaseToOrderQueueIndex := make([]int, 0, len(orderOutQueues))

	for {
		// If every channel is masked, empty buffer, close output channel & return
		allMasked := false
		for _, masked := range mask {
			if masked {
				allMasked = true
			}
		}
		if allMasked {
			fmt.Println("ALL MASKED")
			checkBuffer()
			close(gemolsyrOutQueue)
			return
		}

		// Select the channels
		for i, sq := range selectCases {
			// Skip the ones already used
			if buffer[i] == nil && !mask[i] {
				// The append here is low-cost as the capacity of the slice is well dimensionned
				subSelectCases = append(subSelectCases, sq)
				subSelectCaseToOrderQueueIndex = append(subSelectCaseToOrderQueueIndex, i)
			}
		}

		// If there are none it is an error as when a channel is masked, the buffer is checked, as such it should never
		// happen that not all channel is masked yet there are no cases for select. Unless the sequence number aren't
		// incremental.
		if len(subSelectCases) == 0 {
			panic("No cases produced, are the sequence number really incremental ?")
		}

		// Listen for them
		chosen, recv, ok := reflect.Select(subSelectCases)
		if !ok {
			// Mask worker for case selection (shutdown procedure: finish all buffered work and stop)
			mask[subSelectCaseToOrderQueueIndex[chosen]] = true

			// Check the buffer
			checkBuffer()

			continue
		}
		o, ok := recv.Interface().(*order)
		if !ok {
			panic("This should not happen: non-*order type received")
		}

		// If sequence number is the next one, send it over and increment sequence number
		// and empty the buffer if possible. If not, put it in the buffer.
		if o.seq == seq+1 {
			gemolsyrOutQueue <- o.ls
			seq++

			checkBuffer()
		} else {
			orderQueueIndexForRecv := subSelectCaseToOrderQueueIndex[chosen]
			buffer[orderQueueIndexForRecv] = o
		}

		// Reslice the subSelectCases
		subSelectCases = subSelectCases[0:0]
		subSelectCaseToOrderQueueIndex = subSelectCaseToOrderQueueIndex[0:0]
	}
}