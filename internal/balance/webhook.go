package balance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/baely/balance/pkg/model"
	"github.com/go-chi/chi/v5"
)

type webhook struct {
	upClient *UpClient
	rawChan  chan []byte

	chiRouter chi.Router

	transactionHandlers []TransactionEventHandler
}

func New() *webhook {
	w := &webhook{
		upClient: NewUpClient(os.Getenv("UP_ACCESS_TOKEN")),
		rawChan:  make(chan []byte),
	}

	r := chi.NewRouter()
	r.HandleFunc("/up/event", w.webhook)
	r.HandleFunc("/event", w.webhook)
	w.chiRouter = r

	go w.listen()

	return w
}

func (w *webhook) Chi() chi.Router {
	return w.chiRouter
}

func (w *webhook) RegisterHandler(handler TransactionEventHandler) {
	w.transactionHandlers = append(w.transactionHandlers, handler)
}

func (w *webhook) webhook(writer http.ResponseWriter, request *http.Request) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		fmt.Println("read error:", err)
		http.Error(writer, "", http.StatusInternalServerError)
		return
	}

	if !ValidateWebhookEvent(
		body,
		request.Header.Get("X-Up-Authenticity-Signature"),
	) {
		http.Error(writer, "", http.StatusUnauthorized)
		fmt.Println("error: failed to validate incoming event")
		return
	}

	w.rawChan <- body
}

func (w *webhook) listen() {
	for raw := range w.rawChan {
		w.consume(raw)
	}
}

func (w *webhook) consume(raw []byte) {
	ctx := context.Background()
	event := toEvent(raw)
	fmt.Println(event)

	// Retrieve transaction details
	eventTransaction := event.Data.Relationships.Transaction

	if eventTransaction == nil {
		fmt.Println("no transaction details")
		return
	}
	transaction, err := w.upClient.GetTransaction(ctx, eventTransaction.Data.Id)
	if err != nil {
		fmt.Println("error retrieving transaction:", err)
		return
	}

	// Retrieve account details
	accountId := transaction.Relationships.Account.Data.Id
	account, err := w.upClient.GetAccount(ctx, accountId)
	if err != nil {
		fmt.Println("error retrieving account:", err)
		return
	}

	data := TransactionEvent{
		Account:     account,
		Transaction: transaction,
	}

	for _, handler := range w.transactionHandlers {
		handler.HandleEvent(data)
	}

	return
}

func toEvent(value []byte) model.WebhookEventCallback {
	event := model.WebhookEventCallback{}
	_ = json.Unmarshal(value, &event)
	return event
}
