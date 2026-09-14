package xredis

// Option configures xredis-specific client behavior.
type Option interface {
	apply(opts *options)
}

type optionFunc func(opts *options)

func (f optionFunc) apply(opts *options) {
	f(opts)
}

type options struct {
	codec   Codec
	metrics Metrics
	tracing Tracing
}

func newOptions(opts ...Option) options {
	options := options{
		codec: JSONCodec{},
	}

	for _, opt := range opts {
		if opt != nil {
			opt.apply(&options)
		}
	}

	return options
}

// WithCodec configures value codec.
func WithCodec(codec Codec) Option {
	return optionFunc(func(opts *options) {
		if codec != nil {
			opts.codec = codec
		}
	})
}

// WithMetrics configures wrapper-level client metrics.
func WithMetrics(metrics Metrics) Option {
	return optionFunc(func(opts *options) {
		if metrics != nil {
			opts.metrics = metrics
		}
	})
}

// WithTracing configures client tracing instrumentation.
func WithTracing(tracing Tracing) Option {
	return optionFunc(func(opts *options) {
		if tracing != nil {
			opts.tracing = tracing
		}
	})
}
