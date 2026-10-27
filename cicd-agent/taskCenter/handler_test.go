package taskCenter

import (
	"cicd-agent/common"
	"encoding/json"
	"testing"
)

func TestResolveCallbackPayloadEncrypted(t *testing.T) {
	common.InitLogger()

	plain := []byte(`{"project":"ysh","status":"success","tag":"v1"}`)
	encrypted, err := common.CompressAndEncrypt(plain)
	if err != nil {
		t.Fatalf("encrypt payload: %v", err)
	}
	body, err := json.Marshal(map[string]string{"data": encrypted})
	if err != nil {
		t.Fatalf("marshal encrypted body: %v", err)
	}

	got, err := resolveCallbackPayload(body, false)
	if err != nil {
		t.Fatalf("encrypted payload: unexpected error: %v", err)
	}
	if string(got) != string(plain) {
		t.Fatalf("encrypted payload: got %s, want %s", got, plain)
	}
}

func TestResolveCallbackPayloadPlaintext(t *testing.T) {
	common.InitLogger()

	plain := []byte(`{"project":"ysh","status":"success","tag":"v1"}`)

	got, err := resolveCallbackPayload(plain, true)
	if err != nil {
		t.Fatalf("plain payload: unexpected error: %v", err)
	}
	if string(got) != string(plain) {
		t.Fatalf("plain payload: got %s, want %s", got, plain)
	}

	if _, err := resolveCallbackPayload(plain, false); err == nil {
		t.Fatal("expected plaintext callback to be rejected when allow_plaintext is false")
	}
}

func TestResolveCallbackPayloadInvalidCiphertext(t *testing.T) {
	common.InitLogger()

	body := []byte(`{"data":"not-a-valid-ciphertext"}`)
	if _, err := resolveCallbackPayload(body, false); err == nil {
		t.Fatal("expected error for invalid ciphertext")
	}
}
