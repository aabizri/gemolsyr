// Package lsif is the reference implementation for the L-System Interchange Format
package lsif

import (
	"github.com/aabizri/yaml"
	"io"
)

type Format struct {
	Axiom     []Module
	Constants []rune
	Variables map[rune]Variable
	Rules     []Rule
}

type Variable struct {
	Parameters map[uint8]VariableParameter
}

func (v Variable) ParameterNameToPositionMap() map[rune]uint8 {
	paramNameToPositionMap := make(map[rune]uint8, len(v.Parameters))
	for position, param := range v.Parameters {
		paramNameToPositionMap[param.Name] = position
	}
	return paramNameToPositionMap
}


type VariableParameter struct {
	Name      rune
	Operators []rune
}

type Rule struct {
	From    rune
	Rewrite []Module
}

type Module struct {
	Letter     rune
	Parameters map[rune]string
}

type Decoder struct {
	in io.Reader
	yamlDecoder *yaml.Decoder
}

func NewDecoder(in io.Reader) *Decoder {
	return &Decoder {
		in: in,
		yamlDecoder: yaml.NewDecoder(in),
	}
}

func (dec *Decoder) Decode() (*Format, error) {
	format := &Format{}
	// Read until yaml multi-document delimiter and/or until EOF
	err := dec.yamlDecoder.Decode(format)
	return format, err
}