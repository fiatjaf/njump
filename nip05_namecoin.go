package main

import (
	"context"
	"strings"
	"time"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip05"
	"fiatjaf.com/nostr/sdk"
	"github.com/mstrofnone/nostrlib-nip05-namecoin/namecoin"
)

// nip05IdentifierURL returns the right outbound link for a NIP-05 identifier:
// a Namecoin explorer URL for `.bit`, otherwise the SDK's HTTPS well-known URL.
func nip05IdentifierURL(identifier string) string {
	if u := nip05NamecoinIdentifierToURL(identifier); u != "" {
		return u
	}
	return nip05.IdentifierToURL(identifier)
}

// queryNamecoinIdentifier is overridable for tests.
var queryNamecoinIdentifier = namecoin.QueryIdentifier

// nip05NamecoinTimeout matches the 5s budget used elsewhere for NIP-05 work.
const nip05NamecoinTimeout = 5 * time.Second

// verifyNip05Namecoin checks a Namecoin-backed NIP-05 identifier against an
// expected pubkey. It returns (verified, ok) where ok=false means the caller
// should fall back to the regular DNS-based NIP-05 verification path.
func verifyNip05Namecoin(ctx context.Context, identifier string, pubkey nostr.PubKey) (verified bool, ok bool) {
	if !namecoin.IsDotBit(identifier) {
		return false, false
	}
	cctx, cancel := context.WithTimeout(ctx, nip05NamecoinTimeout)
	defer cancel()
	pp, err := queryNamecoinIdentifier(cctx, identifier)
	if err != nil || pp == nil {
		return false, true
	}
	return pp.PublicKey == pubkey, true
}

// isNip05Valid is the unified NIP-05 validity check. For `.bit` identifiers
// it resolves through Namecoin (ElectrumX); otherwise it defers to the SDK's
// standard DNS path.
func isNip05Valid(ctx context.Context, m *sdk.ProfileMetadata) bool {
	if m == nil || m.NIP05 == "" {
		return false
	}
	if verified, ok := verifyNip05Namecoin(ctx, m.NIP05, m.PubKey); ok {
		return verified
	}
	return m.NIP05Valid(ctx)
}

// nip05NamecoinIdentifierToURL returns a sovereign explorer URL for a `.bit`
// NIP-05 identifier, or "" if the identifier is not `.bit`. The empty return
// signals callers to fall back to nip05.IdentifierToURL.
func nip05NamecoinIdentifierToURL(identifier string) string {
	if !namecoin.IsDotBit(identifier) {
		return ""
	}
	name := identifier
	if i := strings.IndexByte(name, '@'); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimSuffix(name, ".bit")
	name = strings.TrimPrefix(name, "d/")
	if name == "" {
		return ""
	}
	return "https://explorer.namecoin.org/name/d/" + name
}
