package migemo

import (
	"errors"
	"fmt"
)

const defaultEscape = "\\.[]{}()*+-?^$|"

// Rxop names the symbols of a regular expression syntax.
type Rxop struct {
	Prefix     string
	Or         string
	StartGroup string
	EndGroup   string
	StartClass string
	EndClass   string
	Newline    string
	Escape     string
}

// JavaScript is the syntax kensaku.vim falls back on. Go's regexp package reads
// it too.
var JavaScript = Rxop{
	Or:         "|",
	StartGroup: "(?:",
	EndGroup:   ")",
	StartClass: "[",
	EndClass:   "]",
	Escape:     defaultEscape,
}

// ParseRxop reads the "rxop" option of kensaku#query(): a dictionary, a list of
// six symbols, or a list of a prefix and six symbols. nil means JavaScript.
func ParseRxop(value any) (Rxop, error) {
	var r Rxop
	switch v := value.(type) {
	case nil:
		return JavaScript, nil
	case []any:
		s := make([]string, 0, 7)
		for _, e := range v {
			text, ok := e.(string)
			if !ok {
				return r, errors.New("rxop: a list of strings is expected")
			}
			s = append(s, text)
		}
		switch len(s) {
		case 6:
			s = append([]string{""}, s...)
		case 7:
		default:
			return r, fmt.Errorf("rxop: 6 or 7 symbols are expected, got %d", len(s))
		}
		r = Rxop{s[0], s[1], s[2], s[3], s[4], s[5], s[6], defaultEscape}
	case map[string]any:
		if p, ok := v["prefix"]; ok {
			if r.Prefix, ok = p.(string); !ok {
				return r, errors.New("rxop: prefix is not a string")
			}
		}
		for key, field := range map[string]*string{
			"or":         &r.Or,
			"startGroup": &r.StartGroup,
			"endGroup":   &r.EndGroup,
			"startClass": &r.StartClass,
			"endClass":   &r.EndClass,
			"newline":    &r.Newline,
			"escape":     &r.Escape,
		} {
			text, ok := v[key].(string)
			if !ok {
				return r, fmt.Errorf("rxop: %s is not a string", key)
			}
			*field = text
		}
	default:
		return r, errors.New("rxop: a dictionary or a list is expected")
	}
	for _, c := range r.Escape {
		if c >= 128 {
			return r, errors.New("rxop: only ASCII characters can be escaped")
		}
	}
	return r, nil
}
