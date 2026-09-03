package gtk

// NATSOption configures NewNATSClient.
type NATSOption interface {
	applyNATS(*natsConfig)
}

type natsConfig struct {
	shared sharedConfig
}

type natsOptionFunc func(*natsConfig)

func (f natsOptionFunc) applyNATS(c *natsConfig) { f(c) }

func applyNATSOptions(opts []NATSOption) natsConfig {
	cfg := natsConfig{shared: defaultShared()}
	for _, opt := range opts {
		if opt != nil {
			opt.applyNATS(&cfg)
		}
	}
	finalizeShared(&cfg.shared)
	return cfg
}
