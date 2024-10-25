// flag_size.go -- value implementation of a size input
//
// A size is an integer with a suffix of k, M, G, T, P, E
// denoting kilo, Mega, Giga, Tera, Peta, Exa (multiples of 1024)

package main

import (
	//flag "github.com/opencoff/pflag"
	"github.com/opencoff/go-utils"
)

type SizeValue uint64

//var _ flag.Value = &SizeValue(uint64(0))

func NewSizeValue() *SizeValue {
	v := SizeValue(uint64(0))
	return &v
}

func (v *SizeValue) String() string {
	return utils.HumanizeSize(uint64(*v))
}

func (v *SizeValue) Set(s string) error {
	z, err := utils.ParseSize(s)
	*v = SizeValue(z)
	return err
}

func (v *SizeValue) Type() string {
	return "size"
}

func (v *SizeValue) Value() uint64 {
	return uint64(*v)
}
