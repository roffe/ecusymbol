package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/roffe/ecusymbol/as2"
)

var (
	as2file    string
	symbolName string
)

func init() {
	log.SetFlags(0)
	// parse os args 1 as the as2 file to load, and 2 as the symbol to query.
	if len(os.Args) > 1 {
		as2file = os.Args[1]
	} else {
		log.Fatal("Usage: as2 <as2file> [symbol]\n  symbol omitted: dump every symbol\n  symbol given:   substring match, case-insensitive")
	}
	if len(os.Args) > 2 {
		symbolName = os.Args[2]
	}
}

func main() {
	f, err := as2.Load(as2file)
	if err != nil {
		log.Fatal(err)
	}

	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	var shown int
	for _, s := range f.Symbols {
		if !match(s.Name, symbolName) {
			continue
		}
		printSymbol(out, s)
		shown++
	}

	fmt.Fprintf(out, "\n%d of %d symbols", shown, len(f.Symbols))
	if symbolName != "" {
		fmt.Fprintf(out, " matching %q", symbolName)
	}
	fmt.Fprintf(out, " (%s)\n", as2file)
}

// match reports whether name should be printed. An empty query matches all; a
// non-empty query matches exactly or, failing that, as a case-insensitive
// substring.
func match(name, query string) bool {
	if query == "" || name == query {
		return true
	}
	return strings.Contains(strings.ToLower(name), strings.ToLower(query))
}

func printSymbol(w *bufio.Writer, s *as2.Symbol) {
	typ := s.Type
	if typ == "" {
		typ = "?"
	}

	fmt.Fprintf(w, "\n=== %s === %s\n", s.Name, typ)
	field(w, "correction", trim(s.Correctionfactor))
	field(w, "offset", trim(s.Offset))
	field(w, "decimals", strconv.Itoa(s.Decimals))
	field(w, "unit", orNone(s.Unit))
	field(w, "raw range", fmt.Sprintf("%s … %s", trim(s.Min), trim(s.Max)))
	field(w, "scaled range", fmt.Sprintf("%s … %s", scaled(s.Min, s), scaled(s.Max, s)))
	field(w, "info fields", fields(s.Fields))

	if s.XAxis != nil {
		field(w, "X axis", axisStr(s.XAxis))
	}
	if s.YAxis != nil {
		field(w, "Y axis", axisStr(s.YAxis))
	}
	// Surface any raw references that don't fit the X/Y model (rare/odd entries).
	if len(s.AxisRefs) > 4 {
		field(w, "extra refs", strings.Join(s.AxisRefs[4:], ", "))
	}

	if s.Description != "" {
		first := true
		for line := range strings.SplitSeq(s.Description, "\n") {
			if first {
				field(w, "description", line)
				first = false
				continue
			}
			fmt.Fprintf(w, "%*s%s\n", descIndent, "", line)
		}
	}
}

const (
	labelWidth = 14
	// descIndent aligns continuation lines under the value column:
	// 2 leading spaces + labelWidth + len(" : ").
	descIndent = 2 + labelWidth + 3
)

func field(w *bufio.Writer, label, value string) {
	fmt.Fprintf(w, "  %-*s : %s\n", labelWidth, label, value)
}

func axisStr(a *as2.Axis) string {
	return fmt.Sprintf("support=%s  signal=%s", orNone(a.SupportPoints), orNone(a.Signal))
}

// scaled renders raw at the symbol's resolution: raw*correctionfactor + offset.
func scaled(raw float64, s *as2.Symbol) string {
	return strconv.FormatFloat(raw*s.Correctionfactor+s.Offset, 'f', s.Decimals, 64)
}

func fields(vals []float64) string {
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = trim(v)
	}
	return strings.Join(parts, " ")
}

func trim(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}
