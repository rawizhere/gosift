package repo

import (
	"context"
	"testing"

	"github.com/rawizhere/gosift/internal/db"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return NewStore(sqlDB)
}

func TestShouldNotifyOffer(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// first sighting notifies
	ok, err := s.ShouldNotifyOffer(ctx, 1, "komissionki|barcode-1", "1000")
	if err != nil || !ok {
		t.Fatalf("first sighting: ok=%v err=%v", ok, err)
	}

	// same price does not notify
	ok, err = s.ShouldNotifyOffer(ctx, 1, "komissionki|barcode-1", "1000")
	if err != nil || ok {
		t.Fatalf("same price: ok=%v err=%v", ok, err)
	}

	// price drop notifies
	ok, err = s.ShouldNotifyOffer(ctx, 1, "komissionki|barcode-1", "900")
	if err != nil || !ok {
		t.Fatalf("price drop: ok=%v err=%v", ok, err)
	}

	// price increase does not notify
	ok, err = s.ShouldNotifyOffer(ctx, 1, "komissionki|barcode-1", "1200")
	if err != nil || ok {
		t.Fatalf("price increase: ok=%v err=%v", ok, err)
	}

	// drop below the increased price notifies again
	ok, err = s.ShouldNotifyOffer(ctx, 1, "komissionki|barcode-1", "1100")
	if err != nil || !ok {
		t.Fatalf("drop after increase: ok=%v err=%v", ok, err)
	}

	// other chat and offer stay independent
	ok, err = s.ShouldNotifyOffer(ctx, 2, "komissionki|barcode-1", "1000")
	if err != nil || !ok {
		t.Fatalf("other chat: ok=%v err=%v", ok, err)
	}
	ok, err = s.ShouldNotifyOffer(ctx, 1, "komissionki|barcode-2", "1000")
	if err != nil || !ok {
		t.Fatalf("other offer: ok=%v err=%v", ok, err)
	}
}
