package iso8583

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	// ASCII is ASCII encoding
	ASCII = iota
	// BCD is "left-aligned" BCD
	BCD
	// rBCD is "right-aligned" BCD with odd length (for ex. "643" as [6 67] == "0643"), only for Numeric, Llnumeric and Lllnumeric fields
	rBCD
)

const (
	ERR_INVALID_ENCODER        string = "invalid encoder"
	ERR_INVALID_LENGTH_ENCODER string = "invalid length encoder"
	ERR_INVALID_LENGTH_HEAD    string = "invalid length head"
	ERR_MISSING_LENGTH         string = "missing length"
	ERR_VALUE_TOO_LONG         string = "length of value is longer than definition; type=%s, def_len=%d, len=%d"
	ERR_BAD_RAW                string = "bad raw data"
	ERR_PARSE_LENGTH_FAILED    string = "parse length head failed"
)

// Iso8583Type interface for ISO 8583 fields
type Iso8583Type interface {
	// Byte representation of current field.
	Bytes(encoder, lenEncoder, length int) ([]byte, error)

	// Load unmarshal byte value into Iso8583Type according to the
	// specific arguments. It returns the number of bytes actually read.
	Load(raw []byte, encoder, lenEncoder, length int) (int, error)

	// IsEmpty check is field empty
	IsEmpty() bool
}

// A Numeric contains numeric value only in fix length. It holds numeric
// value as a string. Supportted encoder are ascii, bcd and rbcd. Length is
// required for marshalling and unmarshalling.
type Numeric struct {
	Value string
}

// NewNumeric create new Numeric field
func NewNumeric(val string) *Numeric {
	return &Numeric{val}
}

// IsEmpty check Numeric field for empty value
func (n *Numeric) IsEmpty() bool {
	return utf8.RuneCountInString(n.Value) == 0
}

// Bytes encode Numeric field to bytes
func (n *Numeric) Bytes(encoder, lenEncoder, length int) ([]byte, error) {
	val := []byte(n.Value)
	if length == -1 {
		return nil, errors.New(ERR_MISSING_LENGTH)
	}
	// if encoder == rBCD then length can be, for example, 3,
	// but value can be, for example, "0631" (after decode from rBCD, because BCD use 1 byte for 2 digits),
	// and we can encode it only if first digit == 0
	if (encoder == rBCD) &&
		utf8.RuneCount(val) == (length+1) &&
		(string(val[0:1]) == "0") {
		// Cut value to length
		val = val[1:utf8.RuneCount(val)]
	}

	if utf8.RuneCount(val) > length {
		return nil, errors.New(fmt.Sprintf(ERR_VALUE_TOO_LONG, "Numeric", length, utf8.RuneCount(val)))
	}
	if utf8.RuneCount(val) < length {
		val = append([]byte(strings.Repeat("0", length-utf8.RuneCount(val))), val...)
	}
	switch encoder {
	case BCD:
		return lbcd(val), nil
	case rBCD:
		return rbcd(val), nil
	case ASCII:
		return val, nil
	default:
		return nil, errors.New(ERR_INVALID_ENCODER)
	}
}

// Load decode Numeric field from bytes
func (n *Numeric) Load(raw []byte, encoder, lenEncoder, length int) (int, error) {
	if length == -1 {
		return 0, errors.New(ERR_MISSING_LENGTH)
	}
	switch encoder {
	case BCD:
		l := (length + 1) / 2
		if utf8.RuneCount(raw) < l {
			return 0, errors.New(ERR_BAD_RAW)
		}
		n.Value = string(bcdl2Ascii(raw[:l], length))
		return l, nil
	case rBCD:
		l := (length + 1) / 2
		if utf8.RuneCount(raw) < l {
			return 0, errors.New(ERR_BAD_RAW)
		}
		n.Value = string(bcdr2Ascii(raw[0:l], length))
		return l, nil
	case ASCII:
		if utf8.RuneCount(raw) < length {
			return 0, errors.New(ERR_BAD_RAW)
		}
		n.Value = string(raw[:length])
		return length, nil
	default:
		return 0, errors.New(ERR_INVALID_ENCODER)
	}
}

// An Alphanumeric contains alphanumeric value in fix length. The only
// supportted encoder is ascii. Length is required for marshalling and
// unmarshalling.
type Alphanumeric struct {
	Value string
}

// NewAlphanumeric create new Alphanumeric field
func NewAlphanumeric(val string) *Alphanumeric {
	return &Alphanumeric{Value: val}
}

// IsEmpty check Alphanumeric field for empty value
func (a *Alphanumeric) IsEmpty() bool {
	return utf8.RuneCountInString(a.Value) == 0
}

// Bytes encode Alphanumeric field to bytes
func (a *Alphanumeric) Bytes(encoder, lenEncoder, length int) ([]byte, error) {
	val := []byte(a.Value)
	if length == -1 {
		return nil, errors.New(ERR_MISSING_LENGTH)
	}
	if utf8.RuneCount(val) > length {
		return nil, errors.New(fmt.Sprintf(ERR_VALUE_TOO_LONG, "Alphanumeric", length, utf8.RuneCount(val)))
	}
	if utf8.RuneCount(val) < length {
		val = append([]byte(strings.Repeat(" ", length-utf8.RuneCount(val))), val...)
	}
	return val, nil
}

// Load decode Alphanumeric field from bytes
func (a *Alphanumeric) Load(raw []byte, encoder, lenEncoder, length int) (int, error) {
	if length == -1 {
		return 0, errors.New(ERR_MISSING_LENGTH)
	}
	if utf8.RuneCount(raw) < length {
		return 0, errors.New(ERR_BAD_RAW)
	}
	a.Value = string(raw[:length])
	return length, nil
}

// Binary contains binary value
type Binary struct {
	Value  []byte
	FixLen int
}

// NewBinary create new Binary field
func NewBinary(d []byte) *Binary {
	return &Binary{d, -1}
}

// IsEmpty check Binary field for empty value
func (b *Binary) IsEmpty() bool {
	return utf8.RuneCount(b.Value) == 0
}

// Bytes encode Binary field to bytes
func (b *Binary) Bytes(encoder, lenEncoder, l int) ([]byte, error) {
	length := l
	if b.FixLen != -1 {
		length = b.FixLen
	}
	if length == -1 {
		return nil, errors.New(ERR_MISSING_LENGTH)
	}
	if utf8.RuneCount(b.Value) > length {
		return nil, errors.New(fmt.Sprintf(ERR_VALUE_TOO_LONG, "Binary", length, utf8.RuneCount(b.Value)))
	}
	if utf8.RuneCount(b.Value) < length {
		return append(b.Value, make([]byte, length-utf8.RuneCount(b.Value))...), nil
	}
	return b.Value, nil
}

// Load decode Binary field from bytes
func (b *Binary) Load(raw []byte, encoder, lenEncoder, length int) (int, error) {
	if length == -1 {
		return 0, errors.New(ERR_MISSING_LENGTH)
	}
	if utf8.RuneCount(raw) < length {
		return 0, errors.New(ERR_BAD_RAW)
	}
	b.Value = raw[:length]
	b.FixLen = length
	return length, nil
}

// Llvar contains bytes in non-fixed length field, first 2 symbols of field contains length
type Llvar struct {
	Value []byte
}

// NewLlvar create new Llvar field
func NewLlvar(val []byte) *Llvar {
	return &Llvar{val}
}

// IsEmpty check Llvar field for empty value
func (l *Llvar) IsEmpty() bool {
	return utf8.RuneCount(l.Value) == 0
}

// Bytes encode Llvar field to bytes
func (l *Llvar) Bytes(encoder, lenEncoder, length int) ([]byte, error) {
	if length != -1 && utf8.RuneCount(l.Value) > length {
		return nil, errors.New(fmt.Sprintf(ERR_VALUE_TOO_LONG, "Llvar", length, utf8.RuneCount(l.Value)))
	}
	if encoder != ASCII {
		return nil, errors.New(ERR_INVALID_ENCODER)
	}

	lenStr := fmt.Sprintf("%02d", utf8.RuneCount(l.Value))
	contentLen := []byte(lenStr)
	var lenVal []byte
	switch lenEncoder {
	case ASCII:
		lenVal = contentLen
		if utf8.RuneCount(lenVal) > 2 {
			return nil, errors.New(ERR_INVALID_LENGTH_HEAD)
		}
	case rBCD:
		fallthrough
	case BCD:
		lenVal = rbcd(contentLen)
		if utf8.RuneCount(lenVal) > 1 {
			return nil, errors.New(ERR_INVALID_LENGTH_HEAD)
		}
	default:
		return nil, errors.New(ERR_INVALID_LENGTH_ENCODER)
	}
	return append(lenVal, l.Value...), nil
}

// Load decode Llvar field from bytes
func (l *Llvar) Load(raw []byte, encoder, lenEncoder, length int) (read int, err error) {
	// parse length head:
	var contentLen int
	switch lenEncoder {
	case ASCII:
		read = 2
		contentLen, err = strconv.Atoi(string(raw[:read]))
		if err != nil {
			return 0, errors.New(ERR_PARSE_LENGTH_FAILED + ": " + string(raw[:2]))
		}
	case rBCD:
		fallthrough
	case BCD:
		read = 1
		contentLen, err = strconv.Atoi(string(bcdr2Ascii(raw[:read], 2)))
		if err != nil {
			return 0, errors.New(ERR_PARSE_LENGTH_FAILED + ": " + string(raw[0]))
		}
	default:
		return 0, errors.New(ERR_INVALID_LENGTH_ENCODER)
	}
	if utf8.RuneCount(raw) < (read + contentLen) {
		return 0, errors.New(ERR_BAD_RAW)
	}
	// parse body:
	l.Value = raw[read : read+contentLen]
	read += contentLen
	if encoder != ASCII {
		return 0, errors.New(ERR_INVALID_ENCODER)
	}

	return read, nil
}

// A Llnumeric contains numeric value only in non-fix length, contains length in first 2 symbols. It holds numeric
// value as a string. Supportted encoder are ascii, bcd and rbcd. Length is
// required for marshalling and unmarshalling.
type Llnumeric struct {
	Value string
}

// NewLlnumeric create new Llnumeric field
func NewLlnumeric(val string) *Llnumeric {
	return &Llnumeric{val}
}

// IsEmpty check Llnumeric field for empty value
func (l *Llnumeric) IsEmpty() bool {
	return utf8.RuneCountInString(l.Value) == 0
}

// Bytes encode Llnumeric field to bytes
func (l *Llnumeric) Bytes(encoder, lenEncoder, length int) ([]byte, error) {
	raw := []byte(l.Value)
	if length != -1 && utf8.RuneCount(raw) > length {
		return nil, errors.New(fmt.Sprintf(ERR_VALUE_TOO_LONG, "Llnumeric", length, utf8.RuneCount(raw)))
	}

	val := raw
	switch encoder {
	case ASCII:
	case BCD:
		val = lbcd(raw)
	case rBCD:
		val = rbcd(raw)
	default:
		return nil, errors.New(ERR_INVALID_ENCODER)
	}

	lenStr := fmt.Sprintf("%02d", utf8.RuneCount(raw)) // length of digital characters
	contentLen := []byte(lenStr)
	var lenVal []byte
	switch lenEncoder {
	case ASCII:
		lenVal = contentLen
		if utf8.RuneCount(lenVal) > 2 {
			return nil, errors.New(ERR_INVALID_LENGTH_HEAD)
		}
	case rBCD:
		fallthrough
	case BCD:
		lenVal = rbcd(contentLen)
		if utf8.RuneCount(lenVal) > 1 || utf8.RuneCount(contentLen) > 3 {
			return nil, errors.New(ERR_INVALID_LENGTH_HEAD)
		}
	default:
		return nil, errors.New(ERR_INVALID_LENGTH_ENCODER)
	}
	return append(lenVal, val...), nil
}

// Load decode Llnumeric field from bytes
func (l *Llnumeric) Load(raw []byte, encoder, lenEncoder, length int) (read int, err error) {
	// parse length head:
	var contentLen int
	switch lenEncoder {
	case ASCII:
		read = 2
		contentLen, err = strconv.Atoi(string(raw[:read]))
		if err != nil {
			return 0, errors.New(ERR_PARSE_LENGTH_FAILED + ": " + string(raw[:2]))
		}
	case rBCD:
		fallthrough
	case BCD:
		read = 1
		contentLen, err = strconv.Atoi(string(bcdr2Ascii(raw[:read], 2)))
		if err != nil {
			return 0, errors.New(ERR_PARSE_LENGTH_FAILED + ": " + string(raw[0]))
		}
	default:
		return 0, errors.New(ERR_INVALID_LENGTH_ENCODER)
	}

	// parse body:
	switch encoder {
	case ASCII:
		if utf8.RuneCount(raw) < (read + contentLen) {
			return 0, errors.New(ERR_BAD_RAW)
		}
		l.Value = string(raw[read : read+contentLen])
		read += contentLen
	case rBCD:
		fallthrough
	case BCD:
		bcdLen := (contentLen + 1) / 2
		if utf8.RuneCount(raw) < (read + bcdLen) {
			return 0, errors.New(ERR_BAD_RAW)
		}
		l.Value = string(bcdl2Ascii(raw[read:read+bcdLen], contentLen))
		read += bcdLen
	default:
		return 0, errors.New(ERR_INVALID_ENCODER)
	}
	return read, nil
}

// Lllvar contains bytes in non-fixed length field, first 3 symbols of field contains length
type Lllvar struct {
	Value []byte
}

// NewLllvar create new Lllvar field
func NewLllvar(val []byte) *Lllvar {
	return &Lllvar{val}
}

// IsEmpty check Lllvar field for empty value
func (l *Lllvar) IsEmpty() bool {
	return utf8.RuneCount(l.Value) == 0
}

// Bytes encode Lllvar field to bytes
func (l *Lllvar) Bytes(encoder, lenEncoder, length int) ([]byte, error) {
	if length != -1 && utf8.RuneCount(l.Value) > length {
		return nil, errors.New(fmt.Sprintf(ERR_VALUE_TOO_LONG, "Lllvar", length, utf8.RuneCount(l.Value)))
	}
	if encoder != ASCII {
		return nil, errors.New(ERR_INVALID_ENCODER)
	}

	lenStr := fmt.Sprintf("%03d", utf8.RuneCount(l.Value))
	contentLen := []byte(lenStr)
	var lenVal []byte
	switch lenEncoder {
	case ASCII:
		lenVal = contentLen
		if utf8.RuneCount(lenVal) > 3 {
			return nil, errors.New(ERR_INVALID_LENGTH_HEAD)
		}
	case rBCD:
		fallthrough
	case BCD:
		lenVal = rbcd(contentLen)
		if utf8.RuneCount(lenVal) > 2 || utf8.RuneCount(contentLen) > 3 {
			return nil, errors.New(ERR_INVALID_LENGTH_HEAD)
		}
	default:
		return nil, errors.New(ERR_INVALID_LENGTH_ENCODER)
	}
	return append(lenVal, l.Value...), nil
}

// Load decode Lllvar field from bytes
func (l *Lllvar) Load(raw []byte, encoder, lenEncoder, length int) (read int, err error) {
	// parse length head:
	var contentLen int
	switch lenEncoder {
	case ASCII:
		read = 3
		contentLen, err = strconv.Atoi(string(raw[:read]))
		if err != nil {
			return 0, errors.New(ERR_PARSE_LENGTH_FAILED + ": " + string(raw[:3]))
		}
	case rBCD:
		fallthrough
	case BCD:
		read = 2
		contentLen, err = strconv.Atoi(string(bcdr2Ascii(raw[:read], 3)))
		if err != nil {
			return 0, errors.New(ERR_PARSE_LENGTH_FAILED + ": " + string(raw[:2]))
		}
	default:
		return 0, errors.New(ERR_INVALID_LENGTH_ENCODER)
	}
	if utf8.RuneCount(raw) < (read + contentLen) {
		return 0, errors.New(ERR_BAD_RAW)
	}
	// parse body:
	l.Value = raw[read : read+contentLen]
	read += contentLen
	if encoder != ASCII {
		return 0, errors.New(ERR_INVALID_ENCODER)
	}

	return read, nil
}

// A Lllnumeric contains numeric value only in non-fix length, contains length in first 3 symbols. It holds numeric
// value as a string. Supportted encoder are ascii, bcd and rbcd. Length is
// required for marshalling and unmarshalling.
type Lllnumeric struct {
	Value string
}

// NewLllnumeric create new Lllnumeric field
func NewLllnumeric(val string) *Lllnumeric {
	return &Lllnumeric{val}
}

// IsEmpty check Lllnumeric field for empty value
func (l *Lllnumeric) IsEmpty() bool {
	return utf8.RuneCountInString(l.Value) == 0
}

// Bytes encode Lllnumeric field to bytes
func (l *Lllnumeric) Bytes(encoder, lenEncoder, length int) ([]byte, error) {
	raw := []byte(l.Value)
	if length != -1 && utf8.RuneCount(raw) > length {
		return nil, errors.New(fmt.Sprintf(ERR_VALUE_TOO_LONG, "Lllnumeric", length, utf8.RuneCount(raw)))
	}

	val := raw
	switch encoder {
	case ASCII:
	case BCD:
		val = lbcd(raw)
	case rBCD:
		val = rbcd(raw)
	default:
		return nil, errors.New(ERR_INVALID_ENCODER)
	}

	lenStr := fmt.Sprintf("%03d", utf8.RuneCount(raw)) // length of digital characters
	contentLen := []byte(lenStr)
	var lenVal []byte
	switch lenEncoder {
	case ASCII:
		lenVal = contentLen
		if utf8.RuneCount(lenVal) > 3 {
			return nil, errors.New(ERR_INVALID_LENGTH_HEAD)
		}
	case rBCD:
		fallthrough
	case BCD:
		lenVal = rbcd(contentLen)
		if utf8.RuneCount(lenVal) > 2 || utf8.RuneCount(contentLen) > 3 {
			return nil, errors.New(ERR_INVALID_LENGTH_HEAD)
		}
	default:
		return nil, errors.New(ERR_INVALID_LENGTH_ENCODER)
	}
	return append(lenVal, val...), nil
}

// Load decode Lllnumeric field from bytes
func (l *Lllnumeric) Load(raw []byte, encoder, lenEncoder, length int) (read int, err error) {
	// parse length head:
	var contentLen int
	switch lenEncoder {
	case ASCII:
		read = 3
		contentLen, err = strconv.Atoi(string(raw[:read]))
		if err != nil {
			return 0, errors.New(ERR_PARSE_LENGTH_FAILED + ": " + string(raw[:3]))
		}
	case rBCD:
		fallthrough
	case BCD:
		read = 2
		contentLen, err = strconv.Atoi(string(bcdr2Ascii(raw[:read], 2)))
		if err != nil {
			return 0, errors.New(ERR_PARSE_LENGTH_FAILED + ": " + string(raw[:2]))
		}
	default:
		return 0, errors.New(ERR_INVALID_LENGTH_ENCODER)
	}

	// parse body:
	switch encoder {
	case ASCII:
		if utf8.RuneCount(raw) < (read + contentLen) {
			return 0, errors.New(ERR_BAD_RAW)
		}
		l.Value = string(raw[read : read+contentLen])
		read += contentLen
	case rBCD:
		fallthrough
	case BCD:
		bcdLen := (contentLen + 1) / 2
		if utf8.RuneCount(raw) < (read + bcdLen) {
			return 0, errors.New(ERR_BAD_RAW)
		}
		l.Value = string(bcdl2Ascii(raw[read:read+bcdLen], contentLen))
		read += bcdLen
	default:
		return 0, errors.New(ERR_INVALID_ENCODER)
	}
	return read, nil
}
