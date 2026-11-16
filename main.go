package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pelletier/go-toml/v2"
)

const (
	version          = "2.0.0"
	upstreamHost     = "project.feishu.cn"
	upstreamEndpoint = "https://" + upstreamHost + "/mcp_server/v1"
	connectLimit     = 20 * time.Second
	requestLimit     = 2 * time.Minute
	sseRetries       = 2
)

type codexConfig struct {
	Servers struct {
		FeishuProject struct {
			Env struct {
				Token string `toml:"MCP_USER_TOKEN"`
			} `toml:"env"`
		} `toml:"feishu_project_mcp"`
	} `toml:"mcp_servers"`
}

func readToken() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.New("could not locate ~/.codex/config.toml")
	}
	data, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		return "", fmt.Errorf("read Codex config: %w", err)
	}
	var file codexConfig
	if err := toml.Unmarshal(data, &file); err != nil {
		// TOML errors can include source text containing credentials.
		return "", errors.New("could not parse Codex config as TOML")
	}
	token := file.Servers.FeishuProject.Env.Token
	if token == "" {
		return "", errors.New("MCP_USER_TOKEN is missing from [mcp_servers.feishu_project_mcp.env] in the Codex config")
	}
	return token, nil
}

type upstreamTransport struct {
	token string
	base  http.RoundTripper
}

func (t upstreamTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "https" || req.URL.Host != upstreamHost {
		return nil, errors.New("upstream request changed origin")
	}

	authenticated := req.Clone(req.Context())
	authenticated.Header.Set("X-Mcp-Token", t.token)
	return t.base.RoundTrip(authenticated)
}

func openUpstream(ctx context.Context, token string, base http.RoundTripper) (*mcp.ClientSession, error) {
	client := mcp.NewClient(&mcp.Implementation{Name: "meego-mcp-go", Version: version}, &mcp.ClientOptions{
		Logger: slog.New(slog.DiscardHandler),
	})
	for _, method := range toolMethods {
		if err := mcp.AddSendingCustomMethod[*rawParams, *rawResult](client, proxyMethodPrefix+method); err != nil {
			return nil, err
		}
	}
	transport := &mcp.StreamableClientTransport{
		Endpoint: upstreamEndpoint,
		HTTPClient: &http.Client{
			Transport: upstreamTransport{token: token, base: base},
		},
		MaxRetries:   sseRetries,
		MaxEventSize: -1,
	}
	return client.Connect(ctx, methodTransport{Transport: transport}, &mcp.ClientSessionOptions{ProtocolVersion: "2025-11-25"})
}

const proxyMethodPrefix = "local-raw/"

var toolMethods = [...]string{"tools/list", "tools/call"}

type rawParams struct {
	mcp.ParamsBase
	json.RawMessage
}

type rawResult struct {
	mcp.ResultBase
	json.RawMessage
}

// Register internal aliases so the SDK handles protocol lifecycle and
// errors while the proxy preserves the original JSON for forwarded tools.
func newProxy(upstream *mcp.ClientSession) (*mcp.Server, error) {
	server := mcp.NewServer(&mcp.Implementation{Name: "meego-mcp-go", Version: version}, &mcp.ServerOptions{
		Capabilities: &mcp.ServerCapabilities{Tools: &mcp.ToolCapabilities{}},
		Logger:       slog.New(slog.DiscardHandler),
	})
	for _, method := range toolMethods {
		alias := proxyMethodPrefix + method
		err := mcp.AddReceivingCustomMethod(server, alias,
			func(ctx context.Context, _ *mcp.ServerSession, params *rawParams) (*rawResult, error) {
				if params != nil && len(params.RawMessage) == 0 {
					params = nil
				}
				ctx, cancel := context.WithTimeout(ctx, requestLimit)
				defer cancel()
				result, err := mcp.CallCustomMethod[*rawParams, *rawResult](ctx, upstream, alias, params)
				return result, upstreamError(err)
			})
		if err != nil {
			return nil, err
		}
	}
	return server, nil
}

type methodTransport struct {
	mcp.Transport
	local bool
}

func (t methodTransport) Connect(ctx context.Context) (mcp.Connection, error) {
	conn, err := t.Transport.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return methodConnection{Connection: conn, local: t.local}, nil
}

type methodConnection struct {
	mcp.Connection
	local bool
}

func (c methodConnection) Read(ctx context.Context) (jsonrpc.Message, error) {
	message, err := c.Connection.Read(ctx)
	if c.local && err == nil {
		message = translateToolMethod(message, "", proxyMethodPrefix)
	}
	return message, err
}

func (c methodConnection) Write(ctx context.Context, message jsonrpc.Message) error {
	if !c.local {
		message = translateToolMethod(message, proxyMethodPrefix, "")
	}
	return c.Connection.Write(ctx, message)
}

func translateToolMethod(message jsonrpc.Message, fromPrefix, toPrefix string) jsonrpc.Message {
	request, ok := message.(*jsonrpc.Request)
	if !ok {
		return message
	}
	for _, method := range toolMethods {
		if request.Method == fromPrefix+method {
			renamed := *request
			renamed.Method = toPrefix + method
			return &renamed
		}
	}
	return message
}

func upstreamError(err error) error {
	var rpcErr *jsonrpc.Error
	switch {
	case err == nil:
		return nil
	case errors.As(err, &rpcErr):
		return rpcErr
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	default:
		return errors.New("upstream MCP request failed")
	}
}

func run(ctx context.Context, token string, transport mcp.Transport) error {
	connectCtx, cancel := context.WithTimeout(ctx, connectLimit)
	upstream, err := openUpstream(connectCtx, token, http.DefaultTransport)
	cancel()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return errors.New("connection to Feishu Project MCP timed out")
		}
		return errors.New("could not connect to Feishu Project MCP; check the network and MCP_USER_TOKEN")
	}
	defer upstream.Close()
	server, err := newProxy(upstream)
	if err != nil {
		return err
	}
	return server.Run(ctx, methodTransport{Transport: transport, local: true})
}

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "meego-mcp-go does not accept command-line arguments")
		os.Exit(1)
	}
	token, err := readToken()
	if err == nil {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		err = run(ctx, token, &mcp.StdioTransport{MaxLineLength: -1})
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "meego-mcp-go:", err)
		os.Exit(1)
	}
}
