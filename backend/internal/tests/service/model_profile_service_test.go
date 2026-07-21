package service_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agentroom/backend/internal/model"
	"agentroom/backend/internal/service"
	"agentroom/backend/internal/store"
	"agentroom/backend/internal/tests/teststore"
)

func testEncryptionKey() string {
	return base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
}

func TestSecretCipherUsesRandomNonceAndProfileAAD(t *testing.T) {
	cipher, err := service.NewSecretCipher(testEncryptionKey())
	if err != nil {
		t.Fatal(err)
	}
	first, err := cipher.Encrypt("profile-a", "secret-value")
	if err != nil {
		t.Fatal(err)
	}
	second, err := cipher.Encrypt("profile-a", "secret-value")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("ciphertexts must differ because each encryption uses a random nonce")
	}
	plain, err := cipher.Decrypt("profile-a", first)
	if err != nil || plain != "secret-value" {
		t.Fatalf("decrypt = %q, %v", plain, err)
	}
	if _, err := cipher.Decrypt("profile-b", first); err == nil {
		t.Fatal("decrypting with another profile id must fail")
	}
	if strings.Contains(first, "secret-value") {
		t.Fatal("ciphertext leaked plaintext")
	}
}

func TestModelProfileServiceCreatesFirstEnabledProfileAsDefaultAndRedactsSecret(t *testing.T) {
	store := &teststore.Store{}
	cipher, err := service.NewSecretCipher(testEncryptionKey())
	if err != nil {
		t.Fatal(err)
	}
	svc := service.NewModelProfileService(store, cipher, nil)

	created, err := svc.Create(context.Background(), service.CreateModelProfileInput{
		Name: "Primary", RuntimeScope: model.ModelRuntimeGo,
		Protocol: model.ModelProtocolOpenAIChatCompletions,
		BaseURL:  "https://example.com/", ModelName: "example-model", APIKey: "sk-private", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created.IsDefault || !created.HasAPIKey || created.APIKeyHint != "...vate" {
		t.Fatalf("unexpected public profile: %+v", created)
	}
	if strings.Contains(created.APIKeyCiphertext, "sk-private") || created.APIKeyCiphertext != "" {
		t.Fatal("public response must not expose ciphertext or plaintext")
	}
	if store.ModelProfiles[0].BaseURL != "https://example.com/v1" {
		t.Fatalf("normalized base url = %q", store.ModelProfiles[0].BaseURL)
	}
}

func TestSecretCipherRejectsWrongKeyAndTamperingWithoutLeakingSecret(t *testing.T) {
	secret := "sk-never-log-this-value"
	cipher, err := service.NewSecretCipher(testEncryptionKey())
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := cipher.Encrypt("profile-a", secret)
	if err != nil {
		t.Fatal(err)
	}

	otherKey := base64.StdEncoding.EncodeToString([]byte("abcdef0123456789abcdef0123456789"))
	otherCipher, err := service.NewSecretCipher(otherKey)
	if err != nil {
		t.Fatal(err)
	}
	assertSafeDecryptFailure(t, otherCipher, "profile-a", envelope, secret)

	tampered := envelope[:len(envelope)-1]
	if envelope[len(envelope)-1] == 'A' {
		tampered += "B"
	} else {
		tampered += "A"
	}
	assertSafeDecryptFailure(t, cipher, "profile-a", tampered, secret)
}

func assertSafeDecryptFailure(t *testing.T, cipher *service.SecretCipher, profileID, envelope, secret string) {
	t.Helper()
	_, err := cipher.Decrypt(profileID, envelope)
	if err == nil {
		t.Fatal("expected decrypt failure")
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), envelope) {
		t.Fatalf("decrypt error leaked secret material: %q", err)
	}
}

func TestModelProfileServiceUpdatePreservesReplacesAndClearsAPIKey(t *testing.T) {
	backingStore := &teststore.Store{}
	cipher, _ := service.NewSecretCipher(testEncryptionKey())
	svc := service.NewModelProfileService(backingStore, cipher, nil)
	created, err := svc.Create(context.Background(), service.CreateModelProfileInput{
		Name: "Primary", RuntimeScope: model.ModelRuntimeGo, Protocol: model.ModelProtocolOpenAIChatCompletions,
		BaseURL: "https://example.com/v1/chat/completions/", ModelName: "model-a", APIKey: "first-secret", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	originalCiphertext := backingStore.ModelProfiles[0].APIKeyCiphertext

	if _, err := svc.Update(context.Background(), created.ID, service.UpdateModelProfileInput{Name: "Renamed"}); err != nil {
		t.Fatal(err)
	}
	if backingStore.ModelProfiles[0].APIKeyCiphertext != originalCiphertext {
		t.Fatal("omitted API key must preserve ciphertext")
	}
	if backingStore.ModelProfiles[0].BaseURL != "https://example.com/v1" {
		t.Fatalf("unexpected normalized URL %q", backingStore.ModelProfiles[0].BaseURL)
	}

	replacement := "second-secret"
	updated, err := svc.Update(context.Background(), created.ID, service.UpdateModelProfileInput{APIKey: &replacement})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.HasAPIKey || updated.APIKeyCiphertext != "" || updated.APIKeyHint != "...cret" {
		t.Fatalf("replacement was not safely redacted: %+v", updated)
	}
	plaintext, err := cipher.Decrypt(created.ID, backingStore.ModelProfiles[0].APIKeyCiphertext)
	if err != nil || plaintext != replacement {
		t.Fatalf("replacement decrypt = %q, %v", plaintext, err)
	}

	cleared, err := svc.Update(context.Background(), created.ID, service.UpdateModelProfileInput{ClearAPIKey: true})
	if err != nil {
		t.Fatal(err)
	}
	if cleared.HasAPIKey || backingStore.ModelProfiles[0].APIKeyCiphertext != "" || backingStore.ModelProfiles[0].APIKeyHint != "" {
		t.Fatalf("clear did not remove stored key: public=%+v stored=%+v", cleared, backingStore.ModelProfiles[0])
	}

	empty := ""
	if _, err := svc.Update(context.Background(), created.ID, service.UpdateModelProfileInput{APIKey: &empty}); !errors.Is(err, service.ErrInvalidModelProfile) {
		t.Fatalf("expected empty replacement to be rejected, got %v", err)
	}
	if _, err := svc.Update(context.Background(), created.ID, service.UpdateModelProfileInput{APIKey: &replacement, ClearAPIKey: true}); !errors.Is(err, service.ErrInvalidModelProfile) {
		t.Fatalf("expected replace+clear conflict to be rejected, got %v", err)
	}
}

func TestModelProfileServiceDefaultDisableAndDeleteRules(t *testing.T) {
	backingStore := &teststore.Store{}
	cipher, _ := service.NewSecretCipher(testEncryptionKey())
	svc := service.NewModelProfileService(backingStore, cipher, nil)
	first := createProfile(t, svc, "First", model.ModelRuntimeGo, true, false)
	second := createProfile(t, svc, "Second", model.ModelRuntimeGo, true, true)
	if backingStore.ModelProfiles[0].IsDefault || !backingStore.ModelProfiles[1].IsDefault {
		t.Fatalf("creating a new default must atomically replace the old default: %+v", backingStore.ModelProfiles)
	}
	if err := svc.SetDefault(context.Background(), first.ID); err != nil {
		t.Fatal(err)
	}
	disabled := false
	if _, err := svc.Update(context.Background(), first.ID, service.UpdateModelProfileInput{Enabled: &disabled}); !errors.Is(err, service.ErrDefaultModelProfile) {
		t.Fatalf("expected default disable conflict, got %v", err)
	}
	if err := svc.Delete(context.Background(), first.ID); !errors.Is(err, store.ErrModelProfileReferenced) {
		t.Fatalf("expected default delete conflict, got %v", err)
	}
	backingStore.Agents = []model.Agent{{ID: "a", ModelProfileID: second.ID}}
	if err := svc.Delete(context.Background(), second.ID); !errors.Is(err, store.ErrModelProfileReferenced) {
		t.Fatalf("expected agent reference delete conflict, got %v", err)
	}
}

func createProfile(t *testing.T, svc *service.ModelProfileService, name, scope string, enabled, isDefault bool) model.ModelProfile {
	t.Helper()
	profile, err := svc.Create(context.Background(), service.CreateModelProfileInput{
		Name: name, RuntimeScope: scope, Protocol: model.ModelProtocolOpenAIChatCompletions,
		BaseURL: "https://example.com", ModelName: "model-" + strings.ToLower(name), Enabled: enabled, IsDefault: isDefault,
	})
	if err != nil {
		t.Fatal(err)
	}
	return profile
}

func TestModelResolverDoesNotFallbackForDisabledExplicitProfile(t *testing.T) {
	store := &teststore.Store{ModelProfiles: []model.ModelProfile{
		{ID: "bound", RuntimeScope: model.ModelRuntimeGo, Enabled: false},
		{ID: "default", RuntimeScope: model.ModelRuntimeGo, Enabled: true, IsDefault: true},
	}}
	cipher, _ := service.NewSecretCipher(testEncryptionKey())
	resolver := service.NewModelResolver(store, cipher, map[string]service.EnvironmentModelConfig{
		model.ModelRuntimeGo: {BaseURL: "https://env.example/v1", ModelName: "env-model", APIKey: "env-secret"},
	})
	if _, err := resolver.Resolve(context.Background(), model.ModelRuntimeGo, "bound"); err == nil {
		t.Fatal("disabled explicit profile must fail without falling back")
	}
}

func TestModelResolverUsesExplicitThenDefaultThenEnvironment(t *testing.T) {
	cipher, _ := service.NewSecretCipher(testEncryptionKey())
	explicitCiphertext, _ := cipher.Encrypt("explicit", "explicit-key")
	defaultCiphertext, _ := cipher.Encrypt("default", "default-key")
	backingStore := &teststore.Store{ModelProfiles: []model.ModelProfile{
		{ID: "explicit", RuntimeScope: model.ModelRuntimeGo, BaseURL: "https://explicit.example/v1", ModelName: "explicit-model", APIKeyCiphertext: explicitCiphertext, Enabled: true},
		{ID: "default", RuntimeScope: model.ModelRuntimeGo, BaseURL: "https://default.example/v1", ModelName: "default-model", APIKeyCiphertext: defaultCiphertext, Enabled: true, IsDefault: true},
	}}
	resolver := service.NewModelResolver(backingStore, cipher, map[string]service.EnvironmentModelConfig{
		model.ModelRuntimeGo: {BaseURL: "https://env.example/v1", ModelName: "env-model", APIKey: "env-key"},
	})

	explicit, err := resolver.Resolve(context.Background(), model.ModelRuntimeGo, "explicit")
	if err != nil || explicit.ProfileID != "explicit" || explicit.APIKey != "explicit-key" || explicit.Source != "database" {
		t.Fatalf("explicit resolution = %+v, %v", explicit, err)
	}
	byDefault, err := resolver.Resolve(context.Background(), model.ModelRuntimeGo, "")
	if err != nil || byDefault.ProfileID != "default" || byDefault.ModelName != "default-model" {
		t.Fatalf("default resolution = %+v, %v", byDefault, err)
	}

	backingStore.ModelProfiles = nil
	fromEnvironment, err := resolver.Resolve(context.Background(), model.ModelRuntimeGo, "")
	if err != nil || fromEnvironment.Source != "environment" || fromEnvironment.ModelName != "env-model" || fromEnvironment.ProfileID != "" {
		t.Fatalf("environment resolution = %+v, %v", fromEnvironment, err)
	}
	if _, err := resolver.Resolve(context.Background(), model.ModelRuntimeGo, "missing"); !errors.Is(err, store.ErrModelProfileNotFound) {
		t.Fatalf("explicit missing profile must not fall back, got %v", err)
	}
}

func TestModelResolverDoesNotFallbackWhenExplicitCiphertextCannotBeDecrypted(t *testing.T) {
	cipher, _ := service.NewSecretCipher(testEncryptionKey())
	otherCipher, _ := service.NewSecretCipher(base64.StdEncoding.EncodeToString([]byte("abcdef0123456789abcdef0123456789")))
	ciphertext, _ := otherCipher.Encrypt("bound", "do-not-fallback")
	backingStore := &teststore.Store{ModelProfiles: []model.ModelProfile{
		{ID: "bound", RuntimeScope: model.ModelRuntimeGo, BaseURL: "https://bound.example/v1", ModelName: "bound", APIKeyCiphertext: ciphertext, Enabled: true},
		{ID: "default", RuntimeScope: model.ModelRuntimeGo, BaseURL: "https://default.example/v1", ModelName: "default", Enabled: true, IsDefault: true},
	}}
	resolver := service.NewModelResolver(backingStore, cipher, map[string]service.EnvironmentModelConfig{
		model.ModelRuntimeGo: {BaseURL: "https://env.example/v1", ModelName: "env", APIKey: "env-key"},
	})
	_, err := resolver.Resolve(context.Background(), model.ModelRuntimeGo, "bound")
	if err == nil || strings.Contains(err.Error(), "do-not-fallback") || strings.Contains(err.Error(), ciphertext) {
		t.Fatalf("expected safe decrypt failure without fallback, got %v", err)
	}
}

func TestModelProfileConnectionTestSupportsDraftAndSanitizesFailures(t *testing.T) {
	secret := "draft-secret-never-return"
	var captured struct {
		Model string `json:"model"`
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+secret {
			t.Errorf("unexpected auth header %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"` + secret + `"}`))
	}))
	defer upstream.Close()

	svc := service.NewModelProfileService(&teststore.Store{}, nil, upstream.Client())
	result, err := svc.TestConnection(context.Background(), service.TestModelProfileInput{
		BaseURL: upstream.URL + "/v1/chat/completions/", ModelName: "draft-model", APIKey: secret,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.OK || result.Error != "authentication failed" || captured.Model != "draft-model" {
		t.Fatalf("unexpected connection result %+v, request %+v", result, captured)
	}
	encoded, _ := json.Marshal(result)
	if strings.Contains(string(encoded), secret) {
		t.Fatalf("connection result leaked API key: %s", encoded)
	}
}

func TestSavedDisabledProfileCannotBeConnectionTested(t *testing.T) {
	svc := service.NewModelProfileService(&teststore.Store{ModelProfiles: []model.ModelProfile{{ID: "off", Enabled: false}}}, nil, nil)
	_, err := svc.TestConnection(context.Background(), service.TestModelProfileInput{ProfileID: "off"})
	if !errors.Is(err, service.ErrModelProfileDisabled) {
		t.Fatalf("expected disabled profile error, got %v", err)
	}
}
