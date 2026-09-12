package storeflow

import (
	"context"
	"fmt"
	"html"
	"net/url"

	"example.com/store-verification-mail/internal/infrai"
)

type Sender interface {
	SendEmail(context.Context, infrai.Email, string) (infrai.SendResult, error)
}

type Mailer struct {
	sender    Sender
	publicURL string
}

func NewMailer(sender Sender, publicURL string) *Mailer {
	return &Mailer{sender: sender, publicURL: publicURL}
}

func (m *Mailer) SendVerification(ctx context.Context, customerID, address, token string) (infrai.SendResult, error) {
	link := m.publicURL + "/verify?token=" + url.QueryEscape(token)
	mail := infrai.Email{
		To:      address,
		Subject: "Verify your email for Northwind Shop",
		HTML:    fmt.Sprintf(`<p>Confirm your email before checkout:</p><p><a href="%s">Verify email</a></p>`, html.EscapeString(link)),
	}
	return m.sender.SendEmail(ctx, mail, "signup-verification:"+customerID)
}

type OrderStage string

const (
	StageCheckout  OrderStage = "checkout"
	StageFulfilled OrderStage = "fulfilled"
	StageReceipt   OrderStage = "receipt"
)

type OrderUpdate struct {
	OrderID       string
	CustomerEmail string
	Stage         OrderStage
	Total         string
	TrackingCode  string
}

func (m *Mailer) SendOrderUpdate(ctx context.Context, update OrderUpdate) (infrai.SendResult, error) {
	subject, body, err := renderOrderUpdate(update)
	if err != nil {
		return infrai.SendResult{}, err
	}
	return m.sender.SendEmail(ctx, infrai.Email{
		To:      update.CustomerEmail,
		Subject: subject,
		Body:    body,
	}, "order:"+update.OrderID+":"+string(update.Stage))
}

func renderOrderUpdate(update OrderUpdate) (string, string, error) {
	switch update.Stage {
	case StageCheckout:
		return "Order " + update.OrderID + " confirmed", "Checkout accepted. We are preparing your order.", nil
	case StageFulfilled:
		return "Order " + update.OrderID + " shipped", "Your order shipped. Tracking code: " + update.TrackingCode, nil
	case StageReceipt:
		return "Receipt for order " + update.OrderID, "Payment received. Total: " + update.Total, nil
	default:
		return "", "", fmt.Errorf("unknown order stage %q", update.Stage)
	}
}
