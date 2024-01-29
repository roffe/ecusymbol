package symbol

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

type Patch struct {
	Operations []Operation
}

func (p *Patch) Apply(sc SymbolCollection) error {
	for _, op := range p.Operations {
		if err := op.Apply(sc); err != nil {
			return err
		}
	}
	return nil
}

type Operation interface {
	Apply(SymbolCollection) error
}

func ReadTuningPackageFile(filename string) (*Patch, error) {
	patch, err := os.ReadFile(filename)
	if err != nil {
		log.Fatal(err)
	}
	return ReadTuningPackage(patch)
}

func ReadTuningPackage(data []byte) (*Patch, error) {
	r := bytes.NewReader(data)
	var ops []Operation
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		switch {
		case strings.HasPrefix(line, "binaction="):
			log.Println("binaction not implemented")

		case strings.HasPrefix(line, "searchreplace="):
			log.Println("searchreplace not implemented")

		case strings.HasPrefix(line, "symbol="):
			name := strings.TrimPrefix(line, "symbol=")
			ops = append(ops, &SymbolUpdate{
				SymbolName: name,
			})
			// log.Printf("symbol=%s\n", name)
		case strings.HasPrefix(line, "length="):
			if len(ops) > 0 {
				if t, ok := ops[len(ops)-1].(*SymbolUpdate); ok {
					num, err := strconv.Atoi(strings.TrimPrefix(line, "length="))
					if err != nil {
						return nil, err
					}
					t.Length = num
					// log.Printf("length=%d\n", num)
				} else {
					return nil, fmt.Errorf("length= not allowed here")
				}
			}
		case strings.HasPrefix(line, "data="):
			if len(ops) > 0 {
				if t, ok := ops[len(ops)-1].(*SymbolUpdate); ok {
					data, err := hex.DecodeString(
						strings.ReplaceAll(
							strings.TrimPrefix(line, "data="), ",", "",
						),
					)
					if err != nil {
						return nil, err
					}
					t.Data = data
					// log.Printf("data=%s\n", hex.EncodeToString(data))
				} else {
					return nil, fmt.Errorf("data= not allowed here")
				}
			}
		}

	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &Patch{
		Operations: ops,
	}, nil
}

type Binaction struct {
}

type SearchReplace struct {
}

type SymbolUpdate struct {
	SymbolName string
	Length     int
	Data       []byte
}

func (su *SymbolUpdate) Apply(sc SymbolCollection) error {
	sym := sc.GetByName(su.SymbolName)
	if sym == nil {
		return fmt.Errorf("symbol %s not found", su.SymbolName)
	}
	if len(su.Data) != su.Length {
		return fmt.Errorf("length of data (%d) does not match length (%d)", len(su.Data), su.Length)
	}
	if err := sym.SetData(su.Data); err != nil {
		return err
	}
	log.Printf("update symbol %s, Length: %d", su.SymbolName, su.Length)
	return nil
}
