package hub

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// KVStore uses Vercel KV (Redis) REST API for state persistence.
type KVStore struct {
	url   string
	token string
}

func NewKVStore(url, token string) *KVStore {
	return &KVStore{
		url:   strings.TrimRight(url, "/"),
		token: token,
	}
}

func (kv *KVStore) kvGET(key string) (string, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/get/%s", kv.url, key), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+kv.token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Result *string `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if result.Result == nil {
		return "", nil
	}
	return *result.Result, nil
}

func (kv *KVStore) kvSET(key, value string) error {
	payload, _ := json.Marshal([]string{"SET", key, value})
	req, err := http.NewRequest("POST", kv.url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+kv.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (kv *KVStore) kvDEL(key string) error {
	payload, _ := json.Marshal([]string{"DEL", key})
	req, err := http.NewRequest("POST", kv.url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+kv.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// kvSCAN returns all keys matching a pattern using the SCAN command.
func (kv *KVStore) kvSCAN(pattern string) ([]string, error) {
	var allKeys []string
	cursor := "0"
	for {
		payload, _ := json.Marshal([]string{"SCAN", cursor, "MATCH", pattern, "COUNT", "100"})
		req, err := http.NewRequest("POST", kv.url, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+kv.token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var result struct {
			Result []any `json:"result"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, err
		}
		if len(result.Result) < 2 {
			break
		}

		cursor = fmt.Sprintf("%v", result.Result[0])
		if keys, ok := result.Result[1].([]any); ok {
			for _, k := range keys {
				if s, ok := k.(string); ok {
					allKeys = append(allKeys, s)
				}
			}
		}

		if cursor == "0" {
			break
		}
	}
	return allKeys, nil
}

func (kv *KVStore) kvGETMulti(keys []string) ([]string, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	args := append([]string{"MGET"}, keys...)
	payload, _ := json.Marshal(args)
	req, err := http.NewRequest("POST", kv.url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+kv.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Result []any `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	values := make([]string, len(result.Result))
	for i, v := range result.Result {
		if s, ok := v.(string); ok {
			values[i] = s
		}
	}
	return values, nil
}

// Store interface implementation

func (kv *KVStore) GetTokenB() (string, error)              { return kv.kvGET("handshake:token_b") }
func (kv *KVStore) SetTokenB(token string) error             { return kv.kvSET("handshake:token_b", token) }
func (kv *KVStore) GetEMSPCallbackURL() (string, error)     { return kv.kvGET("handshake:emsp_url") }
func (kv *KVStore) SetEMSPCallbackURL(url string) error     { return kv.kvSET("handshake:emsp_url", url) }
func (kv *KVStore) GetEMSPCredentials() ([]byte, error) {
	v, err := kv.kvGET("handshake:emsp_creds")
	return []byte(v), err
}
func (kv *KVStore) SetEMSPCredentials(creds []byte) error {
	return kv.kvSET("handshake:emsp_creds", string(creds))
}

func (kv *KVStore) GetEMSPOwnToken() (string, error) {
	return kv.kvGET("handshake:emsp_own_token")
}
func (kv *KVStore) SetEMSPOwnToken(token string) error {
	return kv.kvSET("handshake:emsp_own_token", token)
}
func (kv *KVStore) GetEMSPVersionsURL() (string, error) {
	return kv.kvGET("handshake:emsp_versions_url")
}
func (kv *KVStore) SetEMSPVersionsURL(url string) error {
	return kv.kvSET("handshake:emsp_versions_url", url)
}

func (kv *KVStore) PutToken(cc, pid, uid string, token []byte) error {
	return kv.kvSET(fmt.Sprintf("token:%s/%s/%s", cc, pid, uid), string(token))
}

func (kv *KVStore) GetToken(cc, pid, uid string) ([]byte, error) {
	v, err := kv.kvGET(fmt.Sprintf("token:%s/%s/%s", cc, pid, uid))
	if v == "" {
		return nil, err
	}
	return []byte(v), err
}

func (kv *KVStore) ListTokens() ([][]byte, error) {
	return kv.listByPrefix("token:*")
}

func (kv *KVStore) PutSession(id string, session []byte) error {
	return kv.kvSET("session:"+id, string(session))
}

func (kv *KVStore) GetSession(id string) ([]byte, error) {
	v, err := kv.kvGET("session:" + id)
	if v == "" {
		return nil, err
	}
	return []byte(v), err
}

func (kv *KVStore) ListSessions() ([][]byte, error) {
	return kv.listByPrefix("session:*")
}

func (kv *KVStore) DeleteSession(id string) error {
	return kv.kvDEL("session:" + id)
}

func (kv *KVStore) PutCDR(id string, cdr []byte) error {
	return kv.kvSET("cdr:"+id, string(cdr))
}

func (kv *KVStore) ListCDRs() ([][]byte, error) {
	return kv.listByPrefix("cdr:*")
}

func (kv *KVStore) GetMode() (string, error) {
	v, err := kv.kvGET("config:mode")
	if v == "" {
		return "happy", err
	}
	return v, err
}

func (kv *KVStore) SetMode(mode string) error {
	return kv.kvSET("config:mode", mode)
}

func (kv *KVStore) listByPrefix(pattern string) ([][]byte, error) {
	keys, err := kv.kvSCAN(pattern)
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, nil
	}
	values, err := kv.kvGETMulti(keys)
	if err != nil {
		return nil, err
	}
	result := make([][]byte, 0, len(values))
	for _, v := range values {
		if v != "" {
			result = append(result, []byte(v))
		}
	}
	return result, nil
}
