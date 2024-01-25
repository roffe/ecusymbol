package blowfish

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/roffe/ecusymbol/blowfish/ecb"
	"github.com/yeka/zip"

	"golang.org/x/crypto/blowfish" //lint:ignore SA1019 we know what we're doing
)

const zipPassword = "yii4uXXwser8"

var key = []byte{0x5A, 0x58, 0x7A, 0x33, 0x32, 0x64, 0x46, 0x64, 0x23, 0x24, 0x31, 0x73, 0x66, 0x46, 0x77, 0x45}

var tagDict = map[string][]byte{
	"TAG_FF_32": {0x67, 0x6F, 0x21, 0xA1, 0x8C, 0xF7, 0xE5, 0x7E, 0xBC, 0x57, 0x37, 0x70, 0x68, 0x0A, 0x33, 0x77},
	"TAG_FE_31": {0x79, 0xC5, 0x39, 0xC8, 0x86, 0x77, 0xD6, 0x14, 0x77, 0x66, 0x03, 0xE3, 0x4D, 0x61, 0xDE, 0x26},
	"TAG_FD_30": {0x19, 0x56, 0x41, 0x4A, 0xBF, 0xE4, 0x5B, 0xF1, 0x32, 0x96, 0x62, 0xC3, 0x42, 0x0D, 0x45, 0x88},
	"TAG_FC_29": {0xD4, 0x68, 0xF7, 0x57, 0x3B, 0xF8, 0x3A, 0xF5, 0x6D, 0x7D, 0x91, 0x78, 0x8C, 0x67, 0x42, 0x5E},
	"TAG_FF_28": {0x0E, 0xCE, 0xA7, 0x3F, 0xBF, 0x56, 0x7F, 0xA7, 0xF3, 0xB6, 0xAC, 0x7C, 0xA5, 0x4C, 0x22, 0xAC},
	"TAG_FF_27": {0x9E, 0x50, 0xEB, 0xF8, 0x64, 0xD5, 0x18, 0x7B, 0x02, 0xCA, 0x0C, 0x66, 0x22, 0x01, 0xDC, 0xD2},
	"TAG_FF_26": {0xCE, 0xD8, 0x77, 0x95, 0xB9, 0x88, 0x5B, 0x52, 0x8C, 0x33, 0x2C, 0xA2, 0x8C, 0x9C, 0xF7, 0x73},
	"TAG_FE_25": {0x98, 0x41, 0x8A, 0x90, 0xCF, 0x4F, 0x86, 0x6D, 0x07, 0x78, 0xD9, 0x48, 0x36, 0xE7, 0xD3, 0xC4},
	"TAG_FC_24": {0xD1, 0x7E, 0x6C, 0x2F, 0xF5, 0x62, 0x0C, 0xAE, 0x96, 0x64, 0x9D, 0x74, 0x18, 0xA0, 0xDF, 0x11},
	"TAG_FF_23": {0x7C, 0x68, 0x59, 0x17, 0x1F, 0x6D, 0xBF, 0xC7, 0xA5, 0x57, 0xCE, 0x73, 0x59, 0x54, 0xCF, 0xCD},
	"TAG_FE_22": {0xC5, 0x76, 0x3A, 0x15, 0xFD, 0x0F, 0xE9, 0xD7, 0x8D, 0x1D, 0xC4, 0x79, 0x4C, 0x2C, 0xF5, 0x0D},
	"TAG_FD_21": {0xEC, 0x5D, 0xFE, 0xAC, 0xA1, 0xB3, 0xDE, 0xA8, 0x1B, 0x3A, 0xA8, 0x6D, 0xA1, 0x70, 0xF2, 0x0D},
	"TAG_FC_20": {0x95, 0x9B, 0x0C, 0xDF, 0xE1, 0x9B, 0x9A, 0xC7, 0xB2, 0xCC, 0x5E, 0xB2, 0x8E, 0xA8, 0xBD, 0x03},
	"TAG_FE_19": {0x33, 0x5D, 0x3F, 0xE9, 0xA8, 0x26, 0xAB, 0x8B, 0x4F, 0xC9, 0x39, 0xC0, 0x52, 0x1B, 0x77, 0xB1},
	"TAG_FD_18": {0x05, 0x05, 0x42, 0x02, 0xCA, 0x0B, 0x6D, 0x08, 0x64, 0x78, 0xB4, 0xC6, 0xAF, 0x46, 0x79, 0x27},
	"TAG_FC_17": {0xAE, 0x79, 0x75, 0x19, 0x54, 0x5A, 0x98, 0xA1, 0x2D, 0xBE, 0xD9, 0xD3, 0xF9, 0x79, 0x8F, 0x39},
	"TAG16":     {0x69, 0xBE, 0x20, 0x0C, 0x83, 0x19, 0x53, 0xE3, 0xE8, 0x6A, 0x26, 0x58, 0xFE, 0x0A, 0xDC, 0xB2},
	"TAG15":     {0x87, 0xAF, 0x91, 0x8B, 0xAA, 0x55, 0xB0, 0x3B, 0xB6, 0x1E, 0xD0, 0xAC, 0x31, 0x87, 0x48, 0x6B},
	"TAG14":     {0x56, 0x05, 0xB2, 0x67, 0x44, 0x2A, 0xB4, 0x55, 0xE3, 0x57, 0x52, 0x50, 0xB6, 0xD1, 0x4C, 0x65},
	"TAG13":     {0x2C, 0x43, 0xF7, 0xE2, 0x3F, 0x3C, 0xBA, 0xD5, 0xF3, 0xBA, 0xB0, 0xDF, 0x4E, 0x7C, 0x2F, 0x50},
	"TAG12":     {0x9E, 0x89, 0xC5, 0x4C, 0x73, 0x73, 0x09, 0xB9, 0xBE, 0x21, 0x8F, 0xAE, 0x88, 0x09, 0x4E, 0x58},
	"TAG11":     {0x21, 0x36, 0xD7, 0x86, 0x34, 0xA8, 0xC7, 0x28, 0x39, 0x17, 0x3E, 0xB0, 0x17, 0x5E, 0x01, 0xE4},
	"TAG10":     {0x8C, 0x35, 0x9A, 0x7F, 0x07, 0x79, 0xF5, 0x38, 0xBA, 0x07, 0x91, 0xCC, 0x04, 0x57, 0x5B, 0x84},
	"TAG9":      {0x76, 0x2B, 0x4B, 0x9E, 0x40, 0x7A, 0x9A, 0x46, 0xCE, 0x7E, 0xC4, 0x62, 0xE4, 0x3C, 0xCD, 0xBC},
	"TAG8":      {0x3B, 0x06, 0xDD, 0xA3, 0x9D, 0x3A, 0x74, 0x82, 0x55, 0x55, 0x9C, 0xDE, 0x03, 0x47, 0x16, 0xC8},
	"TAG7":      {0xA0, 0x13, 0x38, 0x7E, 0xFD, 0x38, 0x51, 0xD8, 0x3F, 0xC3, 0xF9, 0x42, 0x1B, 0x27, 0x28, 0xE4},
	"TAG6":      {0xC9, 0xE2, 0x9D, 0xC6, 0xF2, 0x69, 0x3F, 0x78, 0x50, 0x32, 0x79, 0x4D, 0x89, 0x9D, 0x3A, 0x46},
	"TAG5":      {0xFF, 0x53, 0x9B, 0x28, 0xC8, 0xC4, 0x2B, 0x11, 0x7D, 0x93, 0x31, 0xE8, 0x81, 0x2C, 0xBA, 0x91},
	"TAG4":      {0x34, 0xAD, 0xB7, 0x2A, 0x48, 0xFC, 0x66, 0x85, 0x45, 0x19, 0x14, 0xB1, 0x5D, 0x40, 0x96, 0x82},
	"TAG3":      {0xAD, 0x33, 0x5F, 0x52, 0xFF, 0x95, 0x6F, 0xC1, 0x76, 0xBC, 0x40, 0x95, 0x73, 0x59, 0x09, 0xD9},
	"TAG2":      {0x1B, 0x60, 0x70, 0x54, 0x61, 0x40, 0xFF, 0x5D, 0x68, 0x0A, 0x4A, 0xC5, 0x90, 0xAC, 0x54, 0x16},
	"TAG1":      {0x84, 0x1F, 0xF5, 0x6D, 0x49, 0x5A, 0xE7, 0xF6, 0xD8, 0x70, 0x9A, 0x3B, 0x2F, 0x38, 0x1E, 0x5D},
}

func DecryptSymbolNames(data []byte) ([]string, error) {
	if err := checkHeader(data); err != nil {
		return nil, err
	}

	r := bytes.NewReader(data[8:])

	header, err := readData(r, 16)
	if err != nil {
		return nil, err
	}

	zipEncrypted, err := readData(r, len(data)-24)
	if err != nil {
		return nil, err
	}

	decryptedHeader, err := decrypt(header, key)
	if err != nil {
		return nil, err
	}

	zipBlowfishKey, found := tagDict[strings.ReplaceAll(string(decryptedHeader), "\x00", "")]
	if !found {
		return nil, errors.New("blowfish key not found")
	}

	zipBody, err := decrypt(zipEncrypted, zipBlowfishKey)
	if err != nil {
		return nil, err
	}

	return unzipSymbols(zipBody)
}

func checkHeader(data []byte) error {
	if !bytes.HasPrefix(data, []byte{0xF1, 0x1A, 0x06, 0x5B, 0xA2, 0x6B, 0xCC, 0x6F}) {
		return errors.New("invalid blowfish signature")
	}
	return nil
}

func unzipSymbols(zipBody []byte) ([]string, error) {
	zipReader, err := zip.NewReader(bytes.NewReader(zipBody), int64(len(zipBody)))
	if err != nil {
		return nil, err
	}
	for _, f := range zipReader.File {
		if f.IsEncrypted() {
			f.SetPassword(zipPassword)
		}
		fl, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open zip file %s: %w", f.Name, err)
		}
		defer fl.Close()
		buf, err := io.ReadAll(fl)
		if err != nil {
			return nil, fmt.Errorf("failed to read zip file %s: %w", f.Name, err)
		}
		return strings.Split(strings.TrimSuffix(string(buf), "\r\n"), "\r\n"), nil //lint:ignore SA4004 we know
	}
	return nil, errors.New("no symbols found")
}

func readData(r io.Reader, size int) ([]byte, error) {
	data := make([]byte, size)
	n, err := r.Read(data)
	if err != nil {
		return nil, fmt.Errorf("readData failed to read: %w", err)
	}
	if n != len(data) {
		return nil, fmt.Errorf("readData failed to read %d bytes, got %d", len(data), n)
	}
	return data, nil
}

func decrypt(ct, key []byte) ([]byte, error) {
	block, err := blowfish.NewCipher(key)
	if err != nil {
		return nil, err
	}
	mode := ecb.NewECBDecrypter(block, true)
	pt := make([]byte, len(ct))
	mode.CryptBlocks(pt, ct)
	return pt, nil
}