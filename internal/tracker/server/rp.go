package server

import (
	"fmt"
	"strings"

	"github.com/baely/txn/internal/balance"
	"github.com/baely/txn/internal/tracker/database"
	"github.com/baely/txn/internal/tracker/models"
)

func ProcessEvent(db *database.Client, event balance.TransactionEvent) error {
	if event.Transaction.Relationships.Category.Data == nil {
		return nil
	}

	category := event.Transaction.Relationships.Category.Data.Id

	switch category {
	case "restaurants-and-cafes":
		return transformRestaurantEvent(db, event)
	case "groceries":
		return transformGroceryEvent(db, event)
	}
	
	return nil
}

func transformRestaurantEvent(db *database.Client, event balance.TransactionEvent) error {
	desc := event.Transaction.Attributes.Description
	amt := event.Transaction.Attributes.Amount.ValueInBaseUnits
	if amt < 0 {
		amt = -amt
	}
	createdAt := event.Transaction.Attributes.CreatedAt

	type lookupKey struct {
		Description string
		Cost        int
	}
	lookup := map[lookupKey]int{
		{"Charlie Bit Me Cafe", 680}:  160,
		{"Charlie Bit Me Cafe", 700}:  160,
		{"Charlie Bit Me Cafe", 580}:  80,
		{"Georgie Boy Espresso", 550}: 160,
		{"Georgie Boy Espresso", 600}: 160,
		{"Chia Chia", 550}:            160,
		{"Chia Chia", 540}:            160,
		{"Chia Chia", 500}:            80,
		{"Chia Chia", 590}:            240,
		{"In a Rush", 560}:            160,
		{"Mr Summit", 550}:            160,
		{"The Other Brother", 600}:    160,
	}

	key := lookupKey{Description: desc, Cost: amt}
	amount, ok := lookup[key]
	if !ok {
		return nil
	}

	caffeineEvent := models.CaffeineEvent{
		Timestamp:   createdAt,
		Description: desc,
		Amount:      amount,
		Cost:        amt,
	}

	return db.AddEvent(caffeineEvent)
}

func transformGroceryEvent(db *database.Client, event balance.TransactionEvent) error {
	raw := event.Transaction.Attributes.RawText
	amt := event.Transaction.Attributes.Amount.ValueInBaseUnits
	if amt < 0 {
		amt = -amt
	}
	createdAt := event.Transaction.Attributes.CreatedAt

	fmt.Println("Handling grocery event", raw, amt)

	if raw == nil {
		fmt.Println("Raw text is nil")
		return nil
	}

	rawText := strings.ToUpper(*raw)

	if !strings.Contains(rawText, "WOOLWORTHS") || !strings.Contains(rawText, "DOCK") {
		fmt.Println("Raw text does not contain Woolworths Dock")
		return nil
	}

	if amt < 200 || amt > 700 {
		fmt.Println("Amount is not between 200 and 700")
		return nil
	}

	caffeineEvent := models.CaffeineEvent{
		Timestamp:   createdAt,
		Description: "Dare NAS Intense Espresso",
		Amount:      260,
		Cost:        amt,
	}

	return db.AddEvent(caffeineEvent)
}
