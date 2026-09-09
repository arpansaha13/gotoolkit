package nats

import (
	"fmt"

	"github.com/arpansaha13/gotoolkit/gtk"
)

// NATSOption configures NewNATSClient.
type NATSOption interface {
	applyNATS(*natsConfig)
}

type natsConfig struct {
	shared gtk.Shared
}

type natsOptionFunc func(*natsConfig)

func (f natsOptionFunc) applyNATS(c *natsConfig) { f(c) }

func applyNATSOptions(opts []any) natsConfig {
	cfg := natsConfig{shared: gtk.DefaultShared()}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		switch v := opt.(type) {
		case gtk.Option:
			v.Apply(&cfg.shared)
		case NATSOption:
			v.applyNATS(&cfg)
		default:
			panic(fmt.Sprintf("nats: unsupported option %T", opt))
		}
	}
	gtk.Finalize(&cfg.shared)
	return cfg
}
