package queries

import (
	"strings"
	"testing"
	"time"

	"notifier/panel/dto"
)

func TestBuildListNotifications_NoFilters(t *testing.T) {
	list, count, args := BuildListNotifications(dto.NotificationFilter{Limit: 20})
	if strings.Contains(list, "WHERE") || strings.Contains(count, "WHERE") {
		t.Fatalf("unexpected WHERE clause: %s", list)
	}
	if !strings.Contains(list, "LIMIT $1 OFFSET $2") || len(args) != 0 {
		t.Fatalf("unexpected placeholders: %s %v", list, args)
	}
}

func TestBuildListNotifications_AllFilters(t *testing.T) {
	from := time.Now().Add(-time.Hour)
	to := time.Now()
	list, count, args := BuildListNotifications(dto.NotificationFilter{
		Status: "SENDING", Channel: "sms", Priority: "high", BatchID: "b", From: &from, To: &to,
	})
	for _, want := range []string{"status = $1", "channel = $2", "priority = $3", "batch_id = $4::uuid", "created_at >= $5", "created_at <= $6"} {
		if !strings.Contains(count, want) || !strings.Contains(list, want) {
			t.Fatalf("missing %q in %s", want, list)
		}
	}
	if !strings.Contains(list, "LIMIT $7 OFFSET $8") || len(args) != 6 {
		t.Fatalf("unexpected pagination placeholders: %s (%d args)", list, len(args))
	}
}

func TestDeliveryTableWhitelist(t *testing.T) {
	for ch, want := range map[string]string{"sms": "sms_deliveries", "email": "email_deliveries", "push": "push_deliveries"} {
		if got, err := DeliveryTable(ch); err != nil || got != want {
			t.Fatalf("DeliveryTable(%q) = %q, %v", ch, got, err)
		}
	}
	if _, err := DeliveryTable("sms_deliveries; drop table x"); err == nil {
		t.Fatal("unknown channels must be rejected")
	}
}
