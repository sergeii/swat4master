package browsing

import (
	"encoding/binary"
	"errors"

	"github.com/sergeii/swat4master/pkg/binutils"
	"github.com/sergeii/swat4master/pkg/gamespy/browsing/query/filter"
)

const (
	MinRequestPayloadLength  = 26
	MaxAllowedNumberOfFields = 20
)

type Request struct {
	Filters   string
	Fields    []string
	Challenge [8]byte
	unparsed  []byte
}

var (
	ErrInvalidRequestFormat   = errors.New("invalid payload format")
	ErrNoFieldsRequested      = errors.New("no fields are requested")
	ErrTooManyFieldsRequested = errors.New("too many fields are requested")
)

func NewRequest(data []byte) (*Request, error) {
	// nolint:lll
	// \x00swat4\x00swat4\x00q!8Gp9Rigametype='CO-OP' and gamever='1.1'\x00\hostname\...\password\gamever\x00\x00\x00\x00\x00
	// \x00swat4xp1\x00swat4xp1\x00|OQ0ERkV\x00\hostname\numplayers\...\statsenabled\x00\x00\x00\x00\x00
	// \x00swat4\x00swat4\x00[)HccTB;numplayers!=maxplayers and password=0 and gamever='1.1' and gamevariant='SWAT 4'\x00\hostname\...\gamever\x00\x00\x00\x00\x00 nolint:lll

	// the first 2 bytes denote the actual payload length
	// which also should not be lesser than the minimal accepted length
	if len(data) < 2 {
		return nil, ErrInvalidRequestFormat
	}

	// correctly formatted payload without filters or fields should be at least 26 bytes long
	// but in reality much longer because we require fields to be present
	dataLen := int(binary.BigEndian.Uint16(data[:2]))
	if dataLen < MinRequestPayloadLength || dataLen > len(data) {
		return nil, ErrInvalidRequestFormat
	}
	req := Request{
		// skip the following 7 bytes (excluding the first two that encode the payload length)
		// that contain metadata we don't need to look into
		unparsed: data[9:dataLen],
	}
	if err := req.parse(); err != nil {
		return nil, err
	}
	return &req, nil
}

func (req *Request) parse() error {
	// consume 2 equal strings with game identifier (such as swat4 or swat4xp1)
	// because they don't serve any purpose for us, just bin them
	for i := 0; i < 2; i++ {
		_, rem := binutils.ConsumeCString(req.unparsed)
		if rem == nil {
			return ErrInvalidRequestFormat
		}
		req.unparsed = rem
	}
	if err := req.parseChallenge(); err != nil {
		return err
	}
	if err := req.parseFilters(); err != nil {
		return err
	}
	if err := req.parseFields(); err != nil {
		return err
	}
	return req.validateOptionsMask()
}

func (req *Request) parseChallenge() error {
	// challenge key is 8 bytes long
	// It has fixed size, so it's not a null delimited
	// Check that the remaining slice is at least 8 bytes long
	if len(req.unparsed) < 8 {
		return ErrInvalidRequestFormat
	}
	copy(req.Challenge[:], req.unparsed[:8])
	req.unparsed = req.unparsed[8:]
	return nil
}

func (req *Request) parseFilters() error {
	// filters such as "gametype='CO-OP'" are optional,
	// so it's entirely correct to expect an empty slice here
	// Still, it's null terminated
	filters, rem := binutils.ConsumeCString(req.unparsed)
	if rem == nil {
		return ErrInvalidRequestFormat
	}
	req.Filters = string(filters)
	req.unparsed = rem
	return nil
}

func (req *Request) parseFields() error {
	var fieldNameBin []byte
	// field list is mandatory
	// It starts with a backslash followed by a list of field names each delimited also by a backslash
	fieldsBinString, rem := binutils.ConsumeCString(req.unparsed)
	if rem == nil || len(fieldsBinString) < 1 || fieldsBinString[0] != '\\' {
		return ErrInvalidRequestFormat
	}

	fieldsUnparsed := fieldsBinString[1:] // skip the leading \
	fields := make([]string, 0, 1)        // make a room for at least 1 field
	for len(fieldsUnparsed) > 0 {
		fieldNameBin, fieldsUnparsed = binutils.ConsumeString(fieldsUnparsed, '\\')
		field := string(fieldNameBin)
		if !filter.IsQueryField(field) {
			continue
		}
		fields = append(fields, field)
		if len(fields) > MaxAllowedNumberOfFields {
			return ErrTooManyFieldsRequested
		}
	}
	if len(fields) == 0 {
		return ErrNoFieldsRequested
	}
	req.Fields = fields
	req.unparsed = rem
	return nil
}

func (req *Request) validateOptionsMask() error {
	// the remaining data in the slice should be 4 bytes long
	if len(req.unparsed) != 4 {
		return ErrInvalidRequestFormat
	}
	// Options value should be 0 (plain server list) or 1 (server list with fields, such as \hostname etc)
	options := binary.BigEndian.Uint32(req.unparsed)
	if options != 0 && options != 1 {
		return ErrInvalidRequestFormat
	}
	req.unparsed = nil
	return nil
}
