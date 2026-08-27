package gtk

// ManagedClient is a long-lived dependency that connects on Start and
// releases resources on Stop.
type ManagedClient interface {
	Start() error
	Stop() error
}
