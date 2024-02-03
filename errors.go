package symbol

import "errors"

var (
	ErrSymbolTableNotFound        = errors.New("no symbol table found")
	ErrInvalidSymbolTableHeader   = errors.New("invalid symbol table header")
	ErrEndOfSymbolTableNotFound   = errors.New("end of symbol table not found")
	ErrAddressTableOffsetNotFound = errors.New("address table offset not found")
	ErrInvalidLength              = errors.New("file has incorrect length")
	ErrToLarge                    = errors.New("file is too large")
	ErrMagicBytesNotFound         = errors.New("magic bytes not found")
	ErrOffsetOutOfRange           = errors.New("offset out of range")
	ErrDataIsEmpty                = errors.New("data is empty")
	ErrVersionNotFound            = errors.New("version not found")
	ErrAddressOutOfRange          = errors.New("address out of range")
	ErrInvalidFile                = errors.New("invalid file")
)
