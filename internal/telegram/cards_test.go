package telegram

import (
	"strings"
	"testing"
)

func TestTruncateHTMLShort(t *testing.T) {
	in := "<b>Магазин:</b> komissionki"
	if got := truncateHTML(in, 100); got != in {
		t.Errorf("truncateHTML = %q", got)
	}
}

func TestTruncateHTMLBalancesTags(t *testing.T) {
	in := `<b>Товар:</b> MacBook Pro 16 M3 Pro 2023 <a href="https://x.ru">Ссылка</a>`
	got := truncateHTML(in, 40)
	if !strings.HasPrefix(got, "<b>Товар:</b>") {
		t.Errorf("prefix lost: %q", got)
	}
	opens := strings.Count(got, "<b>") + strings.Count(got, "<a ")
	closes := strings.Count(got, "</b>") + strings.Count(got, "</a>")
	if opens != closes {
		t.Errorf("unbalanced tags in %q: opens=%d closes=%d", got, opens, closes)
	}
}

func TestTruncateHTMLBalancesWithoutLink(t *testing.T) {
	in := "<b>Товар:</b> MacBook Pro 16 M3 Pro 2023"
	got := truncateHTML(in, 15)
	if !strings.HasPrefix(got, "<b>Товар:</b>") {
		t.Errorf("got %q", got)
	}
	if strings.Count(got, "<b>") != strings.Count(got, "</b>") {
		t.Errorf("unbalanced b tags: %q", got)
	}
	// a link cut mid-tag keeps the link open and closes it at the end
	in2 := `<a href="https://x.ru">длинный текст ссылки</a>`
	got2 := truncateHTML(in2, 45)
	if !strings.HasPrefix(got2, `<a href="https://x.ru">`) {
		t.Errorf("got2 %q", got2)
	}
	if !strings.HasSuffix(got2, "</a>") {
		t.Errorf("missing closing a: %q", got2)
	}
	// a cut right inside a tag must not leave a bare '<' in the output
	got3 := truncateHTML(in2, 10)
	if strings.Contains(got3, "<") {
		t.Errorf("bare '<' left in output: %q", got3)
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
