package main

import (
	"bytes"
	"encoding/hex"
	"net/http"
	"testing"
)

func assertHexID(t *testing.T, name, got string) {
	t.Helper()
	if len(got) != 32 {
		t.Fatalf("%s length = %d, want 32: %q", name, len(got), got)
	}
	if _, err := hex.DecodeString(got); err != nil {
		t.Fatalf("%s = %q, want hexadecimal: %v", name, got, err)
	}
}

func TestCommonHeaders_RequestID(t *testing.T) {
	first, err := http.NewRequest(http.MethodPost, "https://example.com/v2/chat/completions", nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := http.NewRequest(http.MethodPost, "https://example.com/v2/chat/completions", nil)
	if err != nil {
		t.Fatal(err)
	}

	commonHeaders(first)
	commonHeaders(second)

	firstID := first.Header.Get("X-Request-ID")
	secondID := second.Header.Get("X-Request-ID")
	assertHexID(t, "X-Request-ID", firstID)
	assertHexID(t, "X-Request-ID", secondID)
	if firstID == secondID {
		t.Fatalf("X-Request-ID repeated across requests: %q", firstID)
	}
}

func TestBackendHeaders_ClientIdentityByRealm(t *testing.T) {
	cases := []struct {
		name           string
		domain         string
		enterpriseID   string
		wantUA         string
		wantTenantID   string
		wantIDEType    string
		wantIDEName    string
		wantIDEVersion string
	}{
		{"CN", "www.codebuddy.cn", "ent-cn", clientUA, "", "", "", ""},
		{"legacy empty domain", "", "ent-cn", clientUA, "", "", "", ""},
		{"Global", "www.workbuddy.ai", "ent-global", clientUAGlobal, "ent-global", clientIDEGlobal, clientIDEGlobal, clientVersionGlobal},
		{"Global without enterprise", "www.workbuddy.ai", "", clientUAGlobal, "", clientIDEGlobal, clientIDEGlobal, clientVersionGlobal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, "https://example.com/v2/chat/completions", nil)
			if err != nil {
				t.Fatal(err)
			}
			sa := &storedAuth{
				Auth:    storedTokens{Domain: tc.domain},
				Account: storedAccount{EnterpriseID: tc.enterpriseID},
			}
			backendHeaders(req, sa)
			requestID := req.Header.Get("X-Request-ID")
			conversationRequestID := req.Header.Get("X-Conversation-Request-ID")
			assertHexID(t, "X-Request-ID", requestID)
			assertHexID(t, "X-Conversation-Request-ID", conversationRequestID)
			if conversationRequestID == requestID {
				t.Fatalf("X-Conversation-Request-ID must differ from X-Request-ID: %q", conversationRequestID)
			}
			if got := req.Header.Get("User-Agent"); got != tc.wantUA {
				t.Fatalf("User-Agent = %q, want %q", got, tc.wantUA)
			}
			if got := req.Header.Get("X-Tenant-Id"); got != tc.wantTenantID {
				t.Fatalf("X-Tenant-Id = %q, want %q", got, tc.wantTenantID)
			}
			if got := req.Header.Get("X-IDE-Type"); got != tc.wantIDEType {
				t.Fatalf("X-IDE-Type = %q, want %q", got, tc.wantIDEType)
			}
			if got := req.Header.Get("X-IDE-Name"); got != tc.wantIDEName {
				t.Fatalf("X-IDE-Name = %q, want %q", got, tc.wantIDEName)
			}
			if got := req.Header.Get("X-IDE-Version"); got != tc.wantIDEVersion {
				t.Fatalf("X-IDE-Version = %q, want %q", got, tc.wantIDEVersion)
			}
		})
	}
}

func TestRebuildRequestWithSA_GlobalClientIdentity(t *testing.T) {
	orig, err := http.NewRequest(
		http.MethodPost,
		"https://www.workbuddy.ai/v2/chat/completions",
		bytes.NewReader([]byte(`{"model":"test","messages":[]}`)),
	)
	if err != nil {
		t.Fatal(err)
	}
	sa := &storedAuth{
		Auth:    storedTokens{Domain: "www.workbuddy.ai"},
		Account: storedAccount{EnterpriseID: "ent-global"},
	}
	next, err := rebuildRequestWithSA(orig, sa)
	if err != nil {
		t.Fatal(err)
	}
	if got := next.Header.Get("User-Agent"); got != clientUAGlobal {
		t.Fatalf("rebuilt User-Agent = %q, want %q", got, clientUAGlobal)
	}
	if got := next.Header.Get("X-Tenant-Id"); got != "ent-global" {
		t.Fatalf("rebuilt X-Tenant-Id = %q, want %q", got, "ent-global")
	}
	if got := next.Header.Get("X-IDE-Type"); got != clientIDEGlobal {
		t.Fatalf("rebuilt X-IDE-Type = %q, want %q", got, clientIDEGlobal)
	}
	if got := next.Header.Get("X-IDE-Name"); got != clientIDEGlobal {
		t.Fatalf("rebuilt X-IDE-Name = %q, want %q", got, clientIDEGlobal)
	}
	if got := next.Header.Get("X-IDE-Version"); got != clientVersionGlobal {
		t.Fatalf("rebuilt X-IDE-Version = %q, want %q", got, clientVersionGlobal)
	}
	requestID := next.Header.Get("X-Request-ID")
	conversationRequestID := next.Header.Get("X-Conversation-Request-ID")
	assertHexID(t, "X-Request-ID", requestID)
	assertHexID(t, "X-Conversation-Request-ID", conversationRequestID)
	if conversationRequestID == requestID {
		t.Fatalf("rebuilt X-Conversation-Request-ID must differ from X-Request-ID: %q", conversationRequestID)
	}
}
