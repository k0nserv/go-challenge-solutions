package drum

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

var magicNumber = [...]byte{0x53, 0x50, 0x4c, 0x49, 0x43, 0x45}

const corruptFileError = "The file could not be read because it was corrupted"

// DecodeFile decodes the drum machine file found at the provided path
// and returns a pointer to a parsed pattern which is the entry point to the
// rest of the data.
//
// File format:
// 6 bytes. Magic number SPLICE
// 8 bytes remaining bytes in file
// 32 Bytes, encoding version string
// 4 Bytes, IEEE754 float tempo
//
// For each track
// 4 Byte track index
// 1 Byte length of Track name. e.g 0x04 for Kick
// n bytes for track name, length from previous byte
// 16 consecuvite bytes for each step
func DecodeFile(path string) (*Pattern, error) {
	file, err := os.Open(path)

	defer func() {
		err := file.Close()
		if err != nil {
			panic(err)
		}
	}()

	if err != nil {
		return nil, err
	}

	err = verifyMagicNumber(file)

	if err != nil {
		return nil, err
	}

	var remainingBytes uint64
	err = binary.Read(file, binary.BigEndian, &remainingBytes)
	if err != nil {
		return nil, errors.New(corruptFileError)
	}

	version, err := readVersionString(file, binary.LittleEndian)
	if err != nil {
		return nil, errors.New(corruptFileError)
	}
	remainingBytes -= 32

	var tempo float32

	err = binary.Read(file, binary.LittleEndian, &tempo)
	if err != nil {
		return nil, errors.New(corruptFileError)
	}
	remainingBytes -= 4

	var tracks = make([]Track, 0)

	err = nil
	for remainingBytes > 0 {
		track, err := readTrack(file, binary.LittleEndian)

		if err != nil && err != io.EOF {
			return nil, err
		} else if err == io.EOF {
			break
		}

		tracks = append(tracks, *track)
		remainingBytes -= uint64(track.sizeInBytes)
	}

	p := Pattern{
		version: *version,
		tempo:   tempo,
		tracks:  tracks,
	}
	return &p, nil
}

// Verify the magic number SPLICE in the file
func verifyMagicNumber(file *os.File) error {
	buffer := make([]byte, 6)
	count, err := file.Read(buffer)

	if count != 6 || err != nil {
		return err
	}

	if bytes.Equal(buffer, magicNumber[:]) {
		return nil
	}

	return errors.New("Unknown file format, does not match magic number")
}

func readVersionString(file *os.File, byteOrder binary.ByteOrder) (*string, error) {
	return readString(file, byteOrder, 32, true)
}

func readString(file *os.File, byteOrder binary.ByteOrder, length uint, nullTerminated bool) (*string, error) {
	buffer := make([]byte, length)
	err := binary.Read(file, byteOrder, buffer)

	if err != nil {
		return nil, err
	}

	var result string
	if nullTerminated {
		zeroIndex := bytes.Index(buffer, []byte{0})
		result = string(buffer[:zeroIndex])
	} else {
		result = string(buffer[:length])
	}
	return &result, nil

}

// For each track
// 4 Byte track index
// 1 Byte length of Track name. e.g 0x04 for Kick
// n bytes for track name, length from previous byte
// 16 consecuvite bytes for each step
func readTrack(file *os.File, byteOrder binary.ByteOrder) (*Track, error) {
	var sizeInBytes uint
	var index uint32
	err := binary.Read(file, byteOrder, &index)
	if err != nil {
		return nil, err
	}
	sizeInBytes += 4

	var nameLength uint8
	err = binary.Read(file, byteOrder, &nameLength)
	if err != nil {
		return nil, err
	}
	sizeInBytes++

	name, err := readString(file, byteOrder, uint(nameLength), false)
	if err != nil {
		return nil, err
	}
	sizeInBytes += uint(nameLength)

	var steps [16]uint8
	err = binary.Read(file, byteOrder, &steps)
	if err != nil {
		return nil, err
	}
	sizeInBytes += 16

	var stepsBool [16]bool

	for index, value := range steps {
		stepsBool[index] = value != 0
	}

	return &Track{
		index:       index,
		name:        *name,
		steps:       stepsBool,
		sizeInBytes: uint(sizeInBytes),
	}, nil
}

// Track represents a single track in the
// pattern
type Track struct {
	index       uint32
	name        string
	steps       [16]bool
	sizeInBytes uint
}

func (t *Track) String() string {
	var result string
	result += fmt.Sprintf("(%d) %s\t", t.index, t.name)

	for index, value := range t.steps {
		if index%4 == 0 {
			result += "|"
		}

		if value {
			result += "x"
		} else {
			result += "-"
		}
	}
	result += "|"

	return result
}

// Pattern is the high level representation of the
// drum pattern contained in a .splice file.
type Pattern struct {
	version string
	tempo   float32
	tracks  []Track
}

func (p *Pattern) String() string {
	var result string
	result += fmt.Sprintf("Saved with HW Version: %s\n", p.version)
	result += fmt.Sprintf("Tempo: %g\n", p.tempo)

	for _, element := range p.tracks {
		result += fmt.Sprintf("%s\n", element.String())
	}

	return result
}
