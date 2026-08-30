package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"strconv"
	"strings"

	"github.com/mymmrac/telego"
	te "github.com/mymmrac/telego/telegoutil"

	"github.com/rawizhere/gosift/internal/models"
)

type dialogData struct {
	Mode     string `json:"mode"`
	RuleID   int64  `json:"rule_id,omitempty"`
	Store    string `json:"store,omitempty"`
	Query    string `json:"query,omitempty"`
	City     string `json:"city,omitempty"`
	MinPrice string `json:"min_price,omitempty"`
	MaxPrice string `json:"max_price,omitempty"`
	Field    string `json:"field,omitempty"`
}

type storeOption struct {
	Key   string
	Label string
}

var stores = []storeOption{{Key: "komissionki", Label: "komissionki.ru"}}

func (b *Bot) cmdAdd(ctx context.Context, msg *telego.Message) {
	b.startDialog(ctx, msg.Chat.ID, "add.store", dialogData{Mode: "add"})
	b.askStore(ctx, msg.Chat.ID)
}

func (b *Bot) cmdEdit(ctx context.Context, msg *telego.Message, arg string) {
	rules, err := b.store.ListRulesByUser(ctx, msg.From.ID)
	if err != nil {
		b.send(ctx, msg.Chat.ID, "Ошибка при чтении правил.")
		return
	}
	if len(rules) == 0 {
		b.send(ctx, msg.Chat.ID, "Правил нет. Добавь через /add.")
		return
	}
	b.startDialog(ctx, msg.Chat.ID, "edit.choose", dialogData{Mode: "edit"})
	b.askRule(ctx, msg.Chat.ID, rules)
}

func (b *Bot) handleDialogStep(ctx context.Context, msg *telego.Message, text string) {
	state, raw, err := b.store.GetDialogState(ctx, msg.Chat.ID)
	if err != nil {
		return
	}
	d := dialogData{}
	_ = json.Unmarshal([]byte(raw), &d)

	switch state {
	case "add.query":
		d.Query = text
		b.setDialog(ctx, msg.Chat.ID, "add.city", d)
		b.askCity(ctx, msg.Chat.ID)
	case "add.city":
		d.City = parseCity(text)
		b.setDialog(ctx, msg.Chat.ID, "add.min_price", d)
		b.askPrice(ctx, msg.Chat.ID, "Мин. цена (0 — без min):")
	case "add.min_price":
		if !validPrice(text) {
			b.send(ctx, msg.Chat.ID, "Введи число, например 10000.")
			return
		}
		d.MinPrice = normalizePrice(text)
		b.setDialog(ctx, msg.Chat.ID, "add.max_price", d)
		b.askPrice(ctx, msg.Chat.ID, "Макс. цена (0 — без max):")
	case "add.max_price":
		if !validPrice(text) {
			b.send(ctx, msg.Chat.ID, "Введи число, например 50000.")
			return
		}
		d.MaxPrice = normalizePrice(text)
		b.setDialog(ctx, msg.Chat.ID, "add.confirm", d)
		b.askConfirm(ctx, msg.Chat.ID, d)
	case "edit.value":
		d.Field = parseFieldArg(d.Field)
		if !validFieldValue(d.Field, text) {
			b.send(ctx, msg.Chat.ID, "Некорректное значение.")
			return
		}
		b.saveEdit(ctx, msg, d, text)
	default:
		b.send(ctx, msg.Chat.ID, "Не понял. Используй команды или /help.")
	}
}

func (b *Bot) handleCallback(ctx context.Context, cb *telego.CallbackQuery) {
	_ = b.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{CallbackQueryID: cb.ID})
	if !b.allowedUser(cb.From.ID) {
		return
	}
	state, raw, err := b.store.GetDialogState(ctx, cb.Message.GetChat().ID)
	if err != nil {
		return
	}
	d := dialogData{}
	_ = json.Unmarshal([]byte(raw), &d)

	parts := strings.Split(cb.Data, ":")
	switch parts[0] {
	case "store":
		if state == "add.store" {
			d.Store = parts[1]
			b.setDialog(ctx, cb.Message.GetChat().ID, "add.query", d)
			b.send(ctx, cb.Message.GetChat().ID, "Магазин: "+storeLabel(parts[1])+"\nЧто ищем?")
		}
	case "city":
		if parts[1] == "moskva" {
			d.City = "Москва"
		} else {
			d.City = ""
		}
		b.setDialog(ctx, cb.Message.GetChat().ID, "add.min_price", d)
		b.askPrice(ctx, cb.Message.GetChat().ID, "Мин. цена (0 — без min):")
	case "confirm":
		if parts[1] == "yes" {
			b.createRule(ctx, cb, d)
		} else {
			b.clearDialog(ctx, cb.Message.GetChat().ID)
			b.send(ctx, cb.Message.GetChat().ID, "Отменено.")
		}
	case "edit":
		b.editField(ctx, cb, parts, state, d)
	case "cancel":
		b.clearDialog(ctx, cb.Message.GetChat().ID)
		b.send(ctx, cb.Message.GetChat().ID, "Отменено.")
	}
}

func (b *Bot) editField(ctx context.Context, cb *telego.CallbackQuery, parts []string, state string, d dialogData) {
	switch parts[1] {
	case "rule":
		ruleID := parseID(parts[2])
		if ruleID == 0 {
			return
		}
		if _, err := b.store.GetRule(ctx, ruleID, cb.From.ID); err != nil {
			b.send(ctx, cb.Message.GetChat().ID, "Правило не найдено.")
			return
		}
		d.RuleID = ruleID
		b.setDialog(ctx, cb.Message.GetChat().ID, "edit.field", d)
		b.askField(ctx, cb.Message.GetChat().ID)
	case "field":
		if state == "edit.field" {
			d.Field = parts[2]
			b.setDialog(ctx, cb.Message.GetChat().ID, "edit.value", d)
			b.send(ctx, cb.Message.GetChat().ID, fieldPrompt(d.Field))
		}
	}
}

func (b *Bot) createRule(ctx context.Context, cb *telego.CallbackQuery, d dialogData) {
	if d.City == "" {
		d.City = "Москва"
	}
	rule := models.Rule{
		UserID:   cb.From.ID,
		ChatID:   cb.Message.GetChat().ID,
		Store:    d.Store,
		Query:    d.Query,
		City:     d.City,
		MinPrice: d.MinPrice,
		MaxPrice: d.MaxPrice,
		Enabled:  true,
	}
	if err := b.store.CreateRule(ctx, rule); err != nil {
		b.log.Error("create rule", "error", err)
		b.send(ctx, cb.Message.GetChat().ID, "Ошибка при сохранении.")
		return
	}
	b.clearDialog(ctx, cb.Message.GetChat().ID)
	b.send(ctx, cb.Message.GetChat().ID, "Правило добавлено: "+html.EscapeString(d.Query))
}

func (b *Bot) saveEdit(ctx context.Context, msg *telego.Message, d dialogData, value string) {
	rule, err := b.store.GetRule(ctx, d.RuleID, msg.From.ID)
	if err != nil {
		b.send(ctx, msg.Chat.ID, "Правило не найдено.")
		b.clearDialog(ctx, msg.Chat.ID)
		return
	}
	switch d.Field {
	case "query":
		rule.Query = value
	case "city":
		rule.City = parseCity(value)
	case "min_price":
		rule.MinPrice = normalizePrice(value)
	case "max_price":
		rule.MaxPrice = normalizePrice(value)
	}
	if err := b.store.UpdateRule(ctx, rule); err != nil {
		b.log.Error("update rule", "error", err)
		b.send(ctx, msg.Chat.ID, "Ошибка при сохранении.")
		return
	}
	b.clearDialog(ctx, msg.Chat.ID)
	b.send(ctx, msg.Chat.ID, "Правило обновлено.")
}

func (b *Bot) askStore(ctx context.Context, chatID int64) {
	rows := [][]telego.InlineKeyboardButton{}
	for _, s := range stores {
		rows = append(rows, te.InlineKeyboardRow(telego.InlineKeyboardButton{Text: storeLabel(s.Key), CallbackData: s.Key}))
	}
	_, _ = b.bot.SendMessage(context.Background(), &telego.SendMessageParams{
		ChatID:      telego.ChatID{ID: chatID},
		Text:        "Какой магазин?",
		ReplyMarkup: te.InlineKeyboard(rows...),
	})
}

func (b *Bot) askCity(ctx context.Context, chatID int64) {
	_, _ = b.bot.SendMessage(context.Background(), &telego.SendMessageParams{
		ChatID:      telego.ChatID{ID: chatID},
		Text:        "Город? (по умолчанию Москва, введи 0 для всех городов)",
		ReplyMarkup: te.InlineKeyboard(te.InlineKeyboardRow(telego.InlineKeyboardButton{Text: "Москва", CallbackData: "city:moskva"}, telego.InlineKeyboardButton{Text: "Все города", CallbackData: "city:all"})),
	})
}

func (b *Bot) askPrice(ctx context.Context, chatID int64, prompt string) {
	b.send(ctx, chatID, prompt)
}

func (b *Bot) askConfirm(ctx context.Context, chatID int64, d dialogData) {
	text := fmt.Sprintf("Правило:\nМагазин: %s\nТовар: %s\nГород: %s\nЦена: %s — %s",
		storeLabel(d.Store), html.EscapeString(d.Query), d.City, orDash(d.MinPrice), orDash(d.MaxPrice))
	_, _ = b.bot.SendMessage(context.Background(), &telego.SendMessageParams{
		ChatID:      telego.ChatID{ID: chatID},
		Text:        text,
		ReplyMarkup: te.InlineKeyboard(te.InlineKeyboardRow(telego.InlineKeyboardButton{Text: "Да", CallbackData: "confirm:yes"}, telego.InlineKeyboardButton{Text: "Отмена", CallbackData: "confirm:no"})),
	})
}

func (b *Bot) askRule(ctx context.Context, chatID int64, rules []models.Rule) {
	rows := [][]telego.InlineKeyboardButton{}
	for _, r := range rules {
		label := fmt.Sprintf("%d. %s", r.ID, r.Query)
		rows = append(rows, te.InlineKeyboardRow(telego.InlineKeyboardButton{Text: label, CallbackData: fmt.Sprintf("edit:rule:%d", r.ID)}))
	}
	_, _ = b.bot.SendMessage(context.Background(), &telego.SendMessageParams{
		ChatID:      telego.ChatID{ID: chatID},
		Text:        "Какое правило изменить?",
		ReplyMarkup: te.InlineKeyboard(rows...),
	})
}

func (b *Bot) askField(ctx context.Context, chatID int64) {
	_, _ = b.bot.SendMessage(context.Background(), &telego.SendMessageParams{
		ChatID: telego.ChatID{ID: chatID},
		Text:   "Что изменить?",
		ReplyMarkup: te.InlineKeyboard(
			te.InlineKeyboardRow(telego.InlineKeyboardButton{Text: "Товар", CallbackData: "edit:field:query"}),
			te.InlineKeyboardRow(telego.InlineKeyboardButton{Text: "Город", CallbackData: "edit:field:city"}),
			te.InlineKeyboardRow(telego.InlineKeyboardButton{Text: "Мин. цена", CallbackData: "edit:field:min_price"}),
			te.InlineKeyboardRow(telego.InlineKeyboardButton{Text: "Макс. цена", CallbackData: "edit:field:max_price"}),
		),
	})
}

func (b *Bot) startDialog(ctx context.Context, chatID int64, state string, d dialogData) {
	raw, _ := json.Marshal(d)
	_ = b.store.UpsertDialogState(ctx, chatID, state, string(raw))
}

func (b *Bot) setDialog(ctx context.Context, chatID int64, state string, d dialogData) {
	raw, _ := json.Marshal(d)
	_ = b.store.UpsertDialogState(ctx, chatID, state, string(raw))
}

func (b *Bot) clearDialog(ctx context.Context, chatID int64) {
	_ = b.store.DeleteDialogState(ctx, chatID)
}

func storeLabel(key string) string {
	for _, s := range stores {
		if s.Key == key {
			return s.Label
		}
	}
	return key
}

func parseCity(text string) string {
	if strings.TrimSpace(text) == "0" {
		return ""
	}
	return strings.TrimSpace(text)
}

func validPrice(text string) bool {
	s := strings.ReplaceAll(strings.TrimSpace(text), ",", ".")
	if s == "" {
		return false
	}
	return isNumeric(s)
}

func normalizePrice(text string) string {
	s := strings.TrimSpace(text)
	if s == "" || s == "0" {
		return ""
	}
	return strings.ReplaceAll(strings.ReplaceAll(s, " ", ""), ",", ".")
}

func validFieldValue(field, value string) bool {
	switch field {
	case "min_price", "max_price":
		return validPrice(value)
	default:
		return strings.TrimSpace(value) != ""
	}
}

func parseFieldArg(field string) string {
	if field == "" {
		return "query"
	}
	return field
}

func fieldPrompt(field string) string {
	switch field {
	case "query":
		return "Новый товар:"
	case "city":
		return "Новый город (0 — все города):"
	case "min_price":
		return "Новая мин. цена (0 — без min):"
	default:
		return "Новая макс. цена (0 — без max):"
	}
}

func orDash(v string) string {
	if v == "" {
		return "—"
	}
	return v
}

func parseID(s string) int64 {
	id, _ := strconv.ParseInt(s, 10, 64)
	return id
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}
