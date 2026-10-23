package telegram

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"image/jpeg"
	"net/url"
	"strings"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoutil"
	"github.com/shopspring/decimal"
	"golang.org/x/image/webp"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/rawizhere/gosift/internal/models"
)

var printer = message.NewPrinter(language.Russian)

const (
	maxMessageLen = 4096
	// maxTitleLen keeps a card under Telegram's 1024-char caption limit.
	maxTitleLen = 600
)

// SendCards sends offers as photo or text messages.
func (b *Bot) SendCards(ctx context.Context, chatID int64, offers []models.Offer) error {
	var batch []string
	flushText := func() error {
		if len(batch) == 0 {
			return nil
		}
		text := makeTextCards(batch)
		batch = nil
		_, err := b.bot.SendMessage(ctx, &telego.SendMessageParams{
			ChatID:             telego.ChatID{ID: chatID},
			Text:               text,
			ParseMode:          "HTML",
			LinkPreviewOptions: &telego.LinkPreviewOptions{IsDisabled: true},
		})
		return err
	}
	for _, o := range offers {
		if len(o.Images) > 0 {
			if err := flushText(); err != nil {
				return err
			}
			if err := b.sendOfferWithImages(ctx, chatID, o); err != nil {
				b.log.Warn("send offer with images failed, keep as text",
					"store", o.Store, "key", o.Key, "error", err)
				batch = append(batch, formatCard(o))
			}
			continue
		}
		batch = append(batch, formatCard(o))
	}
	return flushText()
}

// makeTextCards joins card texts into one message.
func makeTextCards(cards []string) string {
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
	return sb.String()
}

// sendOfferWithImages downloads photos and sends them with the card caption.
func (b *Bot) sendOfferWithImages(ctx context.Context, chatID int64, o models.Offer) error {
	caption := formatCard(o)
	jpegs := make([][]byte, 0, len(o.Images))
	host := "" // resolved CDN host for this offer
	for _, imgURL := range o.Images {
		data, err := b.fetchImage(ctx, imgURL, &host)
		if err != nil {
			b.log.Debug("image download failed", "url", imgURL, "error", err)
			continue
		}
		jpg, err := toJPEG(data)
		if err != nil {
			b.log.Debug("image decode failed", "url", imgURL, "error", err)
			continue
		}
		jpegs = append(jpegs, jpg)
	}
	if len(jpegs) == 0 {
		return fmt.Errorf("no usable images (%d failed)", len(o.Images))
	}
	if len(jpegs) == 1 {
		_, err := b.bot.SendPhoto(ctx, &telego.SendPhotoParams{
			ChatID:    telego.ChatID{ID: chatID},
			Photo:     telegoutil.FileFromBytes(jpegs[0], "photo.jpg"),
			Caption:   caption,
			ParseMode: "HTML",
		})
		return err
	}
	media := make([]telego.InputMedia, 0, len(jpegs))
	for i, jpg := range jpegs {
		photo := &telego.InputMediaPhoto{
			Type:  telego.MediaTypePhoto,
			Media: telegoutil.FileFromBytes(jpg, fmt.Sprintf("photo%d.jpg", i)),
		}
		if i == 0 {
			photo.Caption = caption
			photo.ParseMode = "HTML"
		}
		media = append(media, photo)
	}
	_, err := b.bot.SendMediaGroup(ctx, &telego.SendMediaGroupParams{
		ChatID: telego.ChatID{ID: chatID},
		Media:  media,
	})
	return err
}

// fetchImage downloads an image from any configured CDN host.
func (b *Bot) fetchImage(ctx context.Context, imgURL string, resolvedHost *string) ([]byte, error) {
	if *resolvedHost != "" {
		return b.hc.GetBytes(ctx, withHost(imgURL, *resolvedHost))
	}
	var lastErr error
	for _, host := range b.cdnHosts {
		data, err := b.hc.GetBytes(ctx, withHost(imgURL, host))
		if err == nil {
			*resolvedHost = host
			return data, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// withHost swaps the host of an absolute URL.
func withHost(raw, host string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	u.Host = host
	return u.String()
}

// toJPEG converts webp to JPEG for Telegram uploads.
func toJPEG(data []byte) ([]byte, error) {
	img, err := webp.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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
	if o.Category != "" {
		fmt.Fprintf(&sb, "<b>Категория:</b> %s\n", html.EscapeString(o.Category))
	}
	fmt.Fprintf(&sb, "<b>Товар:</b> %s\n", html.EscapeString(truncate(o.Title, maxTitleLen)))
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
