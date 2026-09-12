package storeflow

import (
	"context"
	"testing"

	"example.com/store-verification-mail/internal/infrai"
)

type recordingSender struct {
	mail infrai.Email
	key  string
}

func (s *recordingSender) SendEmail(_ context.Context, mail infrai.Email, key string) (infrai.SendResult, error) {
	s.mail, s.key = mail, key
	return infrai.SendResult{MessageID: "msg-test"}, nil
}

func TestOrderUpdateDecision(t *testing.T) {
	tests := []struct {
		name        string
		update      OrderUpdate
		wantSubject string
		wantBody    string
		wantKey     string
	}{
		{"checkout", OrderUpdate{"A-42", "buyer@example.com", StageCheckout, "", ""}, "Order A-42 confirmed", "Checkout accepted. We are preparing your order.", "order:A-42:checkout"},
		{"fulfillment", OrderUpdate{"A-42", "buyer@example.com", StageFulfilled, "", "TRACK-7"}, "Order A-42 shipped", "Your order shipped. Tracking code: TRACK-7", "order:A-42:fulfilled"},
		{"receipt", OrderUpdate{"A-42", "buyer@example.com", StageReceipt, "$19.00", ""}, "Receipt for order A-42", "Payment received. Total: $19.00", "order:A-42:receipt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := &recordingSender{}
			mailer := NewMailer(sender, "https://shop.example.com")
			result, err := mailer.SendOrderUpdate(context.Background(), tt.update)
			if err != nil {
				t.Fatal(err)
			}
			if sender.mail.Subject != tt.wantSubject || sender.mail.Body != tt.wantBody || sender.key != tt.wantKey {
				t.Fatalf("mail = %#v, key = %q", sender.mail, sender.key)
			}
			if result.MessageID != "msg-test" {
				t.Fatalf("message id = %q", result.MessageID)
			}
		})
	}
}

func TestVerificationLinkIsEscapedAndStable(t *testing.T) {
	sender := &recordingSender{}
	mailer := NewMailer(sender, "https://shop.example.com")
	_, err := mailer.SendVerification(context.Background(), "customer-7", "buyer@example.com", "a+b&c")
	if err != nil {
		t.Fatal(err)
	}
	wantHTML := `<p>Confirm your email before checkout:</p><p><a href="https://shop.example.com/verify?token=a%2Bb%26c">Verify email</a></p>`
	if sender.mail.HTML != wantHTML || sender.key != "signup-verification:customer-7" {
		t.Fatalf("html = %q, key = %q", sender.mail.HTML, sender.key)
	}
}
