package migemo

import (
	"fmt"
	"slices"
	"sort"
)

// RomanEntry is one rule of a roman table. The last Remain characters of Roman
// stay in the input after the rule applies.
type RomanEntry struct {
	Roman    string
	Hiragana string
	Remain   int
}

// romajiProcessor splits romaji into the hiragana it certainly means and the
// candidates for its unfinished end.
type romajiProcessor interface {
	predict(romaji []uint16) (prefix []uint16, suffixes [][]uint16)
}

// romajiTrie is the processor of the default roman table.
type romajiTrie struct {
	root *romajiNode
}

type romajiNode struct {
	children map[uint16]*romajiNode
	entry    *unitEntry
}

type unitEntry struct {
	roman, hiragana []uint16
	remain          int
	index           int32
}

func newRomajiTrie(table []RomanEntry) *romajiTrie {
	root := &romajiNode{}
	for _, e := range table {
		n := root
		for _, c := range toUnits(e.Roman) {
			if n.children == nil {
				n.children = map[uint16]*romajiNode{}
			}
			child := n.children[c]
			if child == nil {
				child = &romajiNode{}
				n.children[c] = child
			}
			n = child
		}
		n.entry = &unitEntry{hiragana: toUnits(e.Hiragana), remain: e.Remain}
	}
	return &romajiTrie{root}
}

func (n *romajiNode) collect(seen map[string]bool, out *[][]uint16) {
	if n.entry != nil {
		if key := fromUnits(n.entry.hiragana); !seen[key] {
			seen[key] = true
			*out = append(*out, n.entry.hiragana)
		}
	}
	for _, child := range n.children {
		child.collect(seen, out)
	}
}

func (t *romajiTrie) predict(romaji []uint16) ([]uint16, [][]uint16) {
	var prefix []uint16
	cursor := 0
	for cursor < len(romaji) {
		var longest *unitEntry
		n := t.root
		matched := 0
		for cursor+matched < len(romaji) {
			child := n.children[romaji[cursor+matched]]
			if child == nil {
				break
			}
			n = child
			matched++
			if n.entry != nil {
				longest = n.entry
			}
		}
		switch {
		case cursor+matched == len(romaji):
			var list [][]uint16
			n.collect(map[string]bool{}, &list)
			if len(list) == 1 {
				return append(prefix, list[0]...), [][]uint16{nil}
			}
			return prefix, list
		case longest != nil:
			prefix = append(prefix, longest.hiragana...)
			cursor += matched - longest.remain
		default:
			prefix = append(prefix, romaji[cursor])
			cursor++
		}
	}
	return prefix, [][]uint16{nil}
}

// romajiTable is the processor of a roman table the user gave. Its entries are
// sorted by index, the first four code units of roman packed into an integer.
type romajiTable struct {
	entries []unitEntry
	indexes []int64
}

func romanIndex(roman []uint16, start, end int) int32 {
	var result int32
	for i := range 4 {
		var c uint16
		if index := i + start; index < len(roman) && index < end {
			c = roman[index]
		}
		result |= int32(c)
		if i < 3 {
			result <<= 8
		}
	}
	return result
}

func newRomajiTable(table []RomanEntry) *romajiTable {
	t := &romajiTable{}
	for _, e := range table {
		roman := toUnits(e.Roman)
		t.entries = append(t.entries, unitEntry{roman, toUnits(e.Hiragana), e.Remain, romanIndex(roman, 0, 4)})
	}
	sort.SliceStable(t.entries, func(a, b int) bool { return t.entries[a].index < t.entries[b].index })
	for _, e := range t.entries {
		t.indexes = append(t.indexes, int64(e.index))
	}
	return t
}

// search returns the index of key in [from, to), or -(insertion point + 1).
func (t *romajiTable) search(from, to int, key int64) int {
	low, high := from, to-1
	for low <= high {
		mid := (low + high) >> 1
		switch {
		case t.indexes[mid] < key:
			low = mid + 1
		case t.indexes[mid] > key:
			high = mid - 1
		default:
			return mid
		}
	}
	return -(low + 1)
}

// keyStep is the distance from a key of width units to the next key.
func keyStep(width int) int64 {
	return int64(int32(1) << ((32 - 8*width) & 31))
}

func (t *romajiTable) candidates(roman []uint16, offset int) []*unitEntry {
	start, end := 0, len(t.indexes)
	for i := range 4 {
		if len(roman) <= offset+i {
			break
		}
		key := int64(romanIndex(roman, offset, offset+i+1))
		if start = t.search(start, end, key); start < 0 {
			start = -start - 1
		}
		if end = t.search(start, end, key+keyStep(i+1)); end < 0 {
			end = -end - 1
		}
		if end-start == 1 {
			return []*unitEntry{&t.entries[start]}
		}
	}
	var result []*unitEntry
	for i := start; i < end; i++ {
		result = append(result, &t.entries[i])
	}
	return result
}

func (t *romajiTable) predict(romaji []uint16) ([]uint16, [][]uint16) {
	var hiragana []uint16
	start, end := 0, 1
	for start < len(romaji) {
		lastFound := -1
		lower, upper := 0, len(t.indexes)
		for upper-lower > 1 && end <= len(romaji) {
			key := int64(romanIndex(romaji, start, end))
			lower = t.search(lower, upper, key)
			if lower >= 0 {
				lastFound = lower
			} else {
				lower = -lower - 1
			}
			if upper = t.search(lower, upper, key+keyStep(end-start)); upper < 0 {
				upper = -upper - 1
			}
			end++
		}
		if end > len(romaji) && upper-lower > 1 {
			var list [][]uint16
			add := func(s []uint16) {
				if !slices.ContainsFunc(list, func(e []uint16) bool { return slices.Equal(e, s) }) {
					list = append(list, s)
				}
			}
			for i := lower; i < upper; i++ {
				re := &t.entries[i]
				if re.remain == 0 {
					add(re.hiragana)
					continue
				}
				for _, next := range t.candidates(romaji, end-1-re.remain) {
					if next.remain == 0 {
						add(slices.Concat(re.hiragana, next.hiragana))
					}
				}
			}
			if len(list) == 1 {
				return append(hiragana, list[0]...), [][]uint16{nil}
			}
			return hiragana, list
		}
		if lastFound >= 0 {
			entry := &t.entries[lastFound]
			hiragana = append(hiragana, entry.hiragana...)
			start += len(entry.roman) - entry.remain
		} else {
			hiragana = append(hiragana, romaji[start])
			start++
		}
		end = start + 1
	}
	return hiragana, [][]uint16{nil}
}

// ParseRomanTable reads the argument of kensaku#set_roman_table(): a list of
// [roman, hiragana] and [roman, hiragana, remain].
func ParseRomanTable(rows []any) ([]RomanEntry, error) {
	table := make([]RomanEntry, 0, len(rows))
	for i, row := range rows {
		fields, ok := row.([]any)
		if !ok || len(fields) < 2 || len(fields) > 3 {
			return nil, fmt.Errorf("roman table: row %d is not [roman, hiragana, remain?]", i+1)
		}
		var e RomanEntry
		if e.Roman, ok = fields[0].(string); !ok {
			return nil, fmt.Errorf("roman table: row %d: roman is not a string", i+1)
		}
		if e.Hiragana, ok = fields[1].(string); !ok {
			return nil, fmt.Errorf("roman table: row %d: hiragana is not a string", i+1)
		}
		if len(fields) == 3 {
			switch remain := fields[2].(type) {
			case int64:
				e.Remain = int(remain)
			case float64:
				e.Remain = int(remain)
			default:
				return nil, fmt.Errorf("roman table: row %d: remain is not a number", i+1)
			}
		}
		table = append(table, e)
	}
	return table, nil
}
