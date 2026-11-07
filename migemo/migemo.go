// Package migemo turns romaji into a regular expression that matches the
// Japanese words it reads as. It is a port of jsmigemo, and gives the same
// patterns from the same migemo-compact-dict file.
//
// Text is handled as UTF-16 code units, as jsmigemo does, because the order of
// alternatives in a pattern follows them.
package migemo

import (
	"regexp"
	"slices"
	"strings"
	"unicode/utf16"
)

func toUnits(s string) []uint16 {
	return utf16.Encode([]rune(s))
}

func fromUnits(u []uint16) string {
	return string(utf16.Decode(u))
}

func convert(s string, table map[rune]string) string {
	var b strings.Builder
	for _, c := range s {
		if to, ok := table[c]; ok {
			b.WriteString(to)
		} else {
			b.WriteRune(c)
		}
	}
	return b.String()
}

func hiraganaToKatakana(s []uint16) []uint16 {
	out := slices.Clone(s)
	for i, c := range out {
		if 'ぁ' <= c && c <= 'ん' {
			out[i] = c - 'ぁ' + 'ァ'
		}
	}
	return out
}

// Migemo holds a dictionary and a roman table. It does not change after it is
// made, so queries may run concurrently.
type Migemo struct {
	dict   *Dictionary
	romaji romajiProcessor
}

// New returns a Migemo with the default roman table.
func New(dict *Dictionary) *Migemo {
	return &Migemo{dict, newRomajiTrie(defaultRomanTable)}
}

// WithRomanTable returns a Migemo that reads romaji by table.
func (m *Migemo) WithRomanTable(table []RomanEntry) *Migemo {
	return &Migemo{m.dict, newRomajiTable(table)}
}

const whitespace = `\t\n\x{0b}\f\r \x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}`

// clause splits a query at capital letters: "HogeFuga" is two words.
var clause = regexp.MustCompile(`[^A-Z` + whitespace + `]+|[A-Z]{2,}|[A-Z][^A-Z` + whitespace + `]+|[A-Z][` + whitespace + `]*$`)

// Query returns the pattern for word, with the prefix of r in front.
func (m *Migemo) Query(word string, r Rxop) (string, error) {
	out := toUnits(r.Prefix)
	for _, w := range clause.FindAllString(word, -1) {
		pattern, err := m.queryWord(w, r)
		if err != nil {
			return "", err
		}
		out = append(out, pattern...)
	}
	return fromUnits(out), nil
}

func (m *Migemo) queryWord(word string, r Rxop) ([]uint16, error) {
	g := newGenerator(r)
	g.add(toUnits(word))
	lower := toUnits(strings.ToLower(word))
	if err := m.dict.predictive(lower, g.add); err != nil {
		return nil, err
	}
	g.add(toUnits(convert(word, hanToZen)))
	g.add(toUnits(convert(word, zenToHan)))
	prefix, suffixes := m.romaji.predict(lower)
	for _, suffix := range suffixes {
		hiragana := slices.Concat(prefix, suffix)
		g.add(hiragana)
		if err := m.dict.predictive(hiragana, g.add); err != nil {
			return nil, err
		}
		katakana := hiraganaToKatakana(hiragana)
		g.add(katakana)
		g.add(toUnits(convert(fromUnits(katakana), zenToHan)))
	}
	return g.generate(), nil
}
