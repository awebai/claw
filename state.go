package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/awebai/aw/awconfig"
	"github.com/awebai/aw/awid"
)

// Account state (slug + secret) lives in the claw config dir; identity
// state (keys, address) lives in the workspace .aw/ in aw's own format so
// the two CLIs stay interoperable.

type accountState struct {
	Slug      string `json:"slug"`
	Namespace string `json:"namespace"`
	ServerURL string `json:"server_url"`
}

func clawConfigDir() (string, error) {
	if v := strings.TrimSpace(os.Getenv("CLAWEB_SECRET_FILE")); v != "" {
		return filepath.Dir(v), nil
	}
	if v := strings.TrimSpace(os.Getenv("OPENCLAW_STATE_DIR")); v != "" {
		return filepath.Join(v, "claweb"), nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "claweb"), nil
}

func secretPath() (string, error) {
	if v := strings.TrimSpace(os.Getenv("CLAWEB_SECRET_FILE")); v != "" {
		return v, nil
	}
	dir, err := clawConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "account-secret"), nil
}

func accountPath() (string, error) {
	dir, err := clawConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "account.json"), nil
}

func saveAccount(slug, namespace, secret string) error {
	dir, err := clawConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	sp, err := secretPath()
	if err != nil {
		return err
	}
	if err := os.WriteFile(sp, []byte(secret), 0o600); err != nil {
		return err
	}
	ap, err := accountPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(accountState{
		Slug: slug, Namespace: namespace, ServerURL: serverURL(),
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ap, data, 0o600)
}

func loadSecret() (string, error) {
	sp, err := secretPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(sp)
	if errors.Is(err, os.ErrNotExist) {
		return "", errors.New("no ClaWeb account secret found; run `claw register <slug>` first")
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func loadAccount() (*accountState, error) {
	ap, err := accountPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(ap)
	if errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("no ClaWeb account found; run `claw register <slug>` first")
	}
	if err != nil {
		return nil, err
	}
	var st accountState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

// identityClient builds an identity-auth aweb client from the workspace
// identity in the current directory.
func identityClient() (*awid.Client, *awconfig.ResolvedIdentity, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, nil, err
	}
	identity, err := awconfig.ResolveIdentity(wd)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, errors.New("no identity in this directory; run `claw new <name>` first")
		}
		return nil, nil, err
	}
	key, err := awid.LoadSigningKey(identity.SigningKeyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("load signing key: %w", err)
	}
	c, err := awid.NewWithIdentity(apiBase(), key, identity.DID)
	if err != nil {
		return nil, nil, err
	}
	if v := strings.TrimSpace(identity.StableID); v != "" {
		c.SetStableID(v)
	}
	if v := strings.TrimSpace(identity.Address); v != "" {
		c.SetAddress(v)
	}
	resolver := awid.NewRegistryResolver(nil, nil)
	if err := resolver.SetFallbackRegistryURL(registryURL()); err != nil {
		return nil, nil, err
	}
	c.SetResolver(&awid.ChainResolver{
		DIDKey:   &awid.DIDKeyResolver{},
		Registry: resolver,
		Pin:      &awid.PinResolver{Store: awid.NewPinStore()},
	})
	return c, identity, nil
}

// clawebRequest calls a ClaWeb account endpoint with the account secret.
func clawebRequest(method, path string, body any, bearer string) (int, map[string]any, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, apiBase()+path, reader)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	out := map[string]any{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &out); err != nil {
			return resp.StatusCode, nil, fmt.Errorf("invalid response: %s", string(data))
		}
	}
	return resp.StatusCode, out, nil
}

func apiError(status int, out map[string]any) error {
	detail := out["detail"]
	if m, ok := detail.(map[string]any); ok {
		if code, ok := m["error"].(string); ok {
			if msg, ok := m["message"].(string); ok && msg != "" {
				return fmt.Errorf("%s (%d): %s", code, status, msg)
			}
			return fmt.Errorf("%s (%d)", code, status)
		}
	}
	if s, ok := detail.(string); ok && s != "" {
		return fmt.Errorf("%s (%d)", s, status)
	}
	return fmt.Errorf("request failed (%d)", status)
}
