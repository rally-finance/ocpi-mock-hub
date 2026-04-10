package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore uses a standard Redis connection for state persistence.
type RedisStore struct {
	rdb *redis.Client
}

func NewRedisStore(redisURL string) (*RedisStore, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse REDIS_URL: %w", err)
	}
	rdb := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	log.Printf("[redis] connected to %s", maskRedisURL(redisURL))
	return &RedisStore{rdb: rdb}, nil
}

func maskRedisURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "(invalid URL)"
	}
	if u.User != nil {
		u.User = url.UserPassword(u.User.Username(), "***")
	}
	return u.Redacted()
}

func (r *RedisStore) ctx() context.Context {
	return context.Background()
}

func (r *RedisStore) get(key string) (string, error) {
	val, err := r.rdb.Get(r.ctx(), key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

func (r *RedisStore) set(key, value string) error {
	return r.rdb.Set(r.ctx(), key, value, 0).Err()
}

func (r *RedisStore) del(key string) error {
	return r.rdb.Del(r.ctx(), key).Err()
}

func (r *RedisStore) scan(pattern string) ([]string, error) {
	var allKeys []string
	iter := r.rdb.Scan(r.ctx(), 0, pattern, 100).Iterator()
	for iter.Next(r.ctx()) {
		allKeys = append(allKeys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	return allKeys, nil
}

func (r *RedisStore) listByPrefix(pattern string) ([][]byte, error) {
	keys, err := r.scan(pattern)
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, nil
	}
	vals, err := r.rdb.MGet(r.ctx(), keys...).Result()
	if err != nil {
		return nil, err
	}
	result := make([][]byte, 0, len(vals))
	for _, v := range vals {
		if s, ok := v.(string); ok && s != "" {
			result = append(result, []byte(s))
		}
	}
	return result, nil
}

// Store interface implementation

func (r *RedisStore) GetTokenB() (string, error)          { return r.get("handshake:token_b") }
func (r *RedisStore) SetTokenB(token string) error         { return r.set("handshake:token_b", token) }
func (r *RedisStore) GetEMSPCallbackURL() (string, error)  { return r.get("handshake:emsp_url") }
func (r *RedisStore) SetEMSPCallbackURL(url string) error  { return r.set("handshake:emsp_url", url) }
func (r *RedisStore) GetEMSPCredentials() ([]byte, error) {
	v, err := r.get("handshake:emsp_creds")
	return []byte(v), err
}
func (r *RedisStore) SetEMSPCredentials(creds []byte) error {
	return r.set("handshake:emsp_creds", string(creds))
}
func (r *RedisStore) GetEMSPOwnToken() (string, error) {
	return r.get("handshake:emsp_own_token")
}
func (r *RedisStore) SetEMSPOwnToken(token string) error {
	return r.set("handshake:emsp_own_token", token)
}
func (r *RedisStore) GetEMSPVersionsURL() (string, error) {
	return r.get("handshake:emsp_versions_url")
}
func (r *RedisStore) SetEMSPVersionsURL(url string) error {
	return r.set("handshake:emsp_versions_url", url)
}

func (r *RedisStore) PutToken(cc, pid, uid string, token []byte) error {
	return r.set(fmt.Sprintf("token:%s/%s/%s", cc, pid, uid), string(token))
}

func (r *RedisStore) GetToken(cc, pid, uid string) ([]byte, error) {
	v, err := r.get(fmt.Sprintf("token:%s/%s/%s", cc, pid, uid))
	if v == "" {
		return nil, err
	}
	return []byte(v), err
}

func (r *RedisStore) ListTokens() ([][]byte, error) {
	return r.listByPrefix("token:*")
}

func (r *RedisStore) PutSession(id string, session []byte) error {
	return r.set("session:"+id, string(session))
}

func (r *RedisStore) GetSession(id string) ([]byte, error) {
	v, err := r.get("session:" + id)
	if v == "" {
		return nil, err
	}
	return []byte(v), err
}

func (r *RedisStore) ListSessions() ([][]byte, error) {
	return r.listByPrefix("session:*")
}

func (r *RedisStore) DeleteSession(id string) error {
	return r.del("session:" + id)
}

func (r *RedisStore) PutCDR(id string, cdr []byte) error {
	return r.set("cdr:"+id, string(cdr))
}

func (r *RedisStore) GetCDR(id string) ([]byte, error) {
	v, err := r.get("cdr:" + id)
	if v == "" {
		return nil, err
	}
	return []byte(v), err
}

func (r *RedisStore) ListCDRs() ([][]byte, error) {
	return r.listByPrefix("cdr:*")
}

func (r *RedisStore) PutReservation(id string, reservation []byte) error {
	return r.set("reservation:"+id, string(reservation))
}

func (r *RedisStore) GetReservation(id string) ([]byte, error) {
	v, err := r.get("reservation:" + id)
	if v == "" {
		return nil, err
	}
	return []byte(v), err
}

func (r *RedisStore) ListReservations() ([][]byte, error) {
	return r.listByPrefix("reservation:*")
}

func (r *RedisStore) DeleteReservation(id string) error {
	return r.del("reservation:" + id)
}

func (r *RedisStore) PutParty(key string, state []byte) error {
	if err := r.set("party:"+key, string(state)); err != nil {
		return err
	}
	var p struct{ TokenB string `json:"token_b"` }
	if json.Unmarshal(state, &p) == nil && p.TokenB != "" {
		return r.set("tokenb:"+p.TokenB, key)
	}
	return nil
}

func (r *RedisStore) GetParty(key string) ([]byte, error) {
	v, err := r.get("party:" + key)
	if v == "" {
		return nil, err
	}
	return []byte(v), err
}

func (r *RedisStore) GetPartyByTokenB(tokenB string) ([]byte, error) {
	key, err := r.get("tokenb:" + tokenB)
	if key == "" {
		return nil, err
	}
	return r.GetParty(key)
}

func (r *RedisStore) DeleteParty(key string) error {
	raw, _ := r.GetParty(key)
	if raw != nil {
		var p struct{ TokenB string `json:"token_b"` }
		if json.Unmarshal(raw, &p) == nil && p.TokenB != "" {
			r.del("tokenb:" + p.TokenB)
		}
	}
	return r.del("party:" + key)
}

func (r *RedisStore) ListParties() ([][]byte, error) {
	return r.listByPrefix("party:*")
}

func (r *RedisStore) PutChargingProfile(sessionID string, profile []byte) error {
	return r.set("chargingprofile:"+sessionID, string(profile))
}

func (r *RedisStore) GetChargingProfile(sessionID string) ([]byte, error) {
	v, err := r.get("chargingprofile:" + sessionID)
	if v == "" {
		return nil, err
	}
	return []byte(v), err
}

func (r *RedisStore) DeleteChargingProfile(sessionID string) error {
	return r.del("chargingprofile:" + sessionID)
}

func (r *RedisStore) GetMode() (string, error) {
	v, err := r.get("config:mode")
	if v == "" {
		return "happy", err
	}
	return v, err
}

func (r *RedisStore) SetMode(mode string) error {
	return r.set("config:mode", mode)
}
