package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
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
	defaultDomain = "https://project.feishu.cn"
	defaultServer = "feishu_project_readonly_mcp"
	serverPath    = "/mcp_server/v1"
	connectLimit  = 20 * time.Second
	requestLimit  = 2 * time.Minute
	sseRetries    = 2 // Match the legacy Node client's SSE recovery budget.
)

type config struct {
	endpoint *url.URL
	token    string
	ppe      string
	boe      string
	debug    bool
}

func parseConfig(args []string, output io.Writer) (config, error) {
	flags := flag.NewFlagSet("meego-mcp-go", flag.ContinueOnError)
	flags.SetOutput(output)
	domain := flags.String("domain", defaultDomain, "Feishu Project domain")
	configPath := flags.String("config", "", "Read credentials from this Codex TOML file (default: ~/.codex/config.toml)")
	credentialServer := flags.String("credential-server", defaultServer, "MCP server section containing the existing Feishu credentials")
	lang := flags.String("lang", "", "Response language, zh or en")
	debug := flags.Bool("debug", false, "Print connection status to stderr")
	if err := flags.Parse(args); err != nil {
		return config{}, err
	}
	if flags.NArg() != 0 {
		return config{}, errors.New("unexpected positional arguments")
	}
	if *lang != "" && *lang != "zh" && *lang != "en" {
		return config{}, errors.New("--lang must be zh or en")
	}
	endpoint, err := url.Parse(*domain)
	if err != nil || endpoint.Scheme != "https" || endpoint.Hostname() == "" ||
		endpoint.User != nil || endpoint.RawQuery != "" || endpoint.ForceQuery ||
		endpoint.Fragment != "" || (endpoint.Path != "" && endpoint.Path != "/") {
		return config{}, errors.New("--domain must be an HTTPS origin, without credentials, path, query or fragment")
	}
	endpoint.Path = serverPath
	endpoint.RawPath = ""
	if *lang != "" {
		endpoint.RawQuery = url.Values{"lang": {*lang}}.Encode()
	}
	if *configPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return config{}, errors.New("could not locate Codex config; specify --config")
		}
		*configPath = filepath.Join(home, ".codex", "config.toml")
	}
	credentials, err := readCredentials(*configPath, *credentialServer)
	if err != nil {
		return config{}, err
	}
	return config{
		endpoint: endpoint,
		token:    credentials["MCP_USER_TOKEN"],
		ppe:      credentials["MCP_PPE_ENV"],
		boe:      credentials["MCP_BOE_ENV"],
		debug:    *debug,
	}, nil
}

type codexConfig struct {
	Servers map[string]struct {
		Env map[string]string `toml:"env"`
	} `toml:"mcp_servers"`
}

func readCredentials(path, server string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read Codex config: %w", err)
	}
	var file codexConfig
	if err := toml.Unmarshal(data, &file); err != nil {
		// TOML errors can include source text containing credentials.
		return nil, errors.New("could not parse Codex config as TOML")
	}
	credentials := file.Servers[server].Env
	if credentials["MCP_USER_TOKEN"] == "" {
		return nil, fmt.Errorf("MCP_USER_TOKEN is missing from [mcp_servers.%s.env] in the Codex config", server)
	}
	return credentials, nil
}

type upstreamTransport struct {
	config
	base http.RoundTripper
}

func (t upstreamTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != t.endpoint.Scheme || req.URL.Host != t.endpoint.Host {
		return nil, errors.New("upstream request changed origin")
	}

	authenticated := req.Clone(req.Context())
	// The current CLI sends this literal header name on the wire.
	authenticated.Header.Set("HeaderKeyMCPConnectionType", "stdio")
	authenticated.Header.Set("X-Mcp-Token", t.token)
	if t.ppe != "" {
		authenticated.Header.Set("X-Tt-Env", t.ppe)
		authenticated.Header.Set("X-Use-Ppe", "1")
	}
	if t.boe != "" {
		authenticated.Header.Set("X-Tt-Env", t.boe)
	}
	return t.base.RoundTrip(authenticated)
}

func openUpstream(ctx context.Context, cfg config, base http.RoundTripper) (*mcp.ClientSession, error) {
	client := mcp.NewClient(&mcp.Implementation{Name: "lark-project-mcp", Version: "0.0.1"}, &mcp.ClientOptions{
		Capabilities: &mcp.ClientCapabilities{Sampling: &mcp.SamplingCapabilities{}},
		Logger:       slog.New(slog.DiscardHandler),
	})
	for _, method := range toolMethods {
		if err := mcp.AddSendingCustomMethod[*rawParams, *rawResult](client, proxyMethodPrefix+method); err != nil {
			return nil, err
		}
	}
	transport := &mcp.StreamableClientTransport{
		Endpoint: cfg.endpoint.String(),
		HTTPClient: &http.Client{
			Transport: upstreamTransport{config: cfg, base: base},
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
	server := mcp.NewServer(&mcp.Implementation{Name: "meego-mcp-go", Version: "1.0.0"}, &mcp.ServerOptions{
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

func run(ctx context.Context, cfg config, transport mcp.Transport, stderr io.Writer) error {
	connectCtx, cancel := context.WithTimeout(ctx, connectLimit)
	upstream, err := openUpstream(connectCtx, cfg, http.DefaultTransport)
	cancel()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return errors.New("connection to Feishu Project MCP timed out")
		}
		return errors.New("could not connect to Feishu Project MCP; check the domain, network and MCP_USER_TOKEN")
	}
	defer upstream.Close()
	if cfg.debug {
		fmt.Fprintln(stderr, "connected to Feishu Project MCP")
	}
	server, err := newProxy(upstream)
	if err != nil {
		return err
	}
	return server.Run(ctx, methodTransport{Transport: transport, local: true})
}

func main() {
	cfg, err := parseConfig(os.Args[1:], os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err == nil {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		err = run(ctx, cfg, &mcp.StdioTransport{MaxLineLength: -1}, os.Stderr)
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "meego-mcp-go:", err)
		os.Exit(1)
	}
}
