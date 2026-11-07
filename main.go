package main

import (
	"github.com/Shougo/goda.vim/host"

	"github.com/airRnot1106/gks.vim/internal/kensaku"
)

func main() {
	host.Main(kensaku.New())
}
