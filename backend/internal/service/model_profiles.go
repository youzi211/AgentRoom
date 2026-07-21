package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/store"
)

var (
	ErrInvalidModelProfile          = errors.New("invalid model profile")
	ErrModelNotConfigured           = errors.New("model is not configured")
	ErrModelProfileDisabled         = errors.New("model profile is disabled")
	ErrDefaultModelProfile          = errors.New("default model profile must be replaced before it can be disabled")
	ErrModelEncryptionNotConfigured = errors.New("model profile encryption is not configured")
)

type SecretCipher struct{ aead cipher.AEAD }

func NewSecretCipher(encodedKey string) (*SecretCipher, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encodedKey))
	if err != nil || len(key) != 32 {
		return nil, errors.New("MODEL_CONFIG_ENCRYPTION_KEY must be a base64-encoded 32-byte key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.New("initialize model secret cipher")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.New("initialize model secret cipher")
	}
	return &SecretCipher{aead: aead}, nil
}

func (c *SecretCipher) Encrypt(profileID, plaintext string) (string, error) {
	if c == nil || c.aead == nil {
		return "", ErrModelEncryptionNotConfigured
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", errors.New("encrypt model credential")
	}
	sealed := c.aead.Seal(nil, nonce, []byte(plaintext), []byte(profileID))
	return "v1:" + base64.RawStdEncoding.EncodeToString(append(nonce, sealed...)), nil
}

func (c *SecretCipher) Decrypt(profileID, envelope string) (string, error) {
	if c == nil || c.aead == nil {
		return "", ErrModelEncryptionNotConfigured
	}
	if !strings.HasPrefix(envelope, "v1:") {
		return "", errors.New("unsupported model credential version")
	}
	payload, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(envelope, "v1:"))
	if err != nil || len(payload) < c.aead.NonceSize() {
		return "", errors.New("invalid model credential")
	}
	plain, err := c.aead.Open(nil, payload[:c.aead.NonceSize()], payload[c.aead.NonceSize():], []byte(profileID))
	if err != nil {
		return "", errors.New("decrypt model credential")
	}
	return string(plain), nil
}

type modelProfileStore interface {
	ListModelProfiles(context.Context) ([]model.ModelProfile, error)
	GetModelProfile(context.Context, string) (model.ModelProfile, error)
	GetDefaultModelProfile(context.Context, string) (model.ModelProfile, error)
	CreateModelProfile(context.Context, model.ModelProfile) (model.ModelProfile, error)
	UpdateModelProfile(context.Context, model.ModelProfile) (model.ModelProfile, error)
	SetDefaultModelProfile(context.Context, string) error
	CountModelProfileReferences(context.Context, string) (int64, error)
	DeleteModelProfile(context.Context, string) error
}

type CreateModelProfileInput struct {
	Name, RuntimeScope, Protocol, BaseURL, ModelName, APIKey string
	Enabled, IsDefault                                       bool
}
type UpdateModelProfileInput struct {
	Name, BaseURL, ModelName string
	APIKey                   *string
	Enabled                  *bool
	ClearAPIKey              bool
}
type ModelProfileService struct {
	store      modelProfileStore
	cipher     *SecretCipher
	httpClient *http.Client
}

func NewModelProfileService(s modelProfileStore, cipher *SecretCipher, client any) *ModelProfileService {
	httpClient, _ := client.(*http.Client)
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &ModelProfileService{store: s, cipher: cipher, httpClient: httpClient}
}

func normalizeBaseURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return "", fmt.Errorf("%w: valid API base URL is required", ErrInvalidModelProfile)
	}
	u.RawQuery = ""
	u.Fragment = ""
	u.Path = strings.TrimRight(u.Path, "/")
	u.Path = strings.TrimSuffix(u.Path, "/chat/completions")
	u.Path = strings.TrimRight(u.Path, "/")
	if !strings.HasSuffix(u.Path, "/v1") {
		u.Path += "/v1"
	}
	return strings.TrimSuffix(u.String(), "/"), nil
}
func redactProfile(p model.ModelProfile) model.ModelProfile {
	p.HasAPIKey = p.APIKeyCiphertext != ""
	p.APIKeyCiphertext = ""
	return p
}
func keyHint(key string) string {
	r := []rune(key)
	if len(r) <= 4 {
		return "..." + string(r)
	}
	return "..." + string(r[len(r)-4:])
}
func validateScope(scope string) bool {
	return scope == model.ModelRuntimeGo || scope == model.ModelRuntimeDeepAgent
}

func (s *ModelProfileService) Create(ctx context.Context, in CreateModelProfileInput) (model.ModelProfile, error) {
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.ModelName) == "" || !validateScope(in.RuntimeScope) || in.Protocol != model.ModelProtocolOpenAIChatCompletions {
		return model.ModelProfile{}, ErrInvalidModelProfile
	}
	if in.IsDefault && !in.Enabled {
		return model.ModelProfile{}, fmt.Errorf("%w: default profile must be enabled", ErrInvalidModelProfile)
	}
	base, err := normalizeBaseURL(in.BaseURL)
	if err != nil {
		return model.ModelProfile{}, err
	}
	id := model.NewID("model")
	p := model.ModelProfile{ID: id, Name: strings.TrimSpace(in.Name), RuntimeScope: in.RuntimeScope, Protocol: in.Protocol, BaseURL: base, ModelName: strings.TrimSpace(in.ModelName), Enabled: in.Enabled, IsDefault: in.IsDefault, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if in.APIKey != "" {
		p.APIKeyCiphertext, err = s.cipher.Encrypt(id, in.APIKey)
		if err != nil {
			return model.ModelProfile{}, err
		}
		p.APIKeyHint = keyHint(in.APIKey)
	}
	if p.Enabled && !p.IsDefault {
		if _, e := s.store.GetDefaultModelProfile(ctx, p.RuntimeScope); errors.Is(e, store.ErrModelProfileNotFound) {
			p.IsDefault = true
		}
	}
	created, err := s.store.CreateModelProfile(ctx, p)
	if err != nil {
		return model.ModelProfile{}, err
	}
	return redactProfile(created), nil
}
func (s *ModelProfileService) List(ctx context.Context) ([]model.ModelProfile, error) {
	p, err := s.store.ListModelProfiles(ctx)
	for i := range p {
		p[i] = redactProfile(p[i])
	}
	return p, err
}
func (s *ModelProfileService) Get(ctx context.Context, id string) (model.ModelProfile, error) {
	p, e := s.store.GetModelProfile(ctx, id)
	return redactProfile(p), e
}

func (s *ModelProfileService) Update(ctx context.Context, id string, in UpdateModelProfileInput) (model.ModelProfile, error) {
	if strings.TrimSpace(id) == "" || (in.ClearAPIKey && in.APIKey != nil) {
		return model.ModelProfile{}, ErrInvalidModelProfile
	}
	p, err := s.store.GetModelProfile(ctx, id)
	if err != nil {
		return model.ModelProfile{}, err
	}
	if strings.TrimSpace(in.Name) != "" {
		p.Name = strings.TrimSpace(in.Name)
	}
	if strings.TrimSpace(in.BaseURL) != "" {
		p.BaseURL, err = normalizeBaseURL(in.BaseURL)
		if err != nil {
			return model.ModelProfile{}, err
		}
	}
	if strings.TrimSpace(in.ModelName) != "" {
		p.ModelName = strings.TrimSpace(in.ModelName)
	}
	if in.ClearAPIKey {
		p.APIKeyCiphertext, p.APIKeyHint = "", ""
	} else if in.APIKey != nil {
		newKey := strings.TrimSpace(*in.APIKey)
		if newKey == "" {
			return model.ModelProfile{}, fmt.Errorf("%w: replacement API key cannot be empty; use clearAPIKey to clear it", ErrInvalidModelProfile)
		}
		p.APIKeyCiphertext, err = s.cipher.Encrypt(p.ID, newKey)
		if err != nil {
			return model.ModelProfile{}, err
		}
		p.APIKeyHint = keyHint(newKey)
	}
	if in.Enabled != nil {
		if !*in.Enabled && p.IsDefault {
			return model.ModelProfile{}, ErrDefaultModelProfile
		}
		p.Enabled = *in.Enabled
	}
	p.UpdatedAt = time.Now().UTC()
	updated, err := s.store.UpdateModelProfile(ctx, p)
	if err != nil {
		return model.ModelProfile{}, err
	}
	return redactProfile(updated), nil
}
func (s *ModelProfileService) SetDefault(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrInvalidModelProfile
	}
	p, e := s.store.GetModelProfile(ctx, id)
	if e != nil {
		return e
	}
	if !p.Enabled {
		return ErrModelProfileDisabled
	}
	return s.store.SetDefaultModelProfile(ctx, id)
}
func (s *ModelProfileService) Delete(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrInvalidModelProfile
	}
	p, e := s.store.GetModelProfile(ctx, id)
	if e != nil {
		return e
	}
	refs, e := s.store.CountModelProfileReferences(ctx, id)
	if e != nil {
		return e
	}
	if p.IsDefault || refs > 0 {
		return store.ErrModelProfileReferenced
	}
	return s.store.DeleteModelProfile(ctx, id)
}

type TestModelProfileInput struct{ ProfileID, BaseURL, ModelName, APIKey string }
type ModelConnectionResult struct {
	OK        bool   `json:"ok"`
	LatencyMS int64  `json:"latencyMS"`
	Model     string `json:"model,omitempty"`
	Error     string `json:"error,omitempty"`
}

func (s *ModelProfileService) TestConnection(ctx context.Context, in TestModelProfileInput) (ModelConnectionResult, error) {
	if in.ProfileID != "" {
		p, err := s.store.GetModelProfile(ctx, in.ProfileID)
		if err != nil {
			return ModelConnectionResult{}, err
		}
		if !p.Enabled {
			return ModelConnectionResult{}, ErrModelProfileDisabled
		}
		in.BaseURL, in.ModelName = p.BaseURL, p.ModelName
		if in.APIKey == "" && p.APIKeyCiphertext != "" {
			in.APIKey, err = s.cipher.Decrypt(p.ID, p.APIKeyCiphertext)
			if err != nil {
				return ModelConnectionResult{}, err
			}
		}
	}
	base, err := normalizeBaseURL(in.BaseURL)
	if err != nil {
		return ModelConnectionResult{}, err
	}
	if strings.TrimSpace(in.ModelName) == "" {
		return ModelConnectionResult{}, ErrInvalidModelProfile
	}
	body, _ := json.Marshal(map[string]any{"model": in.ModelName, "messages": []map[string]string{{"role": "user", "content": "Reply with OK."}}, "max_tokens": 1})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/chat/completions", strings.NewReader(string(body)))
	if err != nil {
		return ModelConnectionResult{}, ErrInvalidModelProfile
	}
	req.Header.Set("Content-Type", "application/json")
	if in.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+in.APIKey)
	}
	start := time.Now()
	resp, err := s.httpClient.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		message := "connection failed"
		var networkError net.Error
		if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &networkError) && networkError.Timeout()) {
			message = "request timed out"
		}
		return ModelConnectionResult{OK: false, LatencyMS: latency, Error: message}, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := "model request failed"
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			message = "authentication failed"
		}
		return ModelConnectionResult{OK: false, LatencyMS: latency, Error: message}, nil
	}
	var payload struct {
		Model string `json:"model"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload)
	responseModel := strings.TrimSpace(payload.Model)
	if in.APIKey != "" && strings.Contains(responseModel, in.APIKey) {
		responseModel = ""
	}
	if len(responseModel) > 255 {
		responseModel = responseModel[:255]
	}
	return ModelConnectionResult{OK: true, LatencyMS: latency, Model: responseModel}, nil
}

type EnvironmentModelConfig struct{ BaseURL, ModelName, APIKey string }
type ResolvedModelConfig = model.ResolvedModelConfig
type ModelResolver struct {
	store       modelProfileStore
	cipher      *SecretCipher
	environment map[string]EnvironmentModelConfig
}

func NewModelResolver(s modelProfileStore, c *SecretCipher, env map[string]EnvironmentModelConfig) *ModelResolver {
	return &ModelResolver{store: s, cipher: c, environment: env}
}
func (r *ModelResolver) Resolve(ctx context.Context, scope, explicitID string) (ResolvedModelConfig, error) {
	var p model.ModelProfile
	var err error
	if explicitID != "" {
		p, err = r.store.GetModelProfile(ctx, explicitID)
		if err != nil {
			return ResolvedModelConfig{}, fmt.Errorf("explicit model profile: %w", err)
		}
		if p.RuntimeScope != scope {
			return ResolvedModelConfig{}, fmt.Errorf("%w: runtime scope mismatch", ErrInvalidModelProfile)
		}
		if !p.Enabled {
			return ResolvedModelConfig{}, ErrModelProfileDisabled
		}
	} else {
		p, err = r.store.GetDefaultModelProfile(ctx, scope)
		if err != nil && !errors.Is(err, store.ErrModelProfileNotFound) {
			return ResolvedModelConfig{}, err
		}
		if errors.Is(err, store.ErrModelProfileNotFound) {
			e := r.environment[scope]
			if e.BaseURL == "" || e.ModelName == "" {
				return ResolvedModelConfig{}, ErrModelNotConfigured
			}
			return ResolvedModelConfig{Source: "environment", BaseURL: e.BaseURL, ModelName: e.ModelName, APIKey: e.APIKey}, nil
		}
		if !p.Enabled {
			return ResolvedModelConfig{}, ErrModelProfileDisabled
		}
	}
	key := ""
	if p.APIKeyCiphertext != "" {
		key, err = r.cipher.Decrypt(p.ID, p.APIKeyCiphertext)
		if err != nil {
			return ResolvedModelConfig{}, err
		}
	}
	return ResolvedModelConfig{ProfileID: p.ID, Source: "database", BaseURL: p.BaseURL, ModelName: p.ModelName, APIKey: key}, nil
}
