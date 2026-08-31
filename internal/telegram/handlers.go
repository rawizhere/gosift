package telegram

import (
	"context"
	"fmt"
	"html"
	"strconv"
	"strings"

	"github.com/mymmrac/telego"
	te "github.com/mymmrac/telego/telegoutil"

	"github.com/rawizhere/gosift/internal/models"
)

func (b *Bot) handleMessage(ctx context.Context, msg *telego.Message) {
	if !b.allowedUser(msg.From.ID) {
		b.send(ctx, msg.Chat.ID, "Доступ запрещён.")
		return
	}
	text := strings.TrimSpace(msg.Text)
	if strings.HasPrefix(text, "/") {
		_ = b.repo.DeleteDialogState(ctx, msg.Chat.ID)
		b.handleCommand(ctx, msg, text)
		return
	}
	b.handleDialogStep(ctx, msg, text)
}

func (b *Bot) handleCommand(ctx context.Context, msg *telego.Message, text string) {
	cmd, arg, _ := strings.Cut(text, " ")
	cmd = strings.ToLower(cmd)

	switch cmd {
	case "/start":
		b.cmdStart(ctx, msg)
	case "/help":
		b.cmdHelp(ctx, msg)
	case "/list":
		b.cmdList(ctx, msg)
	case "/add":
		b.cmdAdd(ctx, msg)
	case "/edit":
		b.cmdEdit(ctx, msg, arg)
	case "/remove":
		b.cmdRemove(ctx, msg, arg)
	case "/on", "/off":
		b.cmdToggle(ctx, msg, cmd, arg)
	case "/settings":
		b.cmdSettings(ctx, msg)
	default:
		b.cmdHelp(ctx, msg)
	}
}

func (b *Bot) cmdStart(ctx context.Context, msg *telego.Message) {
	err := b.repo.CreateUser(ctx, userFromMessage(msg))
	if err != nil {
		b.log.Error("create user", "error", err)
	}
	b.send(ctx, msg.Chat.ID, "Привет! Я парсю магазины и присылаю карточки по твоим фильтрам.")
	b.showMenu(ctx, msg.Chat.ID)
}

func (b *Bot) cmdHelp(ctx context.Context, msg *telego.Message) {
	b.showMenu(ctx, msg.Chat.ID)
}

func (b *Bot) cmdSettings(ctx context.Context, msg *telego.Message) {
	text := fmt.Sprintf("Интервал парсинга: %s\nПауза между запросами: %s\nЛимит на правило: %d\nМагазины: %s",
		b.cfg.ParseInterval, b.cfg.ParseJitter, b.cfg.ParseLimit, strings.Join(b.stores, ", "))
	if len(b.stores) == 0 {
		text = "Магазины не подключены."
	}
	b.send(ctx, msg.Chat.ID, text)
}

func (b *Bot) cmdList(ctx context.Context, msg *telego.Message) {
	rules, err := b.repo.ListRulesByUser(ctx, msg.From.ID)
	if err != nil {
		b.log.Error("list rules", "error", err)
		b.send(ctx, msg.Chat.ID, "Ошибка при чтении правил.")
		return
	}
	if len(rules) == 0 {
		b.send(ctx, msg.Chat.ID, "Правил нет. Добавь через /add.")
		return
	}
	var sb strings.Builder
	for _, r := range rules {
		status := "вкл"
		if !r.Enabled {
			status = "выкл"
		}
		scope := ""
		if r.Category != "" {
			scope = " [" + categoryTitle(r.Category) + "]"
		}
		fmt.Fprintf(&sb, "%d. %s%s [%s] (%s)\n", r.ID, html.EscapeString(r.Query), scope, status, html.EscapeString(r.City))
	}
	b.send(ctx, msg.Chat.ID, sb.String())
}

func (b *Bot) cmdRemove(ctx context.Context, msg *telego.Message, arg string) {
	id, err := strconv.ParseInt(strings.TrimSpace(arg), 10, 64)
	if err != nil {
		b.send(ctx, msg.Chat.ID, "Использование: /remove <id>")
		return
	}
	if err := b.repo.DeleteRule(ctx, id, msg.From.ID); err != nil {
		b.send(ctx, msg.Chat.ID, "Правило не найдено.")
		return
	}
	b.send(ctx, msg.Chat.ID, "Правило удалено.")
}

func (b *Bot) cmdToggle(ctx context.Context, msg *telego.Message, cmd, arg string) {
	id, err := strconv.ParseInt(strings.TrimSpace(arg), 10, 64)
	if err != nil {
		b.send(ctx, msg.Chat.ID, "Использование: "+cmd+" <id>")
		return
	}
	enabled := cmd == "/on"
	if err := b.repo.SetRuleEnabled(ctx, id, msg.From.ID, enabled); err != nil {
		b.send(ctx, msg.Chat.ID, "Правило не найдено.")
		return
	}
	if enabled {
		b.send(ctx, msg.Chat.ID, "Правило включено.")
	} else {
		b.send(ctx, msg.Chat.ID, "Правило выключено.")
	}
}

func (b *Bot) send(ctx context.Context, chatID int64, text string) {
	_, _ = b.bot.SendMessage(ctx, &telego.SendMessageParams{
		ChatID: telego.ChatID{ID: chatID},
		Text:   text,
	})
}

func userFromMessage(msg *telego.Message) models.User {
	return models.User{
		UserID:    msg.From.ID,
		Username:  msg.From.Username,
		FirstName: msg.From.FirstName,
		ChatID:    msg.Chat.ID,
	}
}

// showMenu renders the main menu.
func (b *Bot) showMenu(ctx context.Context, chatID int64) {
	_, _ = b.bot.SendMessage(ctx, &telego.SendMessageParams{
		ChatID: telego.ChatID{ID: chatID},
		Text:   "Меню:",
		ReplyMarkup: te.InlineKeyboard(
			te.InlineKeyboardRow(telego.InlineKeyboardButton{Text: "➕ Добавить правило", CallbackData: "menu:add"}),
			te.InlineKeyboardRow(telego.InlineKeyboardButton{Text: "📋 Мои правила", CallbackData: "menu:list"}),
			te.InlineKeyboardRow(telego.InlineKeyboardButton{Text: "✏️ Изменить", CallbackData: "menu:edit"}),
		),
	})
}
