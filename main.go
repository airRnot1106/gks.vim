package main

import (
	"github.com/Shougo/goda.vim/host"

	"github.com/airRnot1106/goda-ks.vim/internal/kensaku"
)

func main() {
	host.Main(kensaku.New())
}
