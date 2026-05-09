package orchestrator

import "github.com/gravitl/netmaker/models"

type Options struct {
	useKey                bool
	key                   *models.EnrollmentKey
	skipPublishPeerUpdate bool
	relayedClients        []string
	isInternetGateway     bool
	igwClients            []string
}

type Option func(options *Options) *Options

func applyOptions(opts ...Option) *Options {
	o := &Options{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func UseKey(key *models.EnrollmentKey) Option {
	return func(o *Options) *Options {
		o.useKey = true
		o.key = key
		return o
	}
}

func SkipPublishPeerUpdate() Option {
	return func(o *Options) *Options {
		o.skipPublishPeerUpdate = true
		return o
	}
}

func WithRelayedClients(relayedClients []string) Option {
	return func(o *Options) *Options {
		o.relayedClients = relayedClients
		return o
	}
}

func WithInternetGateway(igwClients []string) Option {
	return func(o *Options) *Options {
		o.isInternetGateway = true
		o.igwClients = igwClients
		return o
	}
}
