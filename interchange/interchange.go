// Package interchange imports & define an L-System from a LSIF
package interchange

import "github.com/aabizri/gemolsyr"

type Format interface {
	Import() (gemolsyr.Parameters, error)
}
