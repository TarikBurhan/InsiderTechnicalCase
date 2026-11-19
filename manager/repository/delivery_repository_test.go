package repository

import (
	"testing"

	"notifier/manager/dto"
)

func id(v int64) *int64 { return &v }

func TestDedupeContactsPhone(t *testing.T) {
	in := []dto.AudienceContact{
		{ContactID: id(1), Address: "+905550000001"},
		{ContactID: id(2), Address: " +905550000001 "}, // same phone, other contact
		{ContactID: id(3), Address: "+905550000002"},
		{ContactID: id(4), Address: "   "},
	}
	out := DedupeContacts(dto.ChannelSMS, in)
	if len(out) != 2 || *out[0].ContactID != 1 || *out[1].ContactID != 3 {
		t.Fatalf("unexpected result %+v", out)
	}
}

func TestDedupeContactsEmailCaseInsensitive(t *testing.T) {
	in := []dto.AudienceContact{
		{ContactID: id(7), Address: "User@Example.com"},
		{ContactID: id(8), Address: "user@example.com"},
	}
	out := DedupeContacts(dto.ChannelEmail, in)
	if len(out) != 1 || *out[0].ContactID != 7 || out[0].Address != "User@Example.com" {
		t.Fatalf("unexpected result %+v", out)
	}
}

func TestDedupeContactsPushIsCaseSensitive(t *testing.T) {
	in := []dto.AudienceContact{{Address: "tokenABC123"}, {Address: "tokenabc123"}}
	if out := DedupeContacts(dto.ChannelPush, in); len(out) != 2 {
		t.Fatalf("device tokens are case-sensitive, got %+v", out)
	}
}
