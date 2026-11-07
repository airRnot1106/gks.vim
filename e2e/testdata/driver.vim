" Runs the plugin in a real editor and writes what it saw to $GKS_E2E_OUT.
set encoding=utf-8
let &runtimepath = join(map([$GKS_E2E_PLUGIN, $GKS_E2E_GODA], {_, dir -> escape(dir, ',')}) + [&runtimepath], ',')
let g:gks_binary = $GKS_E2E_BINARY
let g:kensaku_dictionary_cache = $GKS_E2E_DICT
runtime plugin/kensaku.vim

let s:result = {}

function! s:wait(name) abort
  for _ in range(500)
    if has_key(s:result, a:name)
      return
    endif
    sleep 10m
  endfor
  let s:result[a:name] = 'timeout'
endfunction

function! s:record(name, value) abort
  let s:result[a:name] = a:value
endfunction

function! s:replace() abort
  return "\<C-u>" . kensaku#query(getcmdline())
endfunction

try
  " The first query starts the binary from an expression mapping, where text
  " changes are not allowed.
  cnoremap <expr> <C-x> <SID>replace()
  call setline(1, ['first', '日本語のテキスト'])
  call feedkeys("/nihongo\<C-x>\<CR>", 'tx')
  let s:result.search_line = line('.')

  let s:result.vim = kensaku#query('nihongo')
  let s:result.javascript = kensaku#query('nihongo', #{rxop: g:kensaku#rxop#javascript})

  call kensaku#query_async('kensaku', {v -> s:record('async', v)})
  call s:wait('async')
  call kensaku#query_async('kensaku', {v -> s:record('failed', 'success')}, #{
        \ rxop: 1,
        \ failure: {e -> s:record('failed', e)},
        \})
  call s:wait('failed')

  call kensaku#set_roman_table([['kz', 'かん', 0], ['ka', 'か']])
  let s:result.roman = kensaku#query('kz', #{rxop: g:kensaku#rxop#javascript})
catch
  let s:result.error = v:exception
endtry

call writefile([json_encode(s:result)], $GKS_E2E_OUT)
qall!
