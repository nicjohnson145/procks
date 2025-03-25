package client

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	"github.com/nicjohnson145/hlp"
	pbv1 "github.com/nicjohnson145/procks/gen/procks/v1"
	"github.com/nicjohnson145/procks/gen/procks/v1/procksv1connect"
	"github.com/rs/zerolog"
	"github.com/carlmjohnson/requests"
)

type ClientConfig struct {
	Logger zerolog.Logger
	Url    string
}

func NewClient(conf ClientConfig) *Client {
	return &Client{
		log:    conf.Logger,
		client: procksv1connect.NewProcksServiceClient(http.DefaultClient, conf.Url),
	}
}

type Client struct {
	log    zerolog.Logger
	client procksv1connect.ProcksServiceClient
}

type ProxyOpts struct {
	ID   string
	Port string
}

func (c *Client) logAndHandleError(err error, msg string) error {
	c.log.Err(err).Msg(msg)
	return fmt.Errorf("%v: %w", msg, err)
}

func (c *Client) Proxy(ctx context.Context, opts ProxyOpts) error {
	req := &pbv1.ConnectRequest{}
	if opts.ID != "" {
		req.Id = hlp.Ptr(opts.ID)
	}

	stream, err := c.client.Connect(ctx, connect.NewRequest(req))
	if err != nil {
		return c.logAndHandleError(err, "error issuing connect request")
	}

	for stream.Receive() {
		msg := stream.Msg()
		switch concrete := msg.Event.Message.(type) {
		case *pbv1.Event_ConnectionEstablished:
			c.log.Info().Msgf("connection established with proxy url of %v", concrete.ConnectionEstablished.Url)
			continue
		case *pbv1.Event_RequestRecieved:
			c.log.Info().Msg("forwarding request")
			if err := c.forwardRequest(ctx, opts, concrete.RequestRecieved); err != nil {
				c.logAndHandleError(err, "error forwarding request")
			}
		default:
			return c.logAndHandleError(fmt.Errorf("unhandled message type of %T", msg.Event.Message), "error during message processing")
		}
	}
	if err := stream.Err(); err != nil {
		return c.logAndHandleError(err, "error during stream processing")
	}

	return nil
}

func (c *Client) forwardRequest(ctx context.Context, opts ProxyOpts, msg *pbv1.Event_RequestRecievedEvent) error {
	builder := requests.
		URL(fmt.Sprintf("http://localhost:%v", opts.Port)).
		Path(msg.Path)

	// Set headers
	for key, value := range msg.Headers {
		builder = builder.Header(key, value)
	}

	// TODO: query params

	if err := builder.Fetch(ctx); err != nil {
		return fmt.Errorf("error executing request: %w", err)
	}

	return nil
}
