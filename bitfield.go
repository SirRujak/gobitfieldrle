package gobitfieldrle

import (
	"errors"
	"math"
)

type Bitfield struct {
	GrowVal float64
	Buffer  []uint8
}

type BitfieldOpts struct {
	Grow float64
}

func (bitfield *Bitfield) Init(dataSlice *([]uint8), dataNumber *int, opts *BitfieldOpts) error {
	var err error
	if dataSlice == nil && dataNumber == nil && opts == nil {
		err = errors.New("no arguments supplied to initializer")
		return err
	}

	if dataSlice != nil {
		bitfield.Buffer = *dataSlice
	} else if dataNumber != nil {
		bitfield.Buffer = make([]uint8, getByteSize(*dataNumber))
	}

	if opts != nil {
		bitfield.GrowVal = opts.Grow
	}
	return nil
}

func (bitfield *Bitfield) Length() int {
	return len(bitfield.Buffer)
}

func getByteSize(num int) int {
	var out int
	out = num >> 3
	if num%8 != 0 {
		out++
	}
	return out
}

func (bitfield *Bitfield) Get(i int) (bool, error) {
	var err error
	var j int
	j = i >> 3
	// For if i is out of range.
	if j >= len(bitfield.Buffer) {
		return false, nil
	}
	var k uint
	if i >= 0 {
		k = uint(i)
	} else {
		err = errors.New("i needs to be a positive intiger")
		return false, err
	}
	var tempOut uint8
	tempOut = bitfield.Buffer[j] & (128 >> (k % 8))
	if tempOut == 0 {
		return false, nil
	}
	return true, nil

}

func (bitfield *Bitfield) Set(i int, value bool) error {
	var j int
	j = i >> 3
	var k uint
	if i < 0 {
		return errors.New("i must be a positive integer")
	}
	k = uint(i)
	if value {
		if len(bitfield.Buffer) <= j {
			if bitfield.GrowVal == math.Inf(1) {
				bitfield.Grow(max(j+1, 2*len(bitfield.Buffer)))
			}

			bitfield.Grow(max(j+1, min(2*len(bitfield.Buffer), round(bitfield.GrowVal))))
		}
		bitfield.Buffer[j] |= 128 >> (k % 8)
	} else if j < len(bitfield.Buffer) {
		bitfield.Buffer[j] &= ^(128 >> (k % 8))
	}
	return nil
}

func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func (bitfield *Bitfield) Grow(length int) {
	var tempGrowVal int
	if bitfield.GrowVal == math.Inf(1) {
		tempGrowVal = length
	} else {
		tempGrowVal = int(bitfield.GrowVal)
	}
	if len(bitfield.Buffer) < length && length <= tempGrowVal {
		var tempLength int
		tempLength = length - len(bitfield.Buffer)
		bitfield.Buffer = append(bitfield.Buffer, make([]uint8, tempLength)...)
	}
}

func round(val float64) int {
	if val < 0 {
		return int(val - 0.5)
	}
	return int(val + 0.5)
}
