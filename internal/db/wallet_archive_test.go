package db

import (
	"testing"
	"time"

	"eve-flipper/internal/esi"
)

func TestWalletArchiveStoresAndReadsTransactionsAndJournal(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()

	userID := "wallet-archive-user"
	characterID := int64(9001)

	txStats, err := d.UpsertWalletTransactionsForUser(userID, characterID, []esi.WalletTransaction{
		{
			TransactionID: 1001,
			Date:          "2026-05-01T10:00:00Z",
			TypeID:        34,
			LocationID:    60003760,
			UnitPrice:     5.25,
			Quantity:      100,
			IsBuy:         true,
		},
		{
			TransactionID: 1002,
			Date:          "2026-05-02T10:00:00Z",
			TypeID:        34,
			LocationID:    60003760,
			UnitPrice:     7.00,
			Quantity:      100,
			IsBuy:         false,
		},
	})
	if err != nil {
		t.Fatalf("UpsertWalletTransactionsForUser: %v", err)
	}
	if txStats.LiveRows != 2 || txStats.CharacterID != characterID {
		t.Fatalf("tx stats = %+v", txStats)
	}

	if _, err := d.UpsertWalletJournalForUser(userID, characterID, []esi.WalletJournalEntry{
		{
			ID:      2001,
			Date:    "2026-05-02T10:00:01Z",
			RefType: "market_transaction",
			Amount:  700,
			Balance: 1500,
		},
	}); err != nil {
		t.Fatalf("UpsertWalletJournalForUser: %v", err)
	}

	if err := d.UpdateWalletArchiveBalance(userID, characterID, 1500); err != nil {
		t.Fatalf("UpdateWalletArchiveBalance: %v", err)
	}

	since := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	txns, err := d.ListArchivedWalletTransactions(userID, []int64{characterID}, since, 100)
	if err != nil {
		t.Fatalf("ListArchivedWalletTransactions: %v", err)
	}
	if len(txns) != 2 {
		t.Fatalf("transactions len = %d, want 2", len(txns))
	}
	if txns[0].TransactionID != 1002 || txns[1].TransactionID != 1001 {
		t.Fatalf("transactions order = %+v", txns)
	}

	journal, err := d.ListArchivedWalletJournal(userID, []int64{characterID}, since, 100)
	if err != nil {
		t.Fatalf("ListArchivedWalletJournal: %v", err)
	}
	if len(journal) != 1 || journal[0].ID != 2001 {
		t.Fatalf("journal = %+v", journal)
	}

	stats, err := d.GetWalletArchiveStats(userID, []int64{characterID})
	if err != nil {
		t.Fatalf("GetWalletArchiveStats: %v", err)
	}
	if stats.TransactionRows != 2 || stats.JournalRows != 1 {
		t.Fatalf("archive stats = %+v", stats)
	}
	if stats.TransactionTurnoverISK != 1225 {
		t.Fatalf("transaction turnover = %v, want 1225", stats.TransactionTurnoverISK)
	}
	if stats.OldestTransactionDate != "2026-05-01T10:00:00Z" || stats.NewestTransactionDate != "2026-05-02T10:00:00Z" {
		t.Fatalf("transaction coverage = %s/%s", stats.OldestTransactionDate, stats.NewestTransactionDate)
	}
}

func TestWalletArchiveUpsertUpdatesExistingRows(t *testing.T) {
	d := openTestDB(t)
	defer d.Close()

	const userID = "wallet-archive-upsert"
	const characterID int64 = 42

	row := esi.WalletTransaction{
		TransactionID: 1,
		Date:          "2026-05-01T00:00:00Z",
		TypeID:        34,
		LocationID:    1,
		UnitPrice:     10,
		Quantity:      1,
		IsBuy:         true,
	}
	if _, err := d.UpsertWalletTransactionsForUser(userID, characterID, []esi.WalletTransaction{row}); err != nil {
		t.Fatalf("initial upsert: %v", err)
	}
	row.TypeName = "Tritanium"
	row.LocationName = "Jita IV - Moon 4"
	row.Quantity = 3
	if _, err := d.UpsertWalletTransactionsForUser(userID, characterID, []esi.WalletTransaction{row}); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	txns, err := d.ListArchivedWalletTransactions(userID, []int64{characterID}, time.Time{}, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(txns) != 1 {
		t.Fatalf("transactions len = %d, want 1", len(txns))
	}
	if txns[0].Quantity != 3 || txns[0].TypeName != "Tritanium" || txns[0].LocationName == "" {
		t.Fatalf("updated transaction = %+v", txns[0])
	}
}
