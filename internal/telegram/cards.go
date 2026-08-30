package telegram

import (
	"context"
	"fmt"
	"html"
	"strconv"
	"strings"

	"github.com/mymmrac/telego"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/rawizhere/gosift/internal/models"
)

var printer = message.NewPrinter(language.Russian)

const maxMessageLen = 4096

func (b *Bot) SendCards(ctx context.Context, chatID int64, offers []models.Offer) error {
	for _, chunk := range chunks(offers, maxMessageLen) {
		var sb strings.Builder
		for _, o := range chunk {
			fmt.Fprintf(&sb, "<b>%s</b>\n%s\n<b>%s</b>\n<a href=\"%s\">Открыть</a>\n\n",
				html.EscapeString(o.Title), html.EscapeString(o.City), formatPrice(o.Price), html.EscapeString(o.URL))
		}
		if _, err := b.bot.SendMessage(ctx, &telego.SendMessageParams{
			ChatID:    telego.ChatID{ID: chatID},
			Text:      sb.String(),
			ParseMode: "HTML",
		}); err != nil {
			return err
		}
	}
	return nil
}

func (b *Bot) SendAlert(ctx context.Context, chatID int64, store string, err error) error {
	text := fmt.Sprintf("Магазин %s временно недоступен: %s", html.EscapeString(store), html.EscapeString(err.Error()))
	_, sendErr := b.bot.SendMessage(ctx, &telego.SendMessageParams{
		ChatID: telego.ChatID{ID: chatID},
		Text:   text,
	})
	return sendErr
}

func chunks(offers []models.Offer, limit int) [][]models.Offer {
	out := [][]models.Offer{}
	cur := []models.Offer{}
	size := 0
	for _, o := range offers {
		card := len(o.Title) + len(o.City) + len(o.Price) + len(o.URL) + 40
		if size+card > limit && len(cur) > 0 {
			out = append(out, cur)
			cur = []models.Offer{}
			size = 0
		}
		cur = append(cur, o)
		size += card
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out
}

func formatPrice(price string) string {
	if price == "" {
		return "—"
	}
	if v, err := strconv.ParseInt(price, 10, 64); err == nil {
		return printer.Sprintf("%d ₽", v)
	}
	if v, err := strconv.ParseFloat(price, 64); err == nil {
		return printer.Sprintf("%.2f ₽", v)
	}
	return html.EscapeString(price) + " ₽"
}
