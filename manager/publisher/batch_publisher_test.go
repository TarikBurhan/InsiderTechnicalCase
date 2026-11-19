package publisher

import (
	"testing"

	"notifier/manager/dto"
	"notifier/pkg/rabbitmq"
)

func TestChunk(t *testing.T) {
	items := make([]int, 250)
	chunks := Chunk(items, 100)
	if len(chunks) != 3 || len(chunks[0]) != 100 || len(chunks[2]) != 50 {
		t.Fatalf("unexpected chunking: %d chunks", len(chunks))
	}
	if len(Chunk([]int{}, 100)) != 0 {
		t.Fatal("empty input must produce no chunks")
	}
}

func TestTargetQueue(t *testing.T) {
	q, _ := TargetQueue(dto.Notification{Channel: "email"})
	if q != rabbitmq.QueueEmail {
		t.Fatalf("got %s", q)
	}
	q, _ = TargetQueue(dto.Notification{Channel: "email", CustomizedContent: true})
	if q != rabbitmq.QueueCustomizer {
		t.Fatalf("customized content must go to the customizer, got %s", q)
	}
}

func TestBuildMessage(t *testing.T) {
	id := int64(7)
	msg := BuildMessage(Batch{
		Notification: dto.Notification{ID: "n1", Channel: "sms", Priority: "high", CorrelationID: "c1"},
		Content:      dto.MessageContent{Body: "hi"},
		BatchNo:      3,
		Attempt:      1,
		Targets:      []dto.DeliveryTarget{{DeliveryID: 1, ContactID: &id, Recipient: "+905550000001"}},
	})
	if msg.NotificationID != "n1" || msg.BatchNo != 3 || len(msg.Recipients) != 1 || msg.Recipients[0].To != "+905550000001" {
		t.Fatalf("unexpected message %+v", msg)
	}
	if msg.MessageID == "" {
		t.Fatal("message id must be set")
	}
}
