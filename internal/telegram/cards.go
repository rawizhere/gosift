package telegram

import (
	"context"
	"fmt"
	"html"
	"strings"

	"github.com/mymmrac/telego"
	"github.com/shopspring/decimal"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/rawizhere/gosift/internal/models"
)

var printer = message.NewPrinter(language.Russian)

const maxMessageLen = 4096

func (b *Bot) SendCards(ctx context.Context, chatID int64, offers []models.Offer) error {
	cards := make([]string, 0, len(offers))
	for _, o := range offers {
		cards = append(cards, formatCard(o))
	}
	var sb strings.Builder
	for i, card := range cards {
		if sb.Len()+len(card) > maxMessageLen {
			fmt.Fprintf(&sb, "\n…и ещё %d", len(cards)-i)
			break
		}
		sb.WriteString(card)
	}
	if sb.Len() == 0 && len(cards) > 0 {
		sb.WriteString(truncate(cards[0], maxMessageLen-12))
		if more := len(cards) - 1; more > 0 {
			fmt.Fprintf(&sb, "\n…и ещё %d", more)
		}
	}
	if sb.Len() == 0 {
		return nil
	}
	_, err := b.bot.SendMessage(ctx, &telego.SendMessageParams{
		ChatID:    telego.ChatID{ID: chatID},
		Text:      sb.String(),
		ParseMode: "HTML",
	})
	return err
}

func (b *Bot) SendAlert(ctx context.Context, chatID int64, store string, err error) error {
	text := fmt.Sprintf("Магазин %s временно недоступен: %s", html.EscapeString(store), html.EscapeString(err.Error()))
	_, sendErr := b.bot.SendMessage(ctx, &telego.SendMessageParams{
		ChatID: telego.ChatID{ID: chatID},
		Text:   text,
	})
	return sendErr
}

func formatCard(o models.Offer) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "<b>Магазин:</b> %s\n", html.EscapeString(o.Store))
	fmt.Fprintf(&sb, "<b>Товар:</b> %s\n", html.EscapeString(o.Title))
	price := formatPrice(o.Price)
	if o.OldPrice != nil {
		price += fmt.Sprintf(" (было %s)", formatPrice(*o.OldPrice))
	}
	fmt.Fprintf(&sb, "<b>Цена:</b> %s\n", price)
	fmt.Fprintf(&sb, "<a href=\"%s\">Ссылка</a>\n\n", html.EscapeString(o.URL))
	return sb.String()
}

func formatPrice(v decimal.Decimal) string {
	if v.IsInteger() {
		return printer.Sprintf("%d ₽", v.IntPart())
	}
	return printer.Sprintf("%s ₽", v.StringFixed(2))
}

func truncate(s string, limit int) string {
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit]) + "…"
}
