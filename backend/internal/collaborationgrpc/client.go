package collaborationgrpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"agentroom/backend/internal/collaboration"
	collaborationruntimev1 "agentroom/backend/internal/collaborationproto/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

const healthServiceName = "agentroom.collaboration.v1.CollaborationRuntimeService"

var (
	ErrInvalidRequest   = errors.New("collaboration runtime request is invalid")
	ErrAuthentication   = errors.New("collaboration runtime authentication failed")
	ErrCapacity         = errors.New("collaboration runtime capacity exhausted")
	ErrUnavailable      = errors.New("collaboration runtime is unavailable")
	ErrProtocol         = errors.New("collaboration runtime protocol violation")
	ErrDuplicateRun     = errors.New("collaboration runtime run is already active")
	ErrRoomBusy         = errors.New("collaboration runtime room is busy")
	ErrInvalidTransport = errors.New("collaboration runtime transport configuration is invalid")
)

type ClientConfig struct {
	Address         string
	Insecure        bool
	ServerName      string
	CAFile          string
	ClientCertFile  string
	ClientKeyFile   string
	Timeout         time.Duration
	MaxRequestBytes int
	MaxEventBytes   int
}

func (c ClientConfig) Validate() error {
	if c.Address == "" {
		return fmt.Errorf("%w: address is required", ErrInvalidTransport)
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("%w: timeout must be positive", ErrInvalidTransport)
	}
	if c.MaxRequestBytes <= 0 || c.MaxEventBytes <= 0 {
		return fmt.Errorf("%w: message limits must be positive", ErrInvalidTransport)
	}
	if c.Insecure {
		return nil
	}
	if c.CAFile == "" {
		return fmt.Errorf("%w: CA file is required unless insecure mode is explicit", ErrInvalidTransport)
	}
	if (c.ClientCertFile == "") != (c.ClientKeyFile == "") {
		return fmt.Errorf("%w: client certificate and key must be configured together", ErrInvalidTransport)
	}
	return nil
}

type Client struct {
	conn            *grpc.ClientConn
	client          collaborationruntimev1.CollaborationRuntimeServiceClient
	defaultTimeout  time.Duration
	maxRequestBytes uint32
	maxEventBytes   uint32
}

var _ collaboration.CollaborationRuntime = (*Client)(nil)
var _ collaboration.CapabilityProvider = (*Client)(nil)

func NewClient(config ClientConfig) (*Client, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	transportCredentials, err := transportCredentials(config)
	if err != nil {
		return nil, err
	}
	conn, err := grpc.NewClient(
		config.Address,
		grpc.WithTransportCredentials(transportCredentials),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallSendMsgSize(config.MaxRequestBytes),
			grpc.MaxCallRecvMsgSize(config.MaxEventBytes),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: create gRPC client", ErrUnavailable)
	}
	return &Client{
		conn:            conn,
		client:          collaborationruntimev1.NewCollaborationRuntimeServiceClient(conn),
		defaultTimeout:  config.Timeout,
		maxRequestBytes: uint32(config.MaxRequestBytes),
		maxEventBytes:   uint32(config.MaxEventBytes),
	}, nil
}

func (c *Client) Ready(ctx context.Context) error {
	response, err := grpc_health_v1.NewHealthClient(c.conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{
		Service: healthServiceName,
	})
	if err != nil {
		return mapGRPCError(ctx, err)
	}
	if response.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
		return ErrUnavailable
	}
	return nil
}

func (c *Client) Capabilities(ctx context.Context) (collaboration.RuntimeCapabilities, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.defaultTimeout)
	defer cancel()
	response, err := c.client.GetCapabilities(callCtx, &collaborationruntimev1.GetCapabilitiesRequest{})
	if err != nil {
		return collaboration.RuntimeCapabilities{}, mapGRPCError(callCtx, err)
	}
	return mapCapabilities(response)
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) ExecuteConversation(ctx context.Context, request collaboration.Request) (collaboration.EventStream, error) {
	mapped, err := mapRequest(request)
	if err != nil {
		return nil, err
	}
	if err := collaborationruntimev1.ValidateRequestSize(mapped, c.maxRequestBytes); err != nil {
		return nil, ErrInvalidRequest
	}

	timeout := request.Snapshot.Limits.Timeout
	if timeout <= 0 {
		timeout = c.defaultTimeout
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	stream, err := c.client.ExecuteConversation(callCtx, mapped)
	if err != nil {
		mappedErr := mapGRPCError(callCtx, err)
		cancel()
		return nil, mappedErr
	}
	return &eventStream{ctx: callCtx, cancel: cancel, stream: stream, maxEventBytes: c.maxEventBytes}, nil
}

type eventStream struct {
	ctx           context.Context
	cancel        context.CancelFunc
	stream        collaborationruntimev1.CollaborationRuntimeService_ExecuteConversationClient
	maxEventBytes uint32
}

func (s *eventStream) Recv() (collaboration.Event, error) {
	event, err := s.stream.Recv()
	if errors.Is(err, io.EOF) {
		s.cancel()
		return collaboration.Event{}, io.EOF
	}
	if err != nil {
		mappedErr := mapGRPCError(s.ctx, err)
		s.cancel()
		return collaboration.Event{}, mappedErr
	}
	if err := collaborationruntimev1.ValidateEventSize(event, s.maxEventBytes); err != nil {
		s.cancel()
		return collaboration.Event{}, ErrProtocol
	}
	mapped, err := mapEvent(event)
	if err != nil {
		s.cancel()
		return collaboration.Event{}, err
	}
	if isTerminal(mapped.Kind) {
		s.cancel()
	}
	return mapped, nil
}

func isTerminal(kind collaboration.EventKind) bool {
	switch kind {
	case collaboration.EventCompleted, collaboration.EventStopped, collaboration.EventCancelled, collaboration.EventFailed:
		return true
	default:
		return false
	}
}

func mapCapabilities(response *collaborationruntimev1.GetCapabilitiesResponse) (collaboration.RuntimeCapabilities, error) {
	if response == nil || len(response.GetSupportedProtocolVersions()) == 0 {
		return collaboration.RuntimeCapabilities{}, ErrProtocol
	}
	result := collaboration.RuntimeCapabilities{
		Ready:                     response.GetReady(),
		SupportedProtocolVersions: append([]string(nil), response.GetSupportedProtocolVersions()...),
		Engines:                   make([]collaboration.EngineCapability, 0, len(response.GetEngines())),
		SupportedTriggerModes:     make([]collaboration.TriggerMode, 0, len(response.GetSupportedTriggerModes())),
	}
	for _, engine := range response.GetEngines() {
		if engine == nil {
			return collaboration.RuntimeCapabilities{}, ErrProtocol
		}
		mapped := collaboration.Engine(engine.GetEngine())
		if mapped != collaboration.EngineNative && mapped != collaboration.EngineAutoGen {
			return collaboration.RuntimeCapabilities{}, ErrProtocol
		}
		result.Engines = append(result.Engines, collaboration.EngineCapability{
			Engine: mapped, Version: engine.GetVersion(), Enabled: engine.GetEnabled(), Ready: engine.GetReady(),
		})
	}
	for _, mode := range response.GetSupportedTriggerModes() {
		mapped := collaboration.TriggerMode(mode)
		if mapped != collaboration.TriggerMentionOnly && mapped != collaboration.TriggerAutomatic {
			return collaboration.RuntimeCapabilities{}, ErrProtocol
		}
		result.SupportedTriggerModes = append(result.SupportedTriggerModes, mapped)
	}
	return result, nil
}

func mapGRPCError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	grpcStatus, ok := status.FromError(err)
	if !ok {
		return ErrUnavailable
	}
	switch grpcStatus.Code() {
	case codes.InvalidArgument, codes.Unimplemented:
		return ErrInvalidRequest
	case codes.Unauthenticated, codes.PermissionDenied:
		return ErrAuthentication
	case codes.ResourceExhausted:
		return ErrCapacity
	case codes.AlreadyExists:
		return ErrDuplicateRun
	case codes.FailedPrecondition:
		return ErrRoomBusy
	case codes.Canceled:
		return context.Canceled
	case codes.DeadlineExceeded:
		return context.DeadlineExceeded
	case codes.DataLoss, codes.Internal:
		return ErrProtocol
	case codes.Unavailable:
		return ErrUnavailable
	default:
		return ErrUnavailable
	}
}

func transportCredentials(config ClientConfig) (credentials.TransportCredentials, error) {
	if config.Insecure {
		return insecure.NewCredentials(), nil
	}
	caBytes, err := os.ReadFile(config.CAFile)
	if err != nil {
		return nil, fmt.Errorf("%w: read CA file", ErrInvalidTransport)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caBytes) {
		return nil, fmt.Errorf("%w: CA file contains no certificates", ErrInvalidTransport)
	}
	tlsConfig := &tls.Config{RootCAs: roots, ServerName: config.ServerName, MinVersion: tls.VersionTLS12}
	if config.ClientCertFile != "" {
		certificate, err := tls.LoadX509KeyPair(config.ClientCertFile, config.ClientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("%w: load client identity", ErrInvalidTransport)
		}
		tlsConfig.Certificates = []tls.Certificate{certificate}
	}
	return credentials.NewTLS(tlsConfig), nil
}
