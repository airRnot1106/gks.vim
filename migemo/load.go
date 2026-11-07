package migemo

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Config says where the dictionary is. RomanTable is the argument of
// kensaku#set_roman_table(), or nil for the default table.
type Config struct {
	Dictionary string `json:"dictionary"`
	URL        string `json:"url"`
	RomanTable []any  `json:"roman_table"`
}

// Loader keeps the Migemo of a Config. The zero value has no dictionary.
type Loader struct {
	mu     sync.Mutex
	config Config
	base   *Migemo
	live   *Migemo
}

// Configure replaces the Config, and starts loading the dictionary so that the
// first query finds it ready or waits for less.
func (l *Loader) Configure(c Config) {
	l.mu.Lock()
	if c.Dictionary != l.config.Dictionary || c.URL != l.config.URL {
		l.base = nil
	}
	l.config = c
	l.live = nil
	l.mu.Unlock()
	go func() {
		if _, err := l.Migemo(); err != nil {
			log.Print(err)
		}
	}()
}

// SetRomanTable replaces the roman table of the Config.
func (l *Loader) SetRomanTable(table []any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.config.RomanTable = table
	l.live = nil
}

// Migemo returns the Migemo of the Config, loading the dictionary when needed.
// It holds the lock meanwhile, because every query needs the result. A failed
// load is tried again by the next call.
func (l *Loader) Migemo() (*Migemo, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.live != nil {
		return l.live, nil
	}
	if l.base == nil {
		if l.config.Dictionary == "" {
			return nil, errors.New("migemo: no dictionary configured")
		}
		dict, err := Load(l.config.Dictionary, l.config.URL)
		if err != nil {
			return nil, err
		}
		l.base = New(dict)
	}
	if l.config.RomanTable == nil {
		l.live = l.base
		return l.live, nil
	}
	table, err := ParseRomanTable(l.config.RomanTable)
	if err != nil {
		return nil, err
	}
	l.live = l.base.WithRomanTable(table)
	return l.live, nil
}

// Load reads the dictionary at path, and first downloads it from url when the
// file does not exist.
func Load(path, url string) (*Dictionary, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) && url != "" {
		if err := download(path, url); err != nil {
			return nil, err
		}
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, err
	}
	return ParseDictionary(data)
}

// download writes through a temporary file, so that an interrupted download
// does not leave a partial dictionary at path.
func download(path, url string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	client := http.Client{Timeout: time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("migemo: %s: %s", url, resp.Status)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
