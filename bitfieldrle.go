package gobitfieldrle

import (
	"errors"

	"github.com/SirRujak/govarint"
)

//TODO: Probably should move everything over to uint rather than using int and uint and converting between them.

type Align struct {
}

type State struct {
	InputOffset  uint
	InputLength  uint
	Input        Bitfield
	OutputOffset uint
	Output       []uint8
}

func (state *State) Init(bitfield Bitfield, buffer *([]uint8), offset uint) {
	state.InputOffset = 0
	state.InputLength = uint(bitfield.Length())
	state.Input = bitfield
	state.OutputOffset = offset
	state.Output = *buffer
}

func Encode(bitfield Bitfield, buffer *([]uint8), offset *uint) []uint8 {
	var internalOffset uint
	if offset == nil {
		internalOffset = 0
	} else {
		internalOffset = *offset
	}

	var internalBuffer []uint8
	if buffer == nil {
		internalBuffer = make([]uint8, EncodingLength(bitfield))
	} else {
		internalBuffer = *buffer
	}

	var state State
	state = State{}
	state.Init(bitfield, &internalBuffer, internalOffset)
	state.RLE()
	return internalBuffer
}

func (state *State) RLE() error {
	var err error
	var len uint
	var bits uint8
	len = 0
	bits = 0
	var input Bitfield
	input = state.Input

	// Check and disregard all trailing zeros in the input slice.
	for state.InputLength > 0 && input.Buffer[state.InputLength-1] != uint8(0) {
		state.InputLength--
	}

	// Check if the new bits are the same as the current run. If so increase the length counter.
	// Otherwise, send the update to the encoder and reset our length tracker.
	for i := uint(0); i < state.InputLength; i++ {
		if input.Buffer[i] == bits {
			len++
			continue
		}

		if len != 0 {
			state.EncodeUpdate(i, len, bits)
		}

		if input.Buffer[i] == 0 || input.Buffer[i] == 255 {
			bits = input.Buffer[i]
			len = 1
		} else {
			len = 0
		}

		if len != 0 {
			state.EncodeUpdate(state.InputLength, len, bits)
		}

		err = state.EncodeFinal()
		if err != nil {
			return err
		}
	}
	return nil
}

func (state *State) EncodeHead(end uint) error {
	var err error
	var headLength uint
	headLength = uint(end - state.InputOffset)
	var tempBuffer []byte
	tempBuffer = make([]byte, govarint.Length(2*headLength))
	var encoder govarint.Encode
	_, err = encoder.Encode(2*headLength, &tempBuffer, nil)
	if err != nil {
		return err
	}
	state.OutputOffset += encoder.Bytes
	// Copy from state.Input.Buffer
	// target is state.Output
	// targetStart is state.OutputOffset
	// sourceStart is state.inputOffset
	// sourceEnd is end
	copy(state.Output[state.OutputOffset:], state.Input.Buffer[state.InputOffset:end])
	state.OutputOffset += headLength
	return nil
}

func (state *State) EncodeFinal() error {
	var headLength uint
	var err error
	headLength = uint(state.InputLength - state.InputOffset)
	if headLength == 0 {
		return nil
	}

	if state.Output == nil {
		state.OutputOffset += headLength + govarint.Length(2*headLength)
	} else {
		err = state.EncodeHead(state.InputLength)
		if err != nil {
			return err
		}
	}

	state.InputOffset = state.InputLength
	return nil
}

func EncodingLength(bitfield Bitfield) uint {
	var state State
	state = State{}
	state.Init(bitfield, nil, 0)
	state.RLE()
	return state.OutputOffset
}

func Decode(buffer []uint8, tempOffset *uint) ([]byte, error) {
	var err error
	var offset uint
	if tempOffset == nil {
		offset = 0
	} else {
		offset = *tempOffset
	}

	var bitfield []byte
	bitfield = make([]byte, offset)

	var ptr uint
	ptr = 0

	var next uint
	var repeat uint
	var length uint
	var decoder govarint.Decode
	for offset < uint(len(buffer)) {
		next, err = decoder.Decode(buffer, &offset)
		if err != nil {
			return nil, err
		}
		repeat = next & 1
		if repeat == 1 {
			length = (next - (next & 3)) / 4
		} else {
			length = next / 2
		}

		offset += decoder.Bytes

		if repeat != 0 {
			var tempBitfield []byte
			tempBitfield = make([]byte, length)
			if next&2 != 0 {
				// In this case we are dealing with repeated bytes in the original sequence.
				// Add the correct number of that byte. We can probably simplify this
				// at some point but for now we will just ignore ptr and ptr + len probably
				// not being needed due to how golang handles slices.
				for i := 0; uint(i) < length; i++ {
					tempBitfield[i] = 255
				}
			} else {
				for i := 0; uint(i) < length; i++ {
					tempBitfield[i] = 0
				}
			}
			// Then add the new tempBitfield to the output bitfield.
			bitfield = append(bitfield, tempBitfield...)
		} else {
			// Here it is not a repeating so just copy over the data from the buffer to the bitfield.
			bitfield = append(bitfield, buffer[offset:offset+length]...)
			offset += length
		}

		ptr += length
	}
	// Here we do the same thign as bitfield.fill(0, ptr) except since we started with the minimum
	// length slice for bitfield we will just add on zeros of the proper length to the end.
	var tempDiff int
	tempDiff = int(ptr) - len(bitfield)
	var tempBitfieldFinal []byte
	if tempDiff > 0 {
		tempBitfieldFinal = make([]byte, tempDiff)
		bitfield = append(bitfield, tempBitfieldFinal...)
	}

	// Ingoring decode.bytes until I can actually find where it is ever used.
	return bitfield, nil
}

func DecodingLength(buffer []uint8, offset uint) (uint, error) {
	var length uint
	length = 0
	var next uint
	var err error
	var decoder govarint.Decode
	decoder = govarint.Decode{}
	var repeat, slice uint
	for offset < uint(len(buffer)) {
		next, err = decoder.Decode(buffer, &offset)
		if err != nil {
			return 0, err
		}
		offset += decoder.Bytes

		repeat = next & 1

		if repeat > 0 {
			slice = next - (next&3)/4
		} else {
			slice = next / 2
		}

		length += slice
		if repeat == 0 {
			offset += slice
		}
	}

	if offset > uint(len(buffer)) {
		return 0, errors.New("invalid rle bitfield")
	}

	return length, nil
}

func (state *State) EncodeUpdate(i uint, length uint, bits uint8) {
	var headLength uint
	headLength = uint(i) - length - state.InputOffset
	var headCost uint
	headCost = govarint.Length(2*headLength) + headLength
	var enc uint
	enc = 4*length + 2 + 1 // length << 2 | bit << 1 | 1 ???? TODO: What is this supposed to do?
	var encCost uint
	encCost = headCost + govarint.Length(enc)
	var baseCost uint
	baseCost = govarint.Length(2*(i-state.InputOffset)) + i - state.InputOffset

	if encCost >= baseCost {
		return
	}

	if state.Output != nil {
		state.OutputOffset += encCost
		state.InputOffset = i
		return
	}

	if headLength != 0 {
		state.EncodeHead(i - length)
	}

	var encoder govarint.Encode
	encoder.Encode(enc, &state.Output, &state.OutputOffset)

	state.OutputOffset += encoder.Bytes
	state.InputOffset = i
}
