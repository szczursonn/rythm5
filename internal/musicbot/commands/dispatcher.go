package commands

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/events"
)

const errPrefix = "musicbot/commands: "

type Dispatcher struct {
	logger        *slog.Logger
	client        *bot.Client
	classicPrefix string
	eventListener bot.EventListener

	classicByAlias map[string]Command
	slashByName    map[string]Command

	handlersWg sync.WaitGroup
	ctx        context.Context
	cancelCtx  context.CancelFunc
}

type DispatcherOptions struct {
	Logger        *slog.Logger
	Client        *bot.Client
	ClassicPrefix string
	Commands      []Command
}

func NewDispatcher(opts DispatcherOptions) *Dispatcher {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Client == nil {
		panic(errPrefix + "client must not be nil")
	}
	if opts.ClassicPrefix == "" {
		opts.ClassicPrefix = "!"
	}

	d := &Dispatcher{
		logger:        opts.Logger,
		client:        opts.Client,
		classicPrefix: opts.ClassicPrefix,

		classicByAlias: make(map[string]Command),
		slashByName:    make(map[string]Command),
	}
	d.ctx, d.cancelCtx = context.WithCancel(context.Background())
	d.eventListener = &events.ListenerAdapter{
		OnMessageCreate:                 d.handleMessageCreateEvent,
		OnApplicationCommandInteraction: d.handleApplicationCommandInteractionEvent,
	}

	for _, cmd := range opts.Commands {
		for _, alias := range cmd.ClassicAliases() {
			d.classicByAlias[alias] = cmd
		}

		if def := cmd.SlashDef(); def != nil {
			d.slashByName[def.Name] = cmd
		}
	}

	d.client.AddEventListeners(d.eventListener)

	return d
}

func (d *Dispatcher) Stop() {
	d.client.RemoveEventListeners(d.eventListener)
	d.cancelCtx()
	time.Sleep(50 * time.Millisecond)
	d.handlersWg.Wait()
}
