package telegram

import (
	"strings"
	"testing"

	"github.com/rawizhere/gosift/internal/models"
)

func TestFormatCardFitsCaption(t *testing.T) {
	o := models.Offer{Store: "komissionki", Title: strings.Repeat("я", 5000), URL: "https://x.ru"}
	if n := len([]rune(formatCard(o))); n > 1024 {
		t.Errorf("card too long for a caption: %d runes", n)
	}
}

func TestMakeTextCardsBatching(t *testing.T) {
	cards := []string{
		"<b>Магазин:</b> komissionki\n<b>Товар:</b> A\n",
		"<b>Магазин:</b> komissionki\n<b>Товар:</b> B\n",
	}
	got := makeTextCards(cards)
	if !strings.Contains(got, "A") || !strings.Contains(got, "B") {
		t.Errorf("cards not joined: %q", got)
	}
}

func TestMakeTextCardsOverflow(t *testing.T) {
	long := strings.Repeat("x", 3000)
	cards := []string{long, long, long}
	got := makeTextCards(cards)
	if len(got) > maxMessageLen+200 {
		t.Errorf("message too long: %d", len(got))
	}
	if !strings.Contains(got, "и ещё") {
		t.Errorf("overflow marker missing")
	}
}

func TestCategoryPath(t *testing.T) {
	if got := categoryPath("", 0, false); got != "" {
		t.Fatalf("empty: %q", got)
	}
	if got := categoryPath("", 45, false); got != "child:45" {
		t.Fatalf("child no title: %q", got)
	}
	if got := categoryPath("Ноутбуки", 45, false); got != "child:45:Ноутбуки" {
		t.Fatalf("child: %q", got)
	}
	if got := categoryPath("Компьютерная техника", 37, true); got != "parent:37:Компьютерная техника" {
		t.Fatalf("parent: %q", got)
	}
}

func TestCategoryTitle(t *testing.T) {
	if got := categoryTitle(""); got != "" {
		t.Fatalf("empty: %q", got)
	}
	if got := categoryTitle("child:45:Ноутбуки"); got != "Ноутбуки" {
		t.Fatalf("title: %q", got)
	}
	if got := categoryTitle("parent:37"); got != "категория 37" {
		t.Fatalf("no title: %q", got)
	}
}
