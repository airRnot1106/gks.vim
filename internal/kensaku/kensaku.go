// Package kensaku is the gks plugin. It answers migemo queries for
// autoload/kensaku.vim.
package kensaku

import (
	"context"
	"log"

	"github.com/Shougo/goda.vim/sdk"

	"github.com/airRnot1106/gks.vim/migemo"
)

// scriptVersion matches the version autoload/kensaku.vim passes to goda#register.
const scriptVersion = 1

// Kensaku answers queries with the Migemo of its loader.
type Kensaku struct {
	loader migemo.Loader
}

// New returns a plugin with no dictionary configured.
func New() *Kensaku {
	return &Kensaku{}
}

// Name implements sdk.Plugin.
func (k *Kensaku) Name() string { return "gks" }

// ScriptVersion implements sdk.ScriptVersioner.
func (k *Kensaku) ScriptVersion() int { return scriptVersion }

// Handlers implements sdk.Plugin.
func (k *Kensaku) Handlers() map[string]sdk.Handler {
	return map[string]sdk.Handler{
		"configure":     k.configure,
		"query":         k.query,
		"queryAsync":    k.queryAsync,
		"setRomanTable": k.setRomanTable,
	}
}

func (k *Kensaku) configure(_ context.Context, _ sdk.Host, params []any) (any, error) {
	var config migemo.Config
	if err := sdk.DecodeArgs(params, &config); err != nil {
		return nil, err
	}
	k.loader.Configure(config)
	return nil, nil
}

func (k *Kensaku) setRomanTable(_ context.Context, _ sdk.Host, params []any) (any, error) {
	var table []any
	if err := sdk.DecodeArgs(params, &table); err != nil {
		return nil, err
	}
	k.loader.SetRomanTable(table)
	return nil, nil
}

type option struct {
	Rxop any `json:"rxop"`
}

func (k *Kensaku) run(value string, o option) (string, error) {
	rxop, err := migemo.ParseRxop(o.Rxop)
	if err != nil {
		return "", err
	}
	m, err := k.loader.Migemo()
	if err != nil {
		return "", err
	}
	return m.Query(value, rxop)
}

func (k *Kensaku) query(_ context.Context, _ sdk.Host, params []any) (any, error) {
	var value string
	var o option
	if err := sdk.DecodeArgs(params, &value, sdk.Optional(&o)); err != nil {
		return nil, err
	}
	return k.run(value, o)
}

// queryAsync answers through kensaku#_settle. It leaves the notification queue
// at once, since loading the dictionary may take a download.
func (k *Kensaku) queryAsync(ctx context.Context, h sdk.Host, params []any) (any, error) {
	var id int
	var value string
	var o option
	if err := sdk.DecodeArgs(params, &id, &value, sdk.Optional(&o)); err != nil {
		return nil, err
	}
	go func() {
		result, err := k.run(value, o)
		message := ""
		if err != nil {
			message = err.Error()
		}
		if err := h.Notify(context.WithoutCancel(ctx), "kensaku#_settle", id, result, message); err != nil {
			log.Print(err)
		}
	}()
	return nil, nil
}
