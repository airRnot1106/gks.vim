if exists('g:loaded_kensaku')
  finish
endif
let g:loaded_kensaku = 1

let s:root = expand('<sfile>:p:h:h')

function! s:define(name, default) abort
  let g:{a:name} = get(g:, a:name, a:default)
endfunction

function! s:dictionary() abort
  for dir in [s:root, s:root . '/result']
    let path = dir . '/share/gks/migemo-compact-dict'
    if filereadable(path)
      return path
    endif
  endfor
  return expand('~/.cache/kensaku.vim/migemo-compact-dict')
endfunction

call s:define(
      \ 'kensaku_dictionary_url',
      \ 'https://github.com/oguna/migemo-compact-dict-latest/releases/download/v0.2/migemo-compact-dict',
      \)
" The Nix package holds the dictionary, so that a query does not download it.
call s:define('kensaku_dictionary_cache', s:dictionary())
