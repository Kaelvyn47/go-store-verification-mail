package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	"example.com/store-verification-mail/internal/infrai"
	"example.com/store-verification-mail/internal/storeflow"
)

type server struct {
	mailer *storeflow.Mailer
}

type signupRequest struct {
	CustomerID string `json:"customer_id"`
	Email      string `json:"email"`
	Token      string `json:"token"`
}

type orderRequest struct {
	OrderID       string               `json:"order_id"`
	CustomerEmail string               `json:"customer_email"`
	Stage         storeflow.OrderStage `json:"stage"`
	Total         string               `json:"total"`
	TrackingCode  string               `json:"tracking_code"`
}

func main() {
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	publicURL := os.Getenv("PUBLIC_URL")
	if publicURL == "" {
		publicURL = "http://localhost:8080"
	}

	s := &server{mailer: storeflow.NewMailer(infrai.NewClient(apiKey), publicURL)}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /signup", s.signup)
	mux.HandleFunc("POST /orders/update", s.orderUpdate)
	log.Printf("storemail listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func (s *server) signup(w http.ResponseWriter, r *http.Request) {
	var input signupRequest
	if err := decode(r, &input); err != nil || input.CustomerID == "" || input.Email == "" || input.Token == "" {
		http.Error(w, "customer_id, email, and token are required", http.StatusBadRequest)
		return
	}
	result, err := s.mailer.SendVerification(r.Context(), input.CustomerID, input.Email, input.Token)
	respond(w, result, err)
}

func (s *server) orderUpdate(w http.ResponseWriter, r *http.Request) {
	var input orderRequest
	if err := decode(r, &input); err != nil || input.OrderID == "" || input.CustomerEmail == "" {
		http.Error(w, "order_id and customer_email are required", http.StatusBadRequest)
		return
	}
	result, err := s.mailer.SendOrderUpdate(r.Context(), storeflow.OrderUpdate{
		OrderID: input.OrderID, CustomerEmail: input.CustomerEmail, Stage: input.Stage,
		Total: input.Total, TrackingCode: input.TrackingCode,
	})
	respond(w, result, err)
}

func decode(r *http.Request, dst any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) == nil {
		return errors.New("multiple JSON values")
	}
	return nil
}

func respond(w http.ResponseWriter, result infrai.SendResult, err error) {
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	json.NewEncoder(w).Encode(result)
}
