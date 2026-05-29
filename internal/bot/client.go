package bot

import (
	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/richaardev/concord"
)

type BotClient struct {
	*bot.Client

	InteractionManager concord.InteractionManager
	InteractionWorker  concord.InteractionWorker
}

type clientImplOptions struct {
	disgoOptions []bot.ConfigOpt
}

type ClientOption func(*clientImplOptions)

func NewClient(token string, opts ...ClientOption) (*BotClient, error) {
	manager := concord.NewInteractionManager()
	worker := concord.NewInteractionWorker(manager, 32)

	opt := applyOptions(opts...)
	base, err := disgo.New(token, opt.disgoOptions...)
	if err != nil {
		return nil, err
	}

	base.AddEventListeners(worker.Listeners()...)

	c := &BotClient{
		Client:             base,
		InteractionManager: manager,
		InteractionWorker:  worker,
	}

	return c, err
}

func WithDisgoOpts(opts ...bot.ConfigOpt) ClientOption {
	return func(c *clientImplOptions) {
		c.disgoOptions = opts
	}
}

func applyOptions(opts ...ClientOption) *clientImplOptions {
	options := &clientImplOptions{}

	for _, opt := range opts {
		opt(options)
	}

	return options
}
