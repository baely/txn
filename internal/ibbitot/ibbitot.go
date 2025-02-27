package ibbitot

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/baely/balance/pkg/model"
	"github.com/go-chi/chi/v5"

	"github.com/baely/txn/internal/balance"
)

var loc, _ = time.LoadLocation("Australia/Melbourne")

type ibbitot struct {
	r chi.Router

	cachedTransaction model.TransactionResource
	indexPage         []byte
}

func New() *ibbitot {
	r := chi.NewRouter()

	i := &ibbitot{}

	r.HandleFunc("/raw", i.rawHandler)
	r.HandleFunc("/", i.indexHandler)
	r.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "image/png")
		w.Header().Add("Cache-Control", "public, max-age=604800, immutable")
		w.Write(coffeeCup)
	})

	i.r = r

	i.refreshPage()
	go i.dailyPageRefresher()

	return i
}

func (i *ibbitot) Chi() chi.Router {
	return i.r
}

func (i *ibbitot) HandleEvent(transactionEvent balance.TransactionEvent) {
	i.updatePresence(transactionEvent.Transaction)
}

var (
	//go:embed index.html
	indexHTML string

	//go:embed coffee-cup.png
	coffeeCup []byte
)

// getNow returns the current time with timezone. helper function because i kept having skill issues with tz
func getNow() time.Time {
	return time.Now().In(loc)
}

func (i *ibbitot) rawHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	latestTransaction := i.getLatest()
	w.Header().Add("Content-Type", "text/plain")
	w.Write([]byte(presentString(latestTransaction)))
}

func (i *ibbitot) indexHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("Request received")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Write(i.indexPage)
}

func (i *ibbitot) updatePresence(transaction model.TransactionResource) {
	if !check(transaction,
		amountBetween(-700, -400), // between -$7 and -$4
		//timeBetween(6, 12),                // between 6am and 12pm
		weekday(),                         // on a weekday
		notForeign(),                      // not a foreign transaction
		category("restaurants-and-cafes"), // in the restaurants-and-cafes category
	) {
		slog.Warn("Transaction does not meet criteria")
		return
	}

	i.store(transaction)
}

func present(latestTransaction model.TransactionResource) bool {
	return check(latestTransaction,
		fresh(),
	)
}

func presentString(latestTransaction model.TransactionResource) string {
	if present(latestTransaction) {
		return "yes"
	}
	return "no"
}

type decider func(model.TransactionResource) bool

func check(transaction model.TransactionResource, deciders ...decider) bool {
	for _, d := range deciders {
		if !d(transaction) {
			return false
		}
	}
	return true
}

func amountBetween(minBaseUnits, maxBaseUnits int) decider {
	return func(transaction model.TransactionResource) bool {
		valueInBaseUnits := transaction.Attributes.Amount.ValueInBaseUnits
		return valueInBaseUnits >= minBaseUnits && valueInBaseUnits <= maxBaseUnits
	}
}

func timeBetween(minHour, maxHour int) decider {
	return func(transaction model.TransactionResource) bool {
		hour := transaction.Attributes.CreatedAt.Hour()
		return hour >= minHour && hour <= maxHour
	}
}

func weekday() decider {
	return func(transaction model.TransactionResource) bool {
		day := transaction.Attributes.CreatedAt.Weekday()
		return day >= 1 && day <= 5
	}
}

func fresh() decider {
	return func(transaction model.TransactionResource) bool {
		return isToday(transaction)
	}
}

func notForeign() decider {
	return func(transaction model.TransactionResource) bool {
		return transaction.Attributes.ForeignAmount == nil
	}
}

func category(categoryId string) decider {
	return func(transaction model.TransactionResource) bool {
		if transaction.Relationships.Category.Data == nil {
			return false
		}

		return transaction.Relationships.Category.Data.Id == categoryId
	}
}

func isToday(transaction model.TransactionResource) bool {
	now := getNow()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return transaction.Attributes.CreatedAt.After(midnight)
}

func (i *ibbitot) getLatest() model.TransactionResource {
	return i.cachedTransaction
}

func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
}

func getReason(presence bool, t model.TransactionResource) string {
	if presence {
		amt := fmt.Sprintf("$%.2f", -float64(t.Attributes.Amount.ValueInBaseUnits)/100.0)
		p1 := fmt.Sprintf("%s at %s", t.Attributes.Description, t.Attributes.CreatedAt.In(loc).Format(time.Kitchen))
		return fmt.Sprintf("<img src=\"/favicon.ico\" />%s on %s", amt, p1)
	}

	return ""
}

func (i *ibbitot) store(transaction model.TransactionResource) {
	if i.cachedTransaction.Attributes.CreatedAt.After(transaction.Attributes.CreatedAt) {
		return
	}
	fmt.Printf("Cached transaction updated, %s on %s\n", transaction.Attributes.Description, transaction.Attributes.CreatedAt.Format(time.RFC1123))
	i.cachedTransaction = transaction
	i.refreshPage()
}

func (i *ibbitot) refreshPage() {
	latestTransaction := i.getLatest()
	title := presentString(latestTransaction)
	desc := getReason(present(latestTransaction), latestTransaction)
	replaced := i.replacePage([]byte(fmt.Sprintf(indexHTML, title, desc)))
	if replaced {
		fireSlack(title, desc)
	}
}

func (i *ibbitot) replacePage(b []byte) bool {
	old := i.indexPage
	i.indexPage = b
	return !bytes.Equal(old, b) // return true if the page was updated
}

func fireSlack(title, desc string) {
	u := os.Getenv("SLACK_WEBHOOK")
	u = strings.TrimSpace(u)
	type req struct {
		Status      string `json:"status"`
		Description string `json:"description"`
	}
	desc = strings.Replace(desc, "<img src=\"/favicon.ico\" />", "", -1)
	b, _ := json.Marshal(req{Status: title, Description: desc})
	resp, err := http.DefaultClient.Post(u, "application/json", bytes.NewReader(b))
	if err != nil {
		slog.Error("Error sending slack message", "error", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slog.Error("Error sending slack message", "status", resp.Status)
		return
	}
}

func (i *ibbitot) dailyPageRefresher() {
	ticker := make(chan time.Time)
	go runDailyTicker(ticker)
	for {
		<-ticker
		i.refreshPage()
	}
}

func runDailyTicker(ticker chan<- time.Time) {
	for {
		now := getNow()
		nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		duration := nextMidnight.Sub(now)
		slog.Warn(fmt.Sprintf("Current time: %s. Sleeping for: %s.", now, duration))
		time.Sleep(duration)
		// Sleep for a little longer
		time.Sleep(500 * time.Millisecond)
		ticker <- getNow()
	}
}
