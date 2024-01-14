package symbol

import (
	_ "embed"
	"encoding/xml"
	"fmt"
	"strings"
)

//go:embed EU0AF01C.xml
var EU0AF01C_xml []byte

//go:embed EU0AF01O.xml
var EU0AF01O_xml []byte

type XMLSymbol struct {
	Text         string `xml:",chardata"`
	SYMBOLNAME   string `xml:"SYMBOLNAME"`
	SYMBOLNUMBER int    `xml:"SYMBOLNUMBER"`
	FLASHADDRESS string `xml:"FLASHADDRESS"`
	DESCRIPTION  string `xml:"DESCRIPTION"`
}

type DocumentElement struct {
	XMLName xml.Name    `xml:"DocumentElement"`
	Text    string      `xml:",chardata"`
	Symbols []XMLSymbol `xml:",any"`
}

var xmlMap map[string][]byte = map[string][]byte{
	"EU0AF01C": EU0AF01C_xml,
	"EU0BF01C": EU0AF01C_xml,
	"EU0CF01C": EU0AF01C_xml,
	"EU0AF01O": EU0AF01O_xml,
	"EU0BF01O": EU0AF01O_xml,
	"EU0CF01O": EU0AF01O_xml,
}

func xml2map(name string) (map[int]string, error) {
	xmlBytes, ok := xmlMap[strings.ToUpper(name)]
	if !ok {
		return nil, fmt.Errorf("unknown xml: %s", name)
	}

	var symbols DocumentElement
	if err := xml.Unmarshal(xmlBytes, &symbols); err != nil {
		return nil, err
	}

	results := make(map[int]string)
	for _, s := range symbols.Symbols {
		//fmt.Fprintf(f, "%d %s %s\n", s.SYMBOLNUMBER, s.DESCRIPTION, s.SYMBOLNAME)
		results[s.SYMBOLNUMBER] = s.DESCRIPTION
	}
	return results, nil
}
