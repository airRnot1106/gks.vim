package migemo

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/bits"
)

// bitVector answers rank and select over a bit sequence with a two level
// index: lb counts per 512 bits and sb per 64 bits within those.
type bitVector struct {
	words []uint32
	size  int
	lb    []uint32
	sb    []uint16
}

func newBitVector(words []uint32, size int) *bitVector {
	v := &bitVector{
		words: words,
		size:  size,
		lb:    make([]uint32, (size+511)>>9),
	}
	v.sb = make([]uint16, len(v.lb)*8)
	var sum uint32
	var sumInLb int
	for i := range v.sb {
		bc := 0
		if i < len(words)>>1 {
			bc = bits.OnesCount32(words[i*2]) + bits.OnesCount32(words[i*2+1])
		}
		v.sb[i] = uint16(sumInLb)
		sumInLb += bc
		if i&7 == 7 {
			v.lb[i>>3] = sum
			sum += uint32(sumInLb)
			sumInLb = 0
		}
	}
	return v
}

func (v *bitVector) word(i int) uint32 {
	if i < len(v.words) {
		return v.words[i]
	}
	return 0
}

// rank counts the bits equal to b in [0, pos).
func (v *bitVector) rank(pos int, b bool) int {
	var count int
	if pos>>6 < len(v.sb) {
		count = int(v.sb[pos>>6]) + int(v.lb[pos>>9])
	}
	if pos&63 >= 32 {
		count += bits.OnesCount32(v.word((pos >> 5) &^ 1))
	}
	count += bits.OnesCount32(v.word(pos>>5) & (uint32(1)<<(pos&31) - 1))
	if b {
		return count
	}
	return pos - count
}

// lbCount and sbCount are the bits equal to b before an index entry.
func (v *bitVector) lbCount(i int, b bool) int {
	if b {
		return int(v.lb[i])
	}
	return 512*i - int(v.lb[i])
}

func (v *bitVector) sbCount(i int, b bool) int {
	if b {
		return int(v.sb[i])
	}
	return 64*(i&7) - int(v.sb[i])
}

// selectBit returns the position of the count-th bit equal to b, from 1.
func (v *bitVector) selectBit(count int, b bool) int {
	low, high := -1, len(v.lb)
	for high-low > 1 {
		mid := (high + low) >> 1
		if v.lbCount(mid, b) < count {
			low = mid
		} else {
			high = mid
		}
	}
	lbIndex := high - 1
	if lbIndex == -1 {
		return 0
	}
	countInLb := count - v.lbCount(lbIndex, b)
	low, high = lbIndex*8-1, lbIndex*8+8
	for high-low > 1 {
		mid := (high + low) >> 1
		if v.sbCount(mid, b) < countInLb {
			low = mid
		} else {
			high = mid
		}
	}
	sbIndex := high - 1
	countInSb := countInLb - v.sbCount(sbIndex, b)
	lower, upper := v.word(sbIndex*2), v.word(sbIndex*2+1)
	if !b {
		lower, upper = ^lower, ^upper
	}
	i := 0
	if n := bits.OnesCount32(lower); countInSb > n {
		lower = upper
		countInSb -= n
		i = 32
	}
	for countInSb > 0 {
		countInSb -= int(lower & 1)
		lower >>= 1
		i++
	}
	return sbIndex*64 + i - 1
}

func (v *bitVector) nextClearBit(from int) int {
	u := from >> 5
	word := ^v.word(u) & (uint32(0xffffffff) << (from & 31))
	for {
		if word != 0 {
			return u*32 + bits.TrailingZeros32(word)
		}
		u++
		if u == len(v.words) {
			return -1
		}
		word = ^v.word(u)
	}
}

func (v *bitVector) get(pos int) bool {
	return v.word(pos>>5)>>(pos&31)&1 == 1
}

// loudsTrie is a trie stored in the LOUDS encoding. Nodes are numbered from 1,
// and edges[i] is the code unit on the edge into node i.
type loudsTrie struct {
	vector *bitVector
	edges  []uint16
}

func (t *loudsTrie) reverseLookup(index int) ([]uint16, error) {
	if index <= 0 || index >= len(t.edges) {
		return nil, fmt.Errorf("migemo: node %d is out of range", index)
	}
	var word []uint16
	for index > 1 {
		word = append(word, t.edges[index])
		index = t.parent(index)
	}
	for i, j := 0, len(word)-1; i < j; i, j = i+1, j-1 {
		word[i], word[j] = word[j], word[i]
	}
	return word, nil
}

func (t *loudsTrie) parent(x int) int {
	return t.vector.rank(t.vector.selectBit(x, true), false)
}

func (t *loudsTrie) firstChild(x int) int {
	y := t.vector.selectBit(x, false) + 1
	if t.vector.get(y) {
		return t.vector.rank(y, true) + 1
	}
	return -1
}

func (t *loudsTrie) traverse(index int, c uint16) int {
	first := t.firstChild(index)
	if first == -1 {
		return -1
	}
	start := t.vector.selectBit(first, true)
	low, high := first, first+t.vector.nextClearBit(start)-start-1
	for low <= high {
		mid := (low + high) >> 1
		switch {
		case t.edges[mid] < c:
			low = mid + 1
		case t.edges[mid] > c:
			high = mid - 1
		default:
			return mid
		}
	}
	return -1
}

func (t *loudsTrie) lookup(key []uint16) int {
	index := 1
	for _, c := range key {
		if index = t.traverse(index, c); index == -1 {
			return -1
		}
	}
	return index
}

// descendants calls visit for index and every node below it, level by level.
func (t *loudsTrie) descendants(index int, visit func(int) error) error {
	lower, upper := index, index+1
	for upper-lower > 0 {
		for i := lower; i < upper; i++ {
			if err := visit(i); err != nil {
				return err
			}
		}
		lower = t.vector.rank(t.vector.selectBit(lower, false)+1, true) + 1
		upper = t.vector.rank(t.vector.selectBit(upper, false)+1, true) + 1
	}
	return nil
}

// Dictionary is a migemo-compact-dict file: a key trie of readings, a value
// trie of words, and the mapping between them.
type Dictionary struct {
	keys       *loudsTrie
	values     *loudsTrie
	vector     *bitVector
	mapping    []int32
	hasMapping []bool
}

type dictReader struct {
	data []byte
	off  int
}

var errTruncated = errors.New("migemo: the dictionary is truncated")

func (r *dictReader) need(n int) error {
	if n < 0 || len(r.data)-r.off < n {
		return errTruncated
	}
	return nil
}

func (r *dictReader) uint32() (uint32, error) {
	if err := r.need(4); err != nil {
		return 0, err
	}
	v := binary.BigEndian.Uint32(r.data[r.off:])
	r.off += 4
	return v, nil
}

func (r *dictReader) vector() (*bitVector, error) {
	size, err := r.uint32()
	if err != nil {
		return nil, err
	}
	words := make([]uint32, (int(size)+63)>>6*2)
	if err := r.need(len(words) * 4); err != nil {
		return nil, err
	}
	for i := 0; i < len(words); i += 2 {
		words[i+1], _ = r.uint32()
		words[i], _ = r.uint32()
	}
	return newBitVector(words, int(size)), nil
}

// trie reads a trie. Key edges are one byte each, see decodeHiragana.
func (r *dictReader) trie(compact bool) (*loudsTrie, error) {
	size, err := r.uint32()
	if err != nil {
		return nil, err
	}
	edges := make([]uint16, size)
	width := 2
	if compact {
		width = 1
	}
	if err := r.need(len(edges) * width); err != nil {
		return nil, err
	}
	for i := range edges {
		if compact {
			if edges[i], err = decodeHiragana(r.data[r.off]); err != nil {
				return nil, err
			}
		} else {
			edges[i] = binary.BigEndian.Uint16(r.data[r.off:])
		}
		r.off += width
	}
	vector, err := r.vector()
	if err != nil {
		return nil, err
	}
	return &loudsTrie{vector, edges}, nil
}

func decodeHiragana(c byte) (uint16, error) {
	switch {
	case 0x20 <= c && c <= 0x7e:
		return uint16(c), nil
	case 0xa1 <= c && c <= 0xf6:
		return uint16(c) + 0x3040 - 0xa0, nil
	}
	return 0, fmt.Errorf("migemo: byte %#x is not a compact hiragana", c)
}

// ParseDictionary reads the contents of a migemo-compact-dict file.
func ParseDictionary(data []byte) (*Dictionary, error) {
	r := &dictReader{data: data}
	d := &Dictionary{}
	var err error
	if d.keys, err = r.trie(true); err != nil {
		return nil, err
	}
	if d.values, err = r.trie(false); err != nil {
		return nil, err
	}
	if d.vector, err = r.vector(); err != nil {
		return nil, err
	}
	size, err := r.uint32()
	if err != nil {
		return nil, err
	}
	d.mapping = make([]int32, size)
	if err := r.need(len(d.mapping) * 4); err != nil {
		return nil, err
	}
	for i := range d.mapping {
		v, _ := r.uint32()
		d.mapping[i] = int32(v)
	}
	if r.off != len(data) {
		return nil, fmt.Errorf("migemo: %d bytes after the dictionary", len(data)-r.off)
	}
	nodes := d.vector.rank(d.vector.size+1, false)
	d.hasMapping = make([]bool, nodes)
	position := 0
	for node := 1; node < nodes; node++ {
		d.hasMapping[node] = d.vector.get(position + 1)
		position = d.vector.nextClearBit(position + 1)
	}
	return d, nil
}

// predictive calls add for every word whose reading starts with key.
func (d *Dictionary) predictive(key []uint16, add func([]uint16)) error {
	index := d.keys.lookup(key)
	if index <= 1 {
		return nil
	}
	return d.keys.descendants(index, func(node int) error {
		if node >= len(d.hasMapping) || !d.hasMapping[node] {
			return nil
		}
		start := d.vector.selectBit(node, false)
		end := d.vector.nextClearBit(start + 1)
		offset := d.vector.rank(start, false)
		for i := range end - start - 1 {
			word, err := d.values.reverseLookup(int(d.mapping[start-offset+i]))
			if err != nil {
				return err
			}
			add(word)
		}
		return nil
	})
}
