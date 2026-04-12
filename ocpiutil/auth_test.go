package ocpiutil

import "testing"

func TestAuthHeaderMatchesTokenAcceptsLiteralBase64LookingToken(t *testing.T) {
	const token = "dG9rZW4tYi0xMjM="

	if !AuthHeaderMatchesToken("Token "+token, token) {
		t.Fatal("expected literal token match for a raw token that happens to be valid base64")
	}
}

func TestAuthHeaderMatchesTokenAcceptsBase64EncodedToken(t *testing.T) {
	if !AuthHeaderMatchesToken("Token cGVlci10b2tlbg==", "peer-token") {
		t.Fatal("expected base64-encoded authorization token to match the stored token")
	}
}
