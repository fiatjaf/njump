package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/sdk"
)

func mustPubKey(t *testing.T, hex string) nostr.PubKey {
	t.Helper()
	pk, err := nostr.PubKeyFromHex(hex)
	if err != nil {
		t.Fatalf("PubKeyFromHex(%q): %v", hex, err)
	}
	return pk
}

func TestNip05NamecoinIdentifierToURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"alice@x.bit", "https://explorer.namecoin.org/name/d/x"},
		{"x.bit", "https://explorer.namecoin.org/name/d/x"},
		{"d/x", "https://explorer.namecoin.org/name/d/x"},
		{"alice@example.com", ""},
		{"_@example.com", ""},
		{"", ""},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := nip05NamecoinIdentifierToURL(c.in)
			if got != c.want {
				t.Fatalf("nip05NamecoinIdentifierToURL(%q) = %q; want %q", c.in, got, c.want)
			}
		})
	}
}

func TestNip05IdentifierURL_FallbackForDNS(t *testing.T) {
	got := nip05IdentifierURL("alice@example.com")
	if got == "" || !strings.Contains(got, "example.com") {
		t.Fatalf("expected DNS .well-known URL for example.com, got %q", got)
	}
	if strings.Contains(got, "namecoin") {
		t.Fatalf("expected DNS URL, not Namecoin explorer, got %q", got)
	}
}

func TestVerifyNip05Namecoin_NotDotBit(t *testing.T) {
	verified, ok := verifyNip05Namecoin(context.Background(), "alice@example.com", nostr.ZeroPK)
	if ok {
		t.Fatalf("ok=true for non-.bit identifier; want false")
	}
	if verified {
		t.Fatalf("verified=true for non-.bit identifier; want false")
	}
}

func TestVerifyNip05Namecoin_MatchAndMismatch(t *testing.T) {
	expected := mustPubKey(t, "0000000000000000000000000000000000000000000000000000000000000001")
	other := mustPubKey(t, "0000000000000000000000000000000000000000000000000000000000000002")

	prev := queryNamecoinIdentifier
	defer func() { queryNamecoinIdentifier = prev }()

	t.Run("match", func(t *testing.T) {
		queryNamecoinIdentifier = func(ctx context.Context, id string) (*nostr.ProfilePointer, error) {
			return &nostr.ProfilePointer{PublicKey: expected}, nil
		}
		verified, ok := verifyNip05Namecoin(context.Background(), "alice@example.bit", expected)
		if !ok {
			t.Fatalf("ok=false; want true for .bit identifier")
		}
		if !verified {
			t.Fatalf("verified=false; want true for matching pubkey")
		}
	})

	t.Run("mismatch", func(t *testing.T) {
		queryNamecoinIdentifier = func(ctx context.Context, id string) (*nostr.ProfilePointer, error) {
			return &nostr.ProfilePointer{PublicKey: other}, nil
		}
		verified, ok := verifyNip05Namecoin(context.Background(), "alice@example.bit", expected)
		if !ok {
			t.Fatalf("ok=false; want true for .bit identifier")
		}
		if verified {
			t.Fatalf("verified=true; want false for mismatching pubkey")
		}
	})

	t.Run("error", func(t *testing.T) {
		queryNamecoinIdentifier = func(ctx context.Context, id string) (*nostr.ProfilePointer, error) {
			return nil, errors.New("boom")
		}
		verified, ok := verifyNip05Namecoin(context.Background(), "alice@example.bit", expected)
		if !ok {
			t.Fatalf("ok=false; want true for .bit identifier (errors still indicate the path was taken)")
		}
		if verified {
			t.Fatalf("verified=true; want false on query error")
		}
	})
}

func TestIsNip05Valid_DotBitTakesNamecoinPath(t *testing.T) {
	expected := mustPubKey(t, "0000000000000000000000000000000000000000000000000000000000000003")

	prev := queryNamecoinIdentifier
	defer func() { queryNamecoinIdentifier = prev }()
	queryNamecoinIdentifier = func(ctx context.Context, id string) (*nostr.ProfilePointer, error) {
		return &nostr.ProfilePointer{PublicKey: expected}, nil
	}

	m := &sdk.ProfileMetadata{
		PubKey: expected,
		NIP05:  "alice@example.bit",
	}
	if !isNip05Valid(context.Background(), m) {
		t.Fatalf("isNip05Valid=false for matched .bit; want true")
	}

	m2 := &sdk.ProfileMetadata{
		PubKey: mustPubKey(t, "0000000000000000000000000000000000000000000000000000000000000004"),
		NIP05:  "alice@example.bit",
	}
	if isNip05Valid(context.Background(), m2) {
		t.Fatalf("isNip05Valid=true for mismatched .bit; want false")
	}
}

func TestIsNip05Valid_NilOrEmpty(t *testing.T) {
	if isNip05Valid(context.Background(), nil) {
		t.Fatalf("isNip05Valid(nil)=true; want false")
	}
	m := &sdk.ProfileMetadata{}
	if isNip05Valid(context.Background(), m) {
		t.Fatalf("isNip05Valid(empty)=true; want false")
	}
}
