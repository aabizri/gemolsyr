package main

import (
	"github.com/aabizri/gemolsyr/interchange/lsif"
	"github.com/aabizri/gemolsyr"
	"os"
	"testing"
)

func TestListen(t *testing.T) {
	f, err := os.Open("testdata/stream.lsif.yml")
	if err != nil {
		t.Fatalf("Couldn't open test data file: %v", err)
	}
	listen(os.Stdout, f, os.Stderr)
}

func BenchmarkPipeline(b *testing.B) {
	f, err := os.Open("testdata/single.lsif.yml")
	if err != nil {
		b.Fatalf("Couldn't open test data file: %v", err)
	}

	dec := lsif.NewDecoder(f)
	format, err := dec.Decode()
	if err != nil{
		b.Fatalf("Couldn't parse lsif: %v", err)
	}
	parameters, err := format.Import()
	if err != nil {
		b.Fatalf("Couldn't import lsif: %v", err)
	}


	// Build pipeline
	in, out := buildPipeline()

	// Dev-null the out
	go func() {
		for {
			_, ok := <- out
			if !ok {
				return
			}
		}
	}()

	for n := 0; n < b.N; n++ {
		b.StopTimer()
		ls := gemolsyr.New(parameters)
		b.StartTimer()
		in <- ls
	}
}