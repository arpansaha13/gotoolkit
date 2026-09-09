package nats

import (
	"fmt"

	"github.com/arpansaha13/gotoolkit/gtk"
)

// Option configures NewClient.
type Option interface {
	apply(*natsConfig)
}

type natsConfig struct {
	shared gtk.Shared
}

type natsOptionFunc func(*natsConfig)

func (f natsOptionFunc) apply(c *natsConfig) { f(c) }

func applyOptions(opts []any) natsConfig {
	cfg := natsConfig{shared: gtk.DefaultShared()}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		switch v := opt.(type) {
		case gtk.Option:
			v.Apply(&cfg.shared)
		case Option:
			v.apply(&cfg)
		default:
			panic(fmt.Sprintf("nats: unsupported option %T", opt))
		}
	}
	gtk.Finalize(&cfg.shared)
	return cfg
}
