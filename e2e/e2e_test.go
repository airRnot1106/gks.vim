// Package e2e runs autoload/kensaku.vim in real editors.
package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

var editors = []struct {
	name    string
	command []string
}{
	{"vim", []string{"vim", "-u", "NONE", "-N", "-n", "-i", "NONE", "-es", "-S"}},
	{"nvim", []string{"nvim", "-u", "NONE", "-n", "-i", "NONE", "--headless", "-S"}},
}

func goda(t *testing.T, root string) string {
	t.Helper()
	if dir := os.Getenv("GODA_KS_E2E_GODA"); dir != "" {
		return dir
	}
	list := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/Shougo/goda.vim")
	list.Dir = root
	out, err := list.Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func dictionary(t *testing.T) string {
	t.Helper()
	path := os.Getenv("GODA_KS_TEST_DICT")
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".cache/kensaku.vim/migemo-compact-dict")
	}
	if _, err := os.Stat(path); err != nil {
		t.Skip("no dictionary: ", err)
	}
	return path
}

func TestEditors(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	binary := filepath.Join(dir, "goda-ks")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	for _, editor := range editors {
		t.Run(editor.name, func(t *testing.T) {
			if _, err := exec.LookPath(editor.command[0]); err != nil {
				if os.Getenv("GODA_KS_E2E_STRICT") != "" {
					t.Fatal(err)
				}
				t.Skip(err)
			}
			out := filepath.Join(t.TempDir(), "out.json")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, editor.command[0], append(editor.command[1:], "e2e/testdata/driver.vim")...)
			cmd.Dir = root
			cmd.Env = append(os.Environ(),
				"GODA_KS_E2E_PLUGIN="+root,
				"GODA_KS_E2E_GODA="+goda(t, root),
				"GODA_KS_E2E_BINARY="+binary,
				"GODA_KS_E2E_DICT="+dictionary(t),
				"GODA_KS_E2E_OUT="+out,
			)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s", err, output)
			}
			data, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			var result map[string]any
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatal(err)
			}
			check(t, result)
		})
	}
}

func check(t *testing.T, result map[string]any) {
	t.Helper()
	if e, ok := result["error"]; ok {
		t.Fatal(e)
	}
	if got := fmt.Sprint(result["search_line"]); got != "2" {
		t.Errorf("/nihongo landed on line %s", got)
	}
	if got, _ := result["vim"].(string); !strings.HasPrefix(got, `\m\%(`) || !strings.Contains(got, "日本") {
		t.Errorf("vim pattern: %q", got)
	}
	javascript, _ := result["javascript"].(string)
	if !regexp.MustCompile(javascript).MatchString("日本語") {
		t.Errorf("javascript pattern: %q", javascript)
	}
	if got, _ := result["async"].(string); !strings.Contains(got, "検索") {
		t.Errorf("query_async: %q", got)
	}
	if got, _ := result["failed"].(string); !strings.Contains(got, "rxop") {
		t.Errorf("failure callback: %q", got)
	}
	if got, _ := result["roman"].(string); !strings.Contains(got, "かん") {
		t.Errorf("roman table: %q", got)
	}
}
