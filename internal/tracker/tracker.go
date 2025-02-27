package tracker

import (
	"os"

	"github.com/go-chi/chi/v5"

	"github.com/baely/txn/internal/balance"
	"github.com/baely/txn/internal/tracker/database"
	"github.com/baely/txn/internal/tracker/server"
)

type tracker struct {
	db *database.Client

	transactionHandler balance.TransactionEventHandler
	chiRouter          chi.Router
}

func New() *tracker {
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")

	db, err := database.NewClient(dbUser, dbPassword, dbHost, dbPort, dbName)
	if err != nil {
		panic(err)
	}

	t := &tracker{
		db: db,
	}

	t.chiRouter = server.NewServer(db)

	return t
}

func (t *tracker) Chi() chi.Router {
	return nil
}

func (t *tracker) HandleEvent(transactionEvent balance.TransactionEvent) {
	server.ProcessEvent(t.db, transactionEvent)
}
