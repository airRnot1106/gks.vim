# gks.vim

Search Japanese text in Vim and Neovim by typing romaji.

gks.vim is a port of [kensaku.vim](https://github.com/lambdalisue/kensaku.vim) from [denops.vim](https://github.com/vim-denops/denops.vim) to [goda.vim](https://github.com/Shougo/goda.vim). It defines the same `kensaku#query()`, `kensaku#query_async()` and `kensaku#set_roman_table()`, and needs no Deno. [kensaku-search.vim](https://github.com/lambdalisue/kensaku-search.vim) works with it unchanged.

## Features

- Works in Vim and Neovim
- Gives the same patterns as kensaku.vim: the Go package `migemo` is a port of [jsmigemo](https://github.com/oguna/jsmigemo) and reads the same dictionary
- The first query takes about 10 ms, because the binary starts and loads the dictionary in that time
- With a Nix build, a query needs no network

## Requirements

- Vim 9.1 with `+job` and `+channel`, or Neovim 0.11
- [goda.vim](https://github.com/Shougo/goda.vim)
- [Nix](https://nixos.org) with flakes enabled, or [Go](https://go.dev) 1.26, to build the binary

Windows is not supported.

## Installation

### Using a plugin manager

Install both goda.vim and gks.vim, then run `nix build` inside the plugin directory. The plugin expects the binary at `result/bin/gks` and the dictionary at `result/share/gks/migemo-compact-dict`.

With [lazy.nvim](https://github.com/folke/lazy.nvim):

```lua
{
  'airRnot1106/gks.vim',
  dependencies = { 'Shougo/goda.vim' },
  build = 'nix build',
}
```

With [vim-plug](https://github.com/junegunn/vim-plug):

```vim
Plug 'Shougo/goda.vim'
Plug 'airRnot1106/gks.vim', { 'do': 'nix build' }
```

### Without Nix

Build the binary with Go instead. Run this inside the plugin directory:

```sh
go build -o bin/gks .
```

The plugin finds the binary at `bin/gks`. The first query downloads the dictionary to `~/.cache/kensaku.vim/migemo-compact-dict`, as kensaku.vim does, and reuses a file it already downloaded.

### Using Nix Flakes

The default package provides the Vim plugin along with the binary and the dictionary, and handles the goda.vim dependency automatically. The flake also outputs `gks` (the binary), `dictionary`, and `goda-vim`.

## Usage

With kensaku-search.vim, `<CR>` replaces the romaji on the command line of `/` and `?`:

```vim
cmap <CR> <Plug>(kensaku-search-replace)<CR>
```

From Vim script:

```vim
let @/ = kensaku#query('kensaku')
normal! n

call kensaku#query_async('kensaku', {pattern -> execute('let @/ = pattern')})
```

From Go, for another goda.vim plugin. `kensaku#config()` gives the `migemo.Config` of the user:

```go
var loader migemo.Loader
loader.Configure(config)
m, err := loader.Migemo()
pattern, err := m.Query("nihongo", migemo.JavaScript)
```

See `:help gks`.

## Options

| Variable | Default | Description |
| --- | --- | --- |
| `g:gks_binary` | Unset | Path to the binary. Overrides `bin/gks` and `result/bin/gks`. |
| `g:kensaku_dictionary_url` | The v0.2 release of [migemo-compact-dict-latest](https://github.com/oguna/migemo-compact-dict-latest) | URL of the dictionary. |
| `g:kensaku_dictionary_cache` | The file of the Nix build, or `~/.cache/kensaku.vim/migemo-compact-dict` | Path of the dictionary. |

If a query fails, check `goda#log('gks')`.

## Differences from kensaku.vim

- `kensaku#query()` throws when the binary cannot start, where kensaku.vim returned an empty string when denops.vim was not ready.
- There is no denops dispatcher. Other plugins use `kensaku#query()` or the Go package.

## Development

```sh
nix develop
go test ./...
nix flake check
```

`nix flake check` runs the Go tests, including the golden tests, and the E2E test in real Vim and Neovim. Outside Nix, the tests read the dictionary from `$GKS_TEST_DICT` or `~/.cache/kensaku.vim/migemo-compact-dict`, and skip without it.

`migemo/testdata/golden.json` holds digests of the patterns of jsmigemo 0.4.8 for 55 queries in two syntaxes, with and without a custom roman table, and of its romaji conversion for every string up to four characters. `migemo/testdata/golden.ts` writes it.

## Special thanks

- [kensaku.vim](https://github.com/lambdalisue/kensaku.vim)
- [jsmigemo](https://github.com/oguna/jsmigemo)
- [migemo-compact-dict-latest](https://github.com/oguna/migemo-compact-dict-latest)

## License

MIT, except the dictionary, which is not part of this repository. The dictionary is made from the SKK dictionaries and is under the GPL. The Nix package fetches it from its release and keeps it as a separate data file.
