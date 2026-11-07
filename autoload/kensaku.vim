let s:root = expand('<sfile>:p:h:h')
let s:roman_table = v:null
let s:pending = {}
let s:next_id = 0

function! s:start() abort
  let bins = exists('g:gks_binary') ? [g:gks_binary] : []
  let bins += [s:root . '/bin/gks', s:root . '/result/bin/gks']
  for bin in bins
    if executable(bin)
      call goda#start(bin)
      call goda#notify('gks', 'configure', kensaku#config())
      return
    endif
  endfor
  throw 'gks: no binary found, run nix build in ' . s:root . ' or see :help gks-install'
endfunction

" goda keeps a failed start until the next goda#register(), and a later query
" has to see a binary built in the meantime.
function! s:register() abort
  if !goda#is_running('gks')
    call goda#register('gks', function('s:start'),
          \ #{version: 1, module: 'github.com/airRnot1106/gks.vim'})
  endif
endfunction

" What a binary needs to answer queries as this plugin does. gfm.vim
" gives it to the migemo package it links.
function! kensaku#config() abort
  return #{
        \ dictionary: g:kensaku_dictionary_cache,
        \ url: g:kensaku_dictionary_url,
        \ roman_table: s:roman_table,
        \}
endfunction

function! kensaku#query(value, ...) abort
  call s:register()
  let l:option = extend({
        \ 'rxop': g:kensaku#rxop#vim,
        \}, a:0 ? a:1 : {},
        \)
  return goda#request('gks', 'query', a:value, l:option)
endfunction

function! kensaku#query_async(value, success, ...) abort
  call s:register()
  let l:option = a:0 ? copy(a:1) : {}
  let l:Failure = has_key(l:option, 'failure') ? remove(l:option, 'failure') : function('s:failure')
  let l:option.rxop = get(l:option, 'rxop', g:kensaku#rxop#vim)
  let s:next_id += 1
  let s:pending[s:next_id] = [a:success, l:Failure]
  call goda#notify('gks', 'queryAsync', s:next_id, a:value, l:option)
endfunction

function! kensaku#_settle(id, result, error) abort
  let [l:Success, l:Failure] = remove(s:pending, a:id)
  if empty(a:error)
    call l:Success(a:result)
  else
    call l:Failure(a:error)
  endif
endfunction

function! kensaku#set_roman_table(romanTable) abort
  let s:roman_table = a:romanTable
  if goda#is_running('gks')
    call goda#notify('gks', 'setRomanTable', a:romanTable)
  endif
endfunction

function! s:failure(err) abort
  echoerr a:err
endfunction
