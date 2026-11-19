package rabbitmq

import "testing"

func TestQueueForChannel(t *testing.T) {
	cases := map[string]string{"sms": QueueSMS, "email": QueueEmail, "push": QueuePush}
	for ch, want := range cases {
		got, err := QueueForChannel(ch)
		if err != nil || got != want {
			t.Fatalf("QueueForChannel(%q) = %q, %v; want %q", ch, got, err, want)
		}
	}
	if _, err := QueueForChannel("fax"); err == nil {
		t.Fatal("expected error for unknown channel")
	}
}

func TestPriorityValue(t *testing.T) {
	if PriorityValue("high") <= PriorityValue("normal") || PriorityValue("normal") <= PriorityValue("low") {
		t.Fatal("priorities must be ordered high > normal > low")
	}
	if PriorityValue("high") > MaxPriority {
		t.Fatal("priority exceeds queue max priority")
	}
	if PriorityValue("unknown") != PriorityValue("normal") {
		t.Fatal("unknown priority should default to normal")
	}
}

func TestURL(t *testing.T) {
	got := URL("user", "p@ss", "rabbit", "5672", "/")
	if got != "amqp://user:p%40ss@rabbit:5672/" {
		t.Fatalf("unexpected url %s", got)
	}
}
