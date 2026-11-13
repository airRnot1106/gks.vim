package migemo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The dictionary is GPL, so it is not in this repository. GODA_KS_TEST_DICT
// names it; nix flake check sets it.
func dictionary(t testing.TB) *Dictionary {
	t.Helper()
	path := os.Getenv("GODA_KS_TEST_DICT")
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".cache/kensaku.vim/migemo-compact-dict")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skip("no dictionary: ", err)
	}
	d, err := ParseDictionary(data)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

type goldenQuery struct {
	Query            string
	Vim              string
	VimLength        int
	Javascript       string
	JavascriptLength int
}

type golden struct {
	Plain        []goldenQuery
	RomanTable   [][]any
	Custom       []goldenQuery
	Alphabet     string
	RomajiDigest string
}

func readGolden(t *testing.T) golden {
	t.Helper()
	data, err := os.ReadFile("testdata/golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var g golden
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	return g
}

var vimRxop = Rxop{`\m`, `\|`, `\%(`, `\)`, "[", "]", "", `\.[]*~^$`}

func digest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// utf16Length is the length of s as JavaScript counts it.
func utf16Length(s string) int { return len(toUnits(s)) }

func checkGolden(t *testing.T, m *Migemo, queries []goldenQuery) {
	if len(queries) < 30 {
		t.Fatalf("%d golden queries", len(queries))
	}
	for _, q := range queries {
		v, err := m.Query(q.Query, vimRxop)
		if err != nil {
			t.Fatal(err)
		}
		if digest(v) != q.Vim {
			t.Errorf("%q vim: got %d units, want %d", q.Query, utf16Length(v), q.VimLength)
		}
		j, err := m.Query(q.Query, JavaScript)
		if err != nil {
			t.Fatal(err)
		}
		if digest(j) != q.Javascript {
			t.Errorf("%q javascript: got %d units, want %d", q.Query, utf16Length(j), q.JavascriptLength)
		}
	}
}

func TestPatternsEqualJsmigemo(t *testing.T) {
	checkGolden(t, New(dictionary(t)), readGolden(t).Plain)
}

func TestPatternsEqualJsmigemoWithRomanTable(t *testing.T) {
	g := readGolden(t)
	var table []RomanEntry
	for _, row := range g.RomanTable {
		e := RomanEntry{Roman: row[0].(string), Hiragana: row[1].(string)}
		if len(row) > 2 {
			e.Remain = int(row[2].(float64))
		}
		table = append(table, e)
	}
	checkGolden(t, New(dictionary(t)).WithRomanTable(table), g.Custom)
}

func TestDefaultRomajiEqualsJsmigemo(t *testing.T) {
	g := readGolden(t)
	trie := newRomajiTrie(defaultRomanTable)
	var lines []string
	var walk func(s string, depth int)
	walk = func(s string, depth int) {
		if s != "" {
			prefix, suffixes := trie.predict(toUnits(s))
			var list []string
			for _, u := range suffixes {
				list = append(list, fromUnits(u))
			}
			slices.SortFunc(list, func(a, b string) int { return slices.Compare(toUnits(a), toUnits(b)) })
			lines = append(lines, s+" "+fromUnits(prefix)+" "+strings.Join(list, ","))
		}
		if depth > 0 {
			for _, c := range g.Alphabet {
				walk(s+string(c), depth-1)
			}
		}
	}
	walk("", 4)
	if got := digest(strings.Join(lines, "\n")); got != g.RomajiDigest {
		t.Errorf("digest of %d romaji: got %s, want %s", len(lines), got, g.RomajiDigest)
	}
}

func TestPatternMatchesTheWordsItReads(t *testing.T) {
	m := New(dictionary(t))
	for _, c := range []struct{ query, text string }{
		{"nihongo", "これは日本語です"},
		{"nihongo", "にほんご"},
		{"nihongo", "ニホンゴ"},
		{"nihongo", "ﾆﾎﾝｺﾞ"},
		{"kensaku", "全文検索"},
		{"toukyou", "東京都"},
		{"HogeFuga", "ほげふが"},
	} {
		p, err := m.Query(c.query, JavaScript)
		if err != nil {
			t.Fatal(err)
		}
		if !regexp.MustCompile(p).MatchString(c.text) {
			t.Errorf("%q does not match %q", c.query, c.text)
		}
	}
}

func TestShorterWordsReplaceTheWordsTheyStart(t *testing.T) {
	g := newGenerator(JavaScript)
	for _, w := range []string{"abc", "ab", "abd", "x"} {
		g.add(toUnits(w))
	}
	if got, want := fromUnits(g.generate()), "(?:x|ab)"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseRxop(t *testing.T) {
	list := []any{"\\m", "|", "(", ")", "[", "]", ""}
	r, err := ParseRxop(list)
	if err != nil {
		t.Fatal(err)
	}
	if r.Prefix != "\\m" || r.Or != "|" || r.Escape != defaultEscape {
		t.Errorf("%+v", r)
	}
	if r, _ := ParseRxop(nil); r != JavaScript {
		t.Errorf("%+v", r)
	}
	for _, bad := range []any{"x", []any{"a"}, []any{1, 2, 3, 4, 5, 6}, map[string]any{"or": "|"}} {
		if _, err := ParseRxop(bad); err == nil {
			t.Errorf("%v: no error", bad)
		}
	}
}

func BenchmarkParseDictionary(b *testing.B) {
	path := os.Getenv("GODA_KS_TEST_DICT")
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".cache/kensaku.vim/migemo-compact-dict")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		b.Skip(err)
	}
	for b.Loop() {
		if _, err := ParseDictionary(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkQuery(b *testing.B) {
	m := New(dictionary(b))
	for b.Loop() {
		if _, err := m.Query("ka", vimRxop); err != nil {
			b.Fatal(err)
		}
	}
}
