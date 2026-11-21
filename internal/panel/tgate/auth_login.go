package tgate

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	"mikan/internal/panel/secure"
	"mikan/internal/panel/settings"
)

var (
	ErrAuthNotFound = errors.New("auth_not_found")
	ErrAuthExpired  = errors.New("auth_expired")
	ErrBotNotReady  = errors.New("bot_not_ready")
)

type TelegramAuthRequest struct {
	Token        string    `json:"token"`
	IP           string    `json:"ip"`
	UserAgent    string    `json:"user_agent"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	Status       string    `json:"status"` // "pending", "approved", "rejected", "expired"
	TelegramUser *User     `json:"telegram_user,omitempty"`
}

type TelegramBindRequest struct {
	Token     string    `json:"token"`
	AdminID   int64     `json:"admin_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Status    string    `json:"status"` // "pending", "bound", "expired"
	BoundUser *User     `json:"bound_user,omitempty"`
}

func (g *Gateway) CreateAuthRequest(ctx context.Context, ip, userAgent string) (*TelegramAuthRequest, string, error) {
	st := g.Status()
	botUser := st.Bot.Username
	if botUser == "" {
		if cachedBot, ok, _ := settings.Get[User](ctx, g.d.Settings, KeyBot); ok && cachedBot.Username != "" {
			botUser = cachedBot.Username
		}
	}
	if botUser == "" {
		return nil, "", ErrBotNotReady
	}

	g.authMu.Lock()
	defer g.authMu.Unlock()

	now := g.d.Now()
	for k, req := range g.authRequests {
		if now.After(req.ExpiresAt) {
			delete(g.authRequests, k)
		}
	}

	token := "auth_" + secure.Token(24)
	req := &TelegramAuthRequest{
		Token:     token,
		IP:        ip,
		UserAgent: userAgent,
		CreatedAt: now,
		ExpiresAt: now.Add(5 * time.Minute),
		Status:    "pending",
	}
	g.authRequests[token] = req
	link := fmt.Sprintf("https://t.me/%s?start=%s", botUser, token)
	return req, link, nil
}

func (g *Gateway) CheckAuthRequest(token string) (*TelegramAuthRequest, error) {
	g.authMu.Lock()
	defer g.authMu.Unlock()

	req, ok := g.authRequests[token]
	if !ok {
		return nil, ErrAuthNotFound
	}
	now := g.d.Now()
	if now.After(req.ExpiresAt) {
		req.Status = "expired"
	}
	clone := *req
	return &clone, nil
}

func (g *Gateway) CreateBindRequest(ctx context.Context, adminID int64) (*TelegramBindRequest, string, error) {
	st := g.Status()
	botUser := st.Bot.Username
	if botUser == "" {
		if cachedBot, ok, _ := settings.Get[User](ctx, g.d.Settings, KeyBot); ok && cachedBot.Username != "" {
			botUser = cachedBot.Username
		}
	}
	if botUser == "" {
		return nil, "", ErrBotNotReady
	}

	g.authMu.Lock()
	defer g.authMu.Unlock()

	now := g.d.Now()
	for k, req := range g.bindRequests {
		if now.After(req.ExpiresAt) {
			delete(g.bindRequests, k)
		}
	}

	token := "bind_" + secure.Token(24)
	req := &TelegramBindRequest{
		Token:     token,
		AdminID:   adminID,
		CreatedAt: now,
		ExpiresAt: now.Add(5 * time.Minute),
		Status:    "pending",
	}
	g.bindRequests[token] = req
	link := fmt.Sprintf("https://t.me/%s?start=%s", botUser, token)
	return req, link, nil
}

func (g *Gateway) CheckBindRequest(token string) (*TelegramBindRequest, error) {
	g.authMu.Lock()
	defer g.authMu.Unlock()

	req, ok := g.bindRequests[token]
	if !ok {
		return nil, ErrAuthNotFound
	}
	now := g.d.Now()
	if now.After(req.ExpiresAt) {
		req.Status = "expired"
	}
	clone := *req
	return &clone, nil
}

func (g *Gateway) IsAuthAdmin(ctx context.Context, tgID int64, username string) bool {
	if tgID == 0 {
		return false
	}
	// 1. Check KeyInfraAdminChat
	if adminChat, ok, _ := settings.Get[int64](ctx, g.d.Settings, KeyInfraAdminChat); ok && adminChat != 0 && adminChat == tgID {
		return true
	}
	// 2. Check telegram_admin_ids
	if adminList, ok, _ := settings.Get[string](ctx, g.d.Settings, "telegram_admin_ids"); ok && adminList != "" {
		for _, part := range strings.Split(adminList, ",") {
			if id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64); err == nil && id == tgID {
				return true
			}
		}
	}
	// 3. Check auth_telegram_ids
	if authList, ok, _ := settings.Get[string](ctx, g.d.Settings, settings.KeyAuthTelegramIDs); ok && authList != "" {
		for _, part := range strings.Split(authList, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if id, err := strconv.ParseInt(part, 10, 64); err == nil && id == tgID {
				return true
			}
			if username != "" && strings.EqualFold(strings.TrimPrefix(part, "@"), strings.TrimPrefix(username, "@")) {
				return true
			}
		}
	}
	return false
}

func (g *Gateway) authLogin(ctx context.Context, out *Outbox, m *Message) bool {
	chat := m.Chat.ID
	text := strings.TrimSpace(m.Text)
	if !strings.HasPrefix(text, "/start auth_") {
		return false
	}
	token := strings.TrimSpace(strings.TrimPrefix(text, "/start "))

	// Check admin rights
	if !g.IsAuthAdmin(ctx, m.From.ID, m.From.Username) {
		out.Reply(chat, "", 1, func(ctx context.Context, c *Client) error {
			_, err := c.Send(ctx, chat, "⛔ Доступ запрещён. Ваш Telegram-аккаунт не привязан к администратору панели управления.", nil, false)
			return err
		})
		return true
	}

	g.authMu.Lock()
	req, ok := g.authRequests[token]
	now := g.d.Now()
	if ok && now.After(req.ExpiresAt) {
		req.Status = "expired"
		ok = false
	}
	g.authMu.Unlock()

	if !ok || req.Status != "pending" {
		out.Reply(chat, "", 1, func(ctx context.Context, c *Client) error {
			_, err := c.Send(ctx, chat, "⚠️ Ссылка для входа устарела или недействительна. Запросите новую на странице входа в панель.", nil, false)
			return err
		})
		return true
	}

	ua := req.UserAgent
	if len(ua) > 60 {
		ua = ua[:57] + "..."
	}
	if ua == "" {
		ua = "Неизвестно"
	}
	timeStr := req.CreatedAt.Format("15:04:05")

	msgText := fmt.Sprintf(
		"🔐 <b>Вход в панель управления Hikari</b>\n\n"+
			"IP-адрес: <code>%s</code>\n"+
			"Устройство: <code>%s</code>\n"+
			"Время: <code>%s</code>\n\n"+
			"Подтвердить вход в панель управления?",
		req.IP, ua, timeStr,
	)

	kb := &Keyboard{
		InlineKeyboard: [][]Button{
			{
				{Text: "✅ Войти", CallbackData: "tgauth:ok:" + token},
				{Text: "❌ Отклонить", CallbackData: "tgauth:no:" + token},
			},
		},
	}

	out.Reply(chat, "auth:"+token, 1, func(ctx context.Context, c *Client) error {
		_, err := c.Send(ctx, chat, msgText, kb, false)
		return err
	})
	return true
}

func (g *Gateway) authBind(ctx context.Context, out *Outbox, m *Message) bool {
	chat := m.Chat.ID
	text := strings.TrimSpace(m.Text)
	if !strings.HasPrefix(text, "/start bind_") {
		return false
	}
	token := strings.TrimSpace(strings.TrimPrefix(text, "/start "))

	g.authMu.Lock()
	req, ok := g.bindRequests[token]
	now := g.d.Now()
	if ok && now.After(req.ExpiresAt) {
		req.Status = "expired"
		ok = false
	}
	if ok && req.Status == "pending" {
		req.Status = "bound"
		u := *m.From
		req.BoundUser = &u
	}
	g.authMu.Unlock()

	if !ok || req.Status != "bound" {
		out.Reply(chat, "", 1, func(ctx context.Context, c *Client) error {
			_, err := c.Send(ctx, chat, "⚠️ Ссылка для привязки аккаунта устарела или недействительна. Запросите новую в настройках панели.", nil, false)
			return err
		})
		return true
	}

	current, _ := g.d.Settings.String(ctx, settings.KeyAuthTelegramIDs)
	parts := strings.Split(current, ",")
	seen := false
	idStr := strconv.FormatInt(m.From.ID, 10)
	for _, p := range parts {
		if strings.TrimSpace(p) == idStr {
			seen = true
			break
		}
	}
	if !seen {
		var newParts []string
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				newParts = append(newParts, trimmed)
			}
		}
		newParts = append(newParts, idStr)
		if m.From.Username != "" {
			newParts = append(newParts, "@"+m.From.Username)
		}
		_ = settings.Set(ctx, g.d.Settings, settings.KeyAuthTelegramIDs, strings.Join(newParts, ", "))
	}

	userDisplay := idStr
	if m.From.Username != "" {
		userDisplay = fmt.Sprintf("@%s (ID: %s)", m.From.Username, idStr)
	}

	out.Reply(chat, "bind:"+token, 1, func(ctx context.Context, c *Client) error {
		msg := fmt.Sprintf(
			"✅ <b>Telegram успешно привязан!</b>\n\n"+
				"Аккаунт: <code>%s</code>\n"+
				"Теперь вы можете входить в панель управления Hikari через этого бота.",
			userDisplay,
		)
		_, err := c.Send(ctx, chat, msg, nil, false)
		return err
	})
	return true
}

func (g *Gateway) authCallback(ctx context.Context, out *Outbox, q *CallbackQuery) bool {
	data := q.Data
	if strings.HasPrefix(data, "tgkill:") {
		sessID := strings.TrimPrefix(data, "tgkill:")
		chat := q.Message.Chat.ID
		msgID := q.Message.MessageID

		if !g.IsAuthAdmin(ctx, q.From.ID, q.From.Username) {
			out.Reply(chat, "", 1, func(ctx context.Context, c *Client) error {
				return c.Answer(ctx, q.ID, "Доступ запрещён")
			})
			return true
		}

		_ = g.d.Store.Q.DeleteSession(ctx, sessID)

		out.Reply(chat, "kill:"+sessID, 1, func(ctx context.Context, c *Client) error {
			_ = c.Answer(ctx, q.ID, "Сессия завершена")
			return c.Edit(ctx, chat, msgID, "🚨 <b>Сессия была принудительно завершена.</b>\nДоступ к панели с этого устройства отозван.", nil)
		})
		return true
	}

	if !strings.HasPrefix(data, "tgauth:") {
		return false
	}
	parts := strings.SplitN(data, ":", 3)
	if len(parts) != 3 {
		return true
	}
	action := parts[1]
	token := parts[2]
	chat := q.Message.Chat.ID
	msgID := q.Message.MessageID

	if !g.IsAuthAdmin(ctx, q.From.ID, q.From.Username) {
		out.Reply(chat, "", 1, func(ctx context.Context, c *Client) error {
			_ = c.Answer(ctx, q.ID, "Доступ запрещён")
			return nil
		})
		return true
	}

	g.authMu.Lock()
	req, ok := g.authRequests[token]
	now := g.d.Now()
	if ok && now.After(req.ExpiresAt) {
		req.Status = "expired"
		ok = false
	}
	if ok && req.Status == "pending" {
		if action == "ok" {
			req.Status = "approved"
			u := q.From
			req.TelegramUser = &u
		} else {
			req.Status = "rejected"
		}
	}
	g.authMu.Unlock()

	out.Reply(chat, "auth:ans:"+token, 1, func(ctx context.Context, c *Client) error {
		if !ok {
			_ = c.Answer(ctx, q.ID, "Ссылка устарела")
			return c.Edit(ctx, chat, msgID, "⚠️ Ссылка для входа устарела.", nil)
		}
		if action == "ok" {
			_ = c.Answer(ctx, q.ID, "Вход подтверждён!")
			return c.Edit(ctx, chat, msgID, fmt.Sprintf("✅ <b>Вход в панель управления подтверждён.</b>\nIP: <code>%s</code>", req.IP), nil)
		}
		_ = c.Answer(ctx, q.ID, "Вход отклонён")
		return c.Edit(ctx, chat, msgID, "❌ <b>Вход в панель управления отклонён.</b>", nil)
	})
	return true
}

func (g *Gateway) NotifyLogin(ctx context.Context, sessionIDHash, method, identity, ip, userAgent string) {
	out := g.Outbox()
	if out == nil {
		return
	}

	chats := make(map[int64]struct{})
	if adminChat, ok, _ := settings.Get[int64](ctx, g.d.Settings, KeyInfraAdminChat); ok && adminChat != 0 {
		chats[adminChat] = struct{}{}
	}
	for _, key := range []string{"telegram_admin_ids", settings.KeyAuthTelegramIDs} {
		if val, ok, _ := settings.Get[string](ctx, g.d.Settings, key); ok && val != "" {
			for _, part := range strings.Split(val, ",") {
				if id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64); err == nil && id != 0 {
					chats[id] = struct{}{}
				}
			}
		}
	}
	if len(chats) == 0 {
		return
	}

	methodTitle := "Пароль"
	if method == "telegram" {
		methodTitle = "Telegram"
		if identity != "" {
			methodTitle += " (" + identity + ")"
		}
	} else if method == "github" {
		methodTitle = "GitHub"
		if identity != "" {
			methodTitle += " (" + identity + ")"
		}
	}

	ua := userAgent
	if len(ua) > 60 {
		ua = ua[:57] + "..."
	}
	if ua == "" {
		ua = "Неизвестно"
	}

	nowStr := g.d.Now().Format("15:04:05 02.01.2006")
	msgText := fmt.Sprintf(
		"🔔 <b>Вход в панель управления Hikari</b>\n\n"+
			"👤 Способ: <b>%s</b>\n"+
			"🌐 IP-адрес: <code>%s</code>\n"+
			"💻 Устройство: <code>%s</code>\n"+
			"🕒 Время: <code>%s</code>\n\n"+
			"Если это были не вы, завершите сессию немедленно:",
		html.EscapeString(methodTitle),
		html.EscapeString(ip),
		html.EscapeString(ua),
		html.EscapeString(nowStr),
	)

	kb := &Keyboard{
		InlineKeyboard: [][]Button{
			{
				{Text: "🚨 Завершить эту сессию", CallbackData: "tgkill:" + sessionIDHash},
			},
		},
	}

	for chatID := range chats {
		cid := chatID
		out.Notice(cid, func(ctx context.Context, c *Client) error {
			_, err := c.Send(ctx, cid, msgText, kb, false)
			return err
		}, nil)
	}
}
