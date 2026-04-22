package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rally-finance/ocpi-mock-hub/ocpiutil"
)

// defaultTokenType mirrors the OCPI spec default for the `?type=` query
// parameter on the receiver Token module. The spec lists RFID as the assumed
// value when the query parameter is omitted.
const defaultTokenType = "RFID"

// tokenStorageUID builds the storage identifier for a token. OCPI 2.2.1 allows
// tokens to share a UID as long as their TokenType differs, so the storage
// layer must key by (uid, type). To preserve backward compatibility with any
// pre-existing RFID token data the RFID case keeps the bare UID.
func tokenStorageUID(uid, tokenType string) string {
	normalized := strings.ToUpper(strings.TrimSpace(tokenType))
	if normalized == "" || normalized == defaultTokenType {
		return uid
	}
	return uid + "|" + normalized
}

// tokenTypeFromRequest returns the `type` query parameter, defaulting to RFID
// when omitted per spec.
func tokenTypeFromRequest(r *http.Request) string {
	t := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("type")))
	if t == "" {
		return defaultTokenType
	}
	return t
}

// PutToken handles PUT /ocpi/2.2.1/receiver/tokens/{cc}/{pid}/{uid}?type=.
// The body must be a full Token object. The stored token is enriched with
// country_code/party_id/uid/type so subsequent sender-module reads return a
// consistent object.
func (h *Handler) PutToken(w http.ResponseWriter, r *http.Request) {
	store := h.storeForRequest(r)
	cc := chi.URLParam(r, "countryCode")
	pid := chi.URLParam(r, "partyID")
	uid := chi.URLParam(r, "uid")
	tokenType := tokenTypeFromRequest(r)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusClientError, "Failed to read body")
		return
	}

	// Fold identifiers from the URL into the stored body so listings remain
	// well-formed even if the client omitted them.
	if len(body) > 0 {
		var parsed map[string]any
		if err := json.Unmarshal(body, &parsed); err == nil {
			parsed["country_code"] = cc
			parsed["party_id"] = pid
			parsed["uid"] = uid
			if _, ok := parsed["type"]; !ok {
				parsed["type"] = tokenType
			}
			if merged, err := json.Marshal(parsed); err == nil {
				body = merged
			}
		}
	}

	if err := store.PutToken(cc, pid, tokenStorageUID(uid, tokenType), body); err != nil {
		ocpiutil.Error(w, r, http.StatusInternalServerError, ocpiutil.StatusServerError, "Failed to store token")
		return
	}

	ocpiutil.OK(w, r, nil)
}

// PatchToken handles PATCH /ocpi/2.2.1/receiver/tokens/{cc}/{pid}/{uid}?type=.
// The body is a partial Token object that is JSON-merged on top of the
// existing stored token. Returns 2004 unknown_object if the token is unknown.
// Per spec the request must include `last_updated`; we enforce that.
func (h *Handler) PatchToken(w http.ResponseWriter, r *http.Request) {
	store := h.storeForRequest(r)
	cc := chi.URLParam(r, "countryCode")
	pid := chi.URLParam(r, "partyID")
	uid := chi.URLParam(r, "uid")
	tokenType := tokenTypeFromRequest(r)
	storageUID := tokenStorageUID(uid, tokenType)

	existing, err := store.GetToken(cc, pid, storageUID)
	if err != nil {
		ocpiutil.Error(w, r, http.StatusInternalServerError, ocpiutil.StatusServerError, "Failed to read token")
		return
	}
	if existing == nil {
		ocpiutil.Error(w, r, http.StatusNotFound, ocpiutil.StatusUnknownObject, "Token not found")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusClientError, "Failed to read body")
		return
	}

	var patch map[string]any
	if err := json.Unmarshal(body, &patch); err != nil {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusInvalidParams, "Invalid JSON")
		return
	}
	if _, ok := patch["last_updated"]; !ok {
		ocpiutil.Error(w, r, http.StatusBadRequest, ocpiutil.StatusInvalidParams, "PATCH body must include last_updated")
		return
	}

	var target map[string]any
	if err := json.Unmarshal(existing, &target); err != nil {
		target = map[string]any{}
	}
	for k, v := range patch {
		target[k] = v
	}
	target["country_code"] = cc
	target["party_id"] = pid
	target["uid"] = uid
	if _, ok := target["type"]; !ok {
		target["type"] = tokenType
	}

	merged, err := json.Marshal(target)
	if err != nil {
		ocpiutil.Error(w, r, http.StatusInternalServerError, ocpiutil.StatusServerError, "Failed to encode token")
		return
	}
	if err := store.PutToken(cc, pid, storageUID, merged); err != nil {
		ocpiutil.Error(w, r, http.StatusInternalServerError, ocpiutil.StatusServerError, "Failed to store token")
		return
	}
	ocpiutil.OK(w, r, nil)
}

// GetReceiverToken handles GET /ocpi/2.2.1/receiver/tokens/{cc}/{pid}/{uid}?type=.
// Mirrors the sender-side GetTokenByID but lives under the receiver URL, which
// lets an eMSP verify the latest token state the hub has for it.
func (h *Handler) GetReceiverToken(w http.ResponseWriter, r *http.Request) {
	store := h.storeForRequest(r)
	cc := chi.URLParam(r, "countryCode")
	pid := chi.URLParam(r, "partyID")
	uid := chi.URLParam(r, "uid")
	tokenType := tokenTypeFromRequest(r)

	raw, err := store.GetToken(cc, pid, tokenStorageUID(uid, tokenType))
	if err != nil {
		ocpiutil.Error(w, r, http.StatusInternalServerError, ocpiutil.StatusServerError, "Failed to get token")
		return
	}
	if raw == nil {
		ocpiutil.Error(w, r, http.StatusNotFound, ocpiutil.StatusUnknownObject, "Token not found")
		return
	}
	ocpiutil.OK(w, r, json.RawMessage(raw))
}
