package tgbot

import (
	"context"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	"mikan/internal/panel/domain"
	"mikan/internal/panel/tgate"
)

type adminWords struct {
	title           string
	clientsSection  string
	totalUsers      string
	activeUsers     string
	expiringUsers   string
	limitedUsers    string
	expiredUsers    string
	disabledUsers   string
	online24h       string
	nodesSection    string
	totalNodes      string
	clientsBtn      string
	newClientBtn    string
	searchBtn       string
	nodesBtn        string
	refreshBtn      string
	userMenuBtn     string
	usersTitle      string
	usersEmpty      string
	usersSearchTitle string
	prevBtn         string
	nextBtn         string
	backBtn         string
	cancelBtn       string
	searchPrompt    string
	newClientPrompt string
	createAutoBtn   string
	clientCardTitle string
	statusLabel     string
	tariffLabel     string
	termLabel       string
	trafficLabel    string
	devicesLabel    string
	onlineLabel     string
	createdLabel    string
	subLinkLabel    string
	neverOnline     string
	forever         string
	noTariff        string
	extend30Btn     string
	extend7Btn      string
	resetTrafficBtn string
	disableBtn      string
	enableBtn       string
	devicesBtn      string
	sendLinkBtn     string
	backToListBtn   string
	userNotFound    string
	devicesTitle    string
	devicesEmpty    string
	unbindDeviceBtn string
	backToClientBtn string
	nodesTitle      string
	nodeDetailTitle string
	statusOnline    string
	statusOffline   string
	statusDisabled  string
	connsLabel      string
	cpuLabel        string
	ramLabel        string
	versionLabel    string
	errorLabel      string
	netSpeedLabel   string
	extendedNotice  string
	trafficResetNotice string
	statusNotice    string
	deviceUnboundNotice string
	linkSentNotice  string
	userCreatedNotice string
	errorNotice     string
}

var adminRu = adminWords{
	title:           "📊 <b>Панель управления Hikari</b>",
	clientsSection:  "👥 <b>Клиенты:</b>",
	totalUsers:      "• Всего: <b>%d</b>",
	activeUsers:     "• 🟢 Активные: %d",
	expiringUsers:   "• 🟡 Заканчивается срок: %d",
	limitedUsers:    "• 🔴 Исчерпан трафик: %d",
	expiredUsers:    "• 🔴 Истёк срок: %d",
	disabledUsers:   "• ⚪️ Отключены: %d",
	online24h:       "• ⚡️ Онлайн за 24 часа: <b>%d</b>",
	nodesSection:    "🖥 <b>Серверы и ноды:</b>",
	totalNodes:      "• Всего нод: <b>%d</b> (🟢 %d онлайн, 🔴 %d оффлайн)",
	clientsBtn:      "👥 Клиенты",
	newClientBtn:    "➕ Новый клиент",
	searchBtn:       "🔍 Поиск",
	nodesBtn:        "🖥 Ноды",
	refreshBtn:      "🔄 Обновить",
	userMenuBtn:     "👤 Пользовательский вид",
	usersTitle:      "👥 <b>Клиенты (Всего: %d, стр. %d/%d)</b>",
	usersEmpty:      "Клиенты не найдены.",
	usersSearchTitle: "🔍 <b>Поиск: «%s» (Найдено: %d)</b>",
	prevBtn:         "⬅️ Назад",
	nextBtn:         "Вперёд ➡️",
	backBtn:         "« Главное меню",
	cancelBtn:       "« Отмена",
	searchPrompt:    "🔍 <b>Поиск клиентов</b>\n\nОтправьте в чат имя, ID или фрагмент ссылки/токена:",
	newClientPrompt: "➕ <b>Новый клиент</b>\n\nОтправьте имя для нового клиента в чат или создайте автоматически:",
	createAutoBtn:   "⚡️ Создать автоматически",
	clientCardTitle: "👤 <b>Клиент: %s</b> (ID: <code>%d</code>)",
	statusLabel:     "📊 Статус:",
	tariffLabel:     "📋 Тариф:",
	termLabel:       "📅 Срок:",
	trafficLabel:    "📦 Трафик:",
	devicesLabel:    "📱 Устройства:",
	onlineLabel:     "🕒 Последний онлайн:",
	createdLabel:    "🗓 Создан:",
	subLinkLabel:    "🔗 Ссылка на подписку:",
	neverOnline:     "Никогда",
	forever:         "Бессрочно",
	noTariff:        "Без тарифа",
	extend30Btn:     "⏳ +30 дней",
	extend7Btn:      "⏳ +7 дней",
	resetTrafficBtn: "🔄 Сброс трафика",
	disableBtn:      "🔌 Отключить",
	enableBtn:       "⚡️ Включить",
	devicesBtn:      "📱 Устройства (%d)",
	sendLinkBtn:     "🔗 Ссылка в чат",
	backToListBtn:   "« К списку клиентов",
	userNotFound:    "Пользователь не найден.",
	devicesTitle:    "📱 <b>Устройства клиента %s</b> (Всего: %d):",
	devicesEmpty:    "Привязанных устройств нет.",
	unbindDeviceBtn: "❌ Отвязать",
	backToClientBtn: "« Назад к клиенту",
	nodesTitle:      "🖥 <b>Серверы и ноды (%d):</b>",
	nodeDetailTitle: "🖥 <b>Нода: %s</b>",
	statusOnline:    "🟢 Онлайн",
	statusOffline:   "🔴 Оффлайн",
	statusDisabled:  "⚪️ Отключена",
	connsLabel:      "Соединений:",
	cpuLabel:        "CPU:",
	ramLabel:        "RAM:",
	versionLabel:    "Версия:",
	errorLabel:      "Ошибка:",
	netSpeedLabel:   "Сеть Rx/Tx:",
	extendedNotice:  "✅ Срок продлён на %d дн.",
	trafficResetNotice: "✅ Трафик успешно сброшен",
	statusNotice:    "✅ Статус изменён: %s",
	deviceUnboundNotice: "✅ Устройство отвязано",
	linkSentNotice:  "✅ Ссылка отправлена отдельным сообщением",
	userCreatedNotice: "✅ Клиент успешно создан",
	errorNotice:     "❌ Ошибка: %s",
}

var adminEn = adminWords{
	title:           "📊 <b>Hikari Admin Panel</b>",
	clientsSection:  "👥 <b>Clients:</b>",
	totalUsers:      "• Total: <b>%d</b>",
	activeUsers:     "• 🟢 Active: %d",
	expiringUsers:   "• 🟡 Expiring: %d",
	limitedUsers:    "• 🔴 Limited: %d",
	expiredUsers:    "• 🔴 Expired: %d",
	disabledUsers:   "• ⚪️ Disabled: %d",
	online24h:       "• ⚡️ Online in 24h: <b>%d</b>",
	nodesSection:    "🖥 <b>Servers & Nodes:</b>",
	totalNodes:      "• Total nodes: <b>%d</b> (🟢 %d online, 🔴 %d offline)",
	clientsBtn:      "👥 Clients",
	newClientBtn:    "➕ New client",
	searchBtn:       "🔍 Search",
	nodesBtn:        "🖥 Nodes",
	refreshBtn:      "🔄 Refresh",
	userMenuBtn:     "👤 User View",
	usersTitle:      "👥 <b>Clients (Total: %d, pg. %d/%d)</b>",
	usersEmpty:      "No clients found.",
	usersSearchTitle: "🔍 <b>Search: \"%s\" (Found: %d)</b>",
	prevBtn:         "⬅️ Prev",
	nextBtn:         "Next ➡️",
	backBtn:         "« Main Menu",
	cancelBtn:       "« Cancel",
	searchPrompt:    "🔍 <b>Search clients</b>\n\nSend client name, ID, or token/link fragment:",
	newClientPrompt: "➕ <b>New client</b>\n\nSend client name in chat or create automatically:",
	createAutoBtn:   "⚡️ Create automatically",
	clientCardTitle: "👤 <b>Client: %s</b> (ID: <code>%d</code>)",
	statusLabel:     "📊 Status:",
	tariffLabel:     "📋 Tariff:",
	termLabel:       "📅 Term:",
	trafficLabel:    "📦 Traffic:",
	devicesLabel:    "📱 Devices:",
	onlineLabel:     "🕒 Last online:",
	createdLabel:    "🗓 Created:",
	subLinkLabel:    "🔗 Subscription link:",
	neverOnline:     "Never",
	forever:         "Unlimited",
	noTariff:        "No tariff",
	extend30Btn:     "⏳ +30 days",
	extend7Btn:      "⏳ +7 days",
	resetTrafficBtn: "🔄 Reset traffic",
	disableBtn:      "🔌 Disable",
	enableBtn:       "⚡️ Enable",
	devicesBtn:      "📱 Devices (%d)",
	sendLinkBtn:     "🔗 Send link",
	backToListBtn:   "« Back to clients",
	userNotFound:    "User not found.",
	devicesTitle:    "📱 <b>Devices of %s</b> (Total: %d):",
	devicesEmpty:    "No bound devices.",
	unbindDeviceBtn: "❌ Unbind",
	backToClientBtn: "« Back to client",
	nodesTitle:      "🖥 <b>Servers and nodes (%d):</b>",
	nodeDetailTitle: "🖥 <b>Node: %s</b>",
	statusOnline:    "🟢 Online",
	statusOffline:   "🔴 Offline",
	statusDisabled:  "⚪️ Disabled",
	connsLabel:      "Connections:",
	cpuLabel:        "CPU:",
	ramLabel:        "RAM:",
	versionLabel:    "Version:",
	errorLabel:      "Error:",
	netSpeedLabel:   "Net Rx/Tx:",
	extendedNotice:  "✅ Term extended by %d days",
	trafficResetNotice: "✅ Traffic reset successfully",
	statusNotice:    "✅ Status updated: %s",
	deviceUnboundNotice: "✅ Device unbound",
	linkSentNotice:  "✅ Link sent as separate message",
	userCreatedNotice: "✅ Client created successfully",
	errorNotice:     "❌ Error: %s",
}

func adminWordsFor(lang string) *adminWords {
	if lang == "en" {
		return &adminEn
	}
	return &adminRu
}

func statusEmoji(status string) string {
	switch status {
	case domain.StateActive:
		return "🟢"
	case domain.StateExpiring:
		return "🟡"
	case domain.StateLimited, domain.StateExpired:
		return "🔴"
	case domain.StateDisabled:
		return "⚪️"
	default:
		return "⚪️"
	}
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatSpeed(bps uint64) string {
	bytesPerSec := bps / 8
	if bytesPerSec < 1024 {
		return fmt.Sprintf("%d B/s", bytesPerSec)
	} else if bytesPerSec < 1024*1024 {
		return fmt.Sprintf("%.1f KB/s", float64(bytesPerSec)/1024)
	}
	return fmt.Sprintf("%.1f MB/s", float64(bytesPerSec)/(1024*1024))
}

func (b *Bot) getAdminState(chat int64) string {
	if val, ok := b.adminStates.Load(chat); ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

func (b *Bot) setAdminState(chat int64, state string) {
	if state == "" {
		b.adminStates.Delete(chat)
	} else {
		b.adminStates.Store(chat, state)
	}
}

func (b *Bot) freshAdminMenu(chat int64, screenData, notice string) {
	b.d.Outbox.Reply(chat, "menu:"+strconv.FormatInt(chat, 10), 2, func(ctx context.Context, c *tgate.Client) error {
		text, kb := b.adminScreen(ctx, b.config(ctx), chat, screenData, notice)
		sent, err := c.Send(ctx, chat, text, kb, false)
		if err != nil {
			return err
		}
		if old, err := b.d.Backend.Chat(ctx, chat); err == nil && old.MenuMsgID != 0 && old.MenuMsgID != sent.MessageID {
			_ = c.Delete(ctx, chat, old.MenuMsgID)
		}
		_ = b.d.Backend.SetMenu(ctx, chat, sent.MessageID)
		return nil
	})
}

// adminScreen renders the admin screens.
func (b *Bot) adminScreen(ctx context.Context, cfg Config, chat int64, data, notice string) (string, *tgate.Keyboard) {
	aw := adminWordsFor(cfg.Lang)
	w := wordsFor(cfg.Lang)
	withNotice := func(s string) string {
		if notice != "" {
			return html.EscapeString(notice) + "\n\n" + s
		}
		return s
	}

	cmd, arg, _ := strings.Cut(data, ":")
	switch cmd {
	case "a":
		subcmd, subarg, _ := strings.Cut(arg, ":")
		switch subcmd {
		case "u":
			return b.adminUsersScreen(ctx, aw, w, subarg, withNotice)
		case "ud":
			id, _ := strconv.ParseInt(subarg, 10, 64)
			return b.adminUserDetailScreen(ctx, aw, id, withNotice)
		case "dev":
			id, _ := strconv.ParseInt(subarg, 10, 64)
			return b.adminDevicesScreen(ctx, aw, id, withNotice)
		case "n":
			return b.adminNodesScreen(ctx, aw, withNotice)
		case "nd":
			id, _ := strconv.ParseInt(subarg, 10, 64)
			return b.adminNodeDetailScreen(ctx, aw, id, withNotice)
		}
	}

	// Default: a:m (Admin Overview / Dashboard)
	ov, err := b.d.Backend.AdminOverview(ctx)
	if err != nil {
		text := withNotice(aw.title + "\n\n" + fmt.Sprintf(aw.errorNotice, err.Error()))
		return text, &tgate.Keyboard{InlineKeyboard: [][]tgate.Button{
			{{Text: aw.refreshBtn, CallbackData: "a:m"}},
			{{Text: aw.userMenuBtn, CallbackData: "a:close"}},
		}}
	}

	var sb strings.Builder
	sb.WriteString(aw.title + "\n\n")
	sb.WriteString(aw.clientsSection + "\n")
	sb.WriteString(fmt.Sprintf(aw.totalUsers, ov.TotalUsers) + "\n")
	sb.WriteString(fmt.Sprintf(aw.activeUsers, ov.ActiveUsers) + "\n")
	if ov.ExpiringUsers > 0 {
		sb.WriteString(fmt.Sprintf(aw.expiringUsers, ov.ExpiringUsers) + "\n")
	}
	if ov.LimitedUsers > 0 {
		sb.WriteString(fmt.Sprintf(aw.limitedUsers, ov.LimitedUsers) + "\n")
	}
	if ov.ExpiredUsers > 0 {
		sb.WriteString(fmt.Sprintf(aw.expiredUsers, ov.ExpiredUsers) + "\n")
	}
	if ov.DisabledUsers > 0 {
		sb.WriteString(fmt.Sprintf(aw.disabledUsers, ov.DisabledUsers) + "\n")
	}
	sb.WriteString(fmt.Sprintf(aw.online24h, ov.Online24h) + "\n\n")

	sb.WriteString(aw.nodesSection + "\n")
	sb.WriteString(fmt.Sprintf(aw.totalNodes, ov.TotalNodes, ov.OnlineNodes, ov.OfflineNodes))

	kb := &tgate.Keyboard{
		InlineKeyboard: [][]tgate.Button{
			{
				{Text: aw.clientsBtn, CallbackData: "a:u:0"},
				{Text: aw.newClientBtn, CallbackData: "a:u:new"},
			},
			{
				{Text: aw.searchBtn, CallbackData: "a:u:search"},
				{Text: aw.nodesBtn, CallbackData: "a:n"},
			},
			{
				{Text: aw.refreshBtn, CallbackData: "a:m"},
				{Text: aw.userMenuBtn, CallbackData: "a:close"},
			},
		},
	}
	return withNotice(sb.String()), kb
}

func (b *Bot) adminUsersScreen(ctx context.Context, aw *adminWords, w *words, subarg string, withNotice func(string) string) (string, *tgate.Keyboard) {
	pageStr, query, _ := strings.Cut(subarg, ":")
	if pageStr == "search" {
		text := withNotice(aw.searchPrompt)
		return text, &tgate.Keyboard{
			InlineKeyboard: [][]tgate.Button{{{Text: aw.cancelBtn, CallbackData: "a:u:0"}}},
		}
	}
	if pageStr == "new" {
		text := withNotice(aw.newClientPrompt)
		return text, &tgate.Keyboard{
			InlineKeyboard: [][]tgate.Button{
				{{Text: aw.createAutoBtn, CallbackData: "a:u:cauto"}},
				{{Text: aw.cancelBtn, CallbackData: "a:u:0"}},
			},
		}
	}

	page, _ := strconv.Atoi(pageStr)
	if page < 0 {
		page = 0
	}
	pageSize := 8
	offset := page * pageSize

	resp, err := b.d.Backend.AdminListUsers(ctx, query, offset, pageSize)
	if err != nil {
		return withNotice(fmt.Sprintf(aw.errorNotice, err.Error())), &tgate.Keyboard{
			InlineKeyboard: [][]tgate.Button{{{Text: aw.backBtn, CallbackData: "a:m"}}},
		}
	}

	totalPages := (resp.TotalCount + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}

	var title string
	if query != "" {
		title = fmt.Sprintf(aw.usersSearchTitle, html.EscapeString(query), resp.TotalCount)
	} else {
		title = fmt.Sprintf(aw.usersTitle, resp.TotalCount, page+1, totalPages)
	}

	var sb strings.Builder
	sb.WriteString(title + "\n\n")

	if len(resp.Users) == 0 {
		sb.WriteString(aw.usersEmpty)
	} else {
		for i, u := range resp.Users {
			em := statusEmoji(u.Status)
			term := aw.forever
			if u.ExpiresAt > 0 {
				t := time.Unix(u.ExpiresAt, 0).UTC()
				term = w.shortDate(t)
			}
			traffic := formatBytes(u.TrafficUsed)
			if u.TrafficLimit > 0 {
				traffic += " / " + formatBytes(u.TrafficLimit)
			}
			sb.WriteString(fmt.Sprintf("%d. %s <b>%s</b> (#%d)\n   └ %s, %s\n", offset+i+1, em, html.EscapeString(u.Name), u.ID, term, traffic))
		}
	}

	var rows [][]tgate.Button
	for i := 0; i < len(resp.Users); i += 2 {
		var row []tgate.Button
		u1 := resp.Users[i]
		row = append(row, tgate.Button{
			Text:         fmt.Sprintf("%s %s (#%d)", statusEmoji(u1.Status), u1.Name, u1.ID),
			CallbackData: fmt.Sprintf("a:ud:%d", u1.ID),
		})
		if i+1 < len(resp.Users) {
			u2 := resp.Users[i+1]
			row = append(row, tgate.Button{
				Text:         fmt.Sprintf("%s %s (#%d)", statusEmoji(u2.Status), u2.Name, u2.ID),
				CallbackData: fmt.Sprintf("a:ud:%d", u2.ID),
			})
		}
		rows = append(rows, row)
	}

	var navRow []tgate.Button
	if page > 0 {
		prevData := fmt.Sprintf("a:u:%d", page-1)
		if query != "" {
			prevData += ":" + query
		}
		navRow = append(navRow, tgate.Button{Text: aw.prevBtn, CallbackData: prevData})
	}
	if (page+1)*pageSize < resp.TotalCount {
		nextData := fmt.Sprintf("a:u:%d", page+1)
		if query != "" {
			nextData += ":" + query
		}
		navRow = append(navRow, tgate.Button{Text: aw.nextBtn, CallbackData: nextData})
	}
	if len(navRow) > 0 {
		rows = append(rows, navRow)
	}

	rows = append(rows, []tgate.Button{
		{Text: aw.searchBtn, CallbackData: "a:u:search"},
		{Text: aw.newClientBtn, CallbackData: "a:u:new"},
	})
	rows = append(rows, []tgate.Button{
		{Text: aw.backBtn, CallbackData: "a:m"},
	})

	return withNotice(sb.String()), &tgate.Keyboard{InlineKeyboard: rows}
}

func (b *Bot) adminUserDetailScreen(ctx context.Context, aw *adminWords, id int64, withNotice func(string) string) (string, *tgate.Keyboard) {
	u, err := b.d.Backend.AdminGetUser(ctx, id)
	if err != nil {
		return withNotice(aw.userNotFound), &tgate.Keyboard{
			InlineKeyboard: [][]tgate.Button{{{Text: aw.backToListBtn, CallbackData: "a:u:0"}}},
		}
	}

	tariffName := aw.noTariff
	if u.TariffName != "" {
		tariffName = u.TariffName
	}
	term := aw.forever
	if u.ExpiresAt > 0 {
		exp := time.Unix(u.ExpiresAt, 0).UTC()
		left := int((exp.Sub(time.Now()) + 24*time.Hour - time.Second) / (24 * time.Hour))
		term = fmt.Sprintf("%s (%d дн.)", exp.Format("02.01.2006"), max(0, left))
	}
	traffic := formatBytes(u.TrafficUsed)
	if u.TrafficLimit > 0 {
		traffic += " / " + formatBytes(u.TrafficLimit)
	} else {
		traffic += " (" + aw.forever + ")"
	}
	devices := strconv.FormatInt(u.DeviceCount, 10)
	if u.DeviceLimit > 0 {
		devices += " / " + strconv.FormatInt(u.DeviceLimit, 10)
	}
	lastOnline := aw.neverOnline
	if u.OnlineAt > 0 {
		lastOnline = time.Unix(u.OnlineAt, 0).Format("02.01.2006 15:04")
	}
	created := time.Unix(u.CreatedAt, 0).Format("02.01.2006")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(aw.clientCardTitle, html.EscapeString(u.Name), u.ID) + "\n\n")
	sb.WriteString(fmt.Sprintf("%s %s %s\n", aw.statusLabel, statusEmoji(u.Status), u.Status))
	sb.WriteString(fmt.Sprintf("%s %s\n", aw.tariffLabel, html.EscapeString(tariffName)))
	sb.WriteString(fmt.Sprintf("%s %s\n", aw.termLabel, term))
	sb.WriteString(fmt.Sprintf("%s %s\n", aw.trafficLabel, traffic))
	sb.WriteString(fmt.Sprintf("%s %s\n", aw.devicesLabel, devices))
	sb.WriteString(fmt.Sprintf("%s %s\n", aw.onlineLabel, lastOnline))
	sb.WriteString(fmt.Sprintf("%s %s\n\n", aw.createdLabel, created))

	if u.SubURL != "" {
		sb.WriteString(aw.subLinkLabel + "\n<code>" + html.EscapeString(u.SubURL) + "</code>")
	}

	toggleBtnText := aw.disableBtn
	if u.RawStatus == "disabled" {
		toggleBtnText = aw.enableBtn
	}

	rows := [][]tgate.Button{
		{
			{Text: aw.extend30Btn, CallbackData: fmt.Sprintf("a:ue:%d:30", u.ID)},
			{Text: aw.extend7Btn, CallbackData: fmt.Sprintf("a:ue:%d:7", u.ID)},
		},
		{
			{Text: aw.resetTrafficBtn, CallbackData: fmt.Sprintf("a:ur:%d", u.ID)},
			{Text: toggleBtnText, CallbackData: fmt.Sprintf("a:ut:%d", u.ID)},
		},
		{
			{Text: fmt.Sprintf(aw.devicesBtn, u.DeviceCount), CallbackData: fmt.Sprintf("a:dev:%d", u.ID)},
			{Text: aw.sendLinkBtn, CallbackData: fmt.Sprintf("a:us:%d", u.ID)},
		},
		{
			{Text: aw.backToListBtn, CallbackData: "a:u:0"},
		},
	}
	return withNotice(sb.String()), &tgate.Keyboard{InlineKeyboard: rows}
}

func (b *Bot) adminDevicesScreen(ctx context.Context, aw *adminWords, userID int64, withNotice func(string) string) (string, *tgate.Keyboard) {
	u, err := b.d.Backend.AdminGetUser(ctx, userID)
	if err != nil {
		return withNotice(aw.userNotFound), &tgate.Keyboard{
			InlineKeyboard: [][]tgate.Button{{{Text: aw.backToListBtn, CallbackData: "a:u:0"}}},
		}
	}
	devs, _ := b.d.Backend.Devices(ctx, userID)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(aw.devicesTitle, html.EscapeString(u.Name), len(devs)) + "\n\n")

	var rows [][]tgate.Button
	if len(devs) == 0 {
		sb.WriteString(aw.devicesEmpty)
	} else {
		for i, d := range devs {
			name := d.Os
			if d.Model != "" {
				name += " " + d.Model
			}
			if name == "" {
				name = "Device #" + strconv.FormatInt(d.ID, 10)
			}
			lastSeen := aw.neverOnline
			if d.LastSeen > 0 {
				lastSeen = time.Unix(d.LastSeen, 0).Format("02.01.2006 15:04")
			}
			sb.WriteString(fmt.Sprintf("%d. 📱 <b>%s</b> (%s)\n   IP: %s | Онлайн: %s\n", i+1, html.EscapeString(name), html.EscapeString(d.App), d.LastIp, lastSeen))

			rows = append(rows, []tgate.Button{{
				Text:         fmt.Sprintf("%s: %s", aw.unbindDeviceBtn, name),
				CallbackData: fmt.Sprintf("a:dd:%d:%d", userID, d.ID),
			}})
		}
	}

	rows = append(rows, []tgate.Button{{Text: aw.backToClientBtn, CallbackData: fmt.Sprintf("a:ud:%d", userID)}})
	return withNotice(sb.String()), &tgate.Keyboard{InlineKeyboard: rows}
}

func (b *Bot) adminNodesScreen(ctx context.Context, aw *adminWords, withNotice func(string) string) (string, *tgate.Keyboard) {
	nodes, err := b.d.Backend.AdminListNodes(ctx)
	if err != nil {
		return withNotice(fmt.Sprintf(aw.errorNotice, err.Error())), &tgate.Keyboard{
			InlineKeyboard: [][]tgate.Button{{{Text: aw.backBtn, CallbackData: "a:m"}}},
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(aw.nodesTitle, len(nodes)) + "\n\n")

	var rows [][]tgate.Button
	for _, n := range nodes {
		stText := aw.statusOnline
		if !n.Enabled {
			stText = aw.statusDisabled
		} else if !n.Online {
			stText = aw.statusOffline
		}

		host := n.Address
		if n.PublicHost != "" {
			host = n.PublicHost
		}

		if n.Online {
			sb.WriteString(fmt.Sprintf("🟢 <b>%s</b> (%s)\n   └ CPU: %.1f%% | RAM: %s / %s | %s %d\n",
				html.EscapeString(n.Name), host, n.CPUPercent, formatBytes(int64(n.MemUsed)), formatBytes(int64(n.MemTotal)), aw.connsLabel, n.ActiveConns))
		} else if !n.Enabled {
			sb.WriteString(fmt.Sprintf("⚪️ <b>%s</b> (%s) — %s\n", html.EscapeString(n.Name), host, aw.statusDisabled))
		} else {
			errMsg := n.Error
			if errMsg == "" {
				errMsg = "offline"
			}
			sb.WriteString(fmt.Sprintf("🔴 <b>%s</b> (%s)\n   └ %s %s\n", html.EscapeString(n.Name), host, aw.errorLabel, html.EscapeString(errMsg)))
		}

		rows = append(rows, []tgate.Button{{
			Text:         fmt.Sprintf("%s %s", stText[:2], n.Name),
			CallbackData: fmt.Sprintf("a:nd:%d", n.ID),
		}})
	}

	rows = append(rows, []tgate.Button{
		{Text: aw.refreshBtn, CallbackData: "a:n"},
		{Text: aw.backBtn, CallbackData: "a:m"},
	})
	return withNotice(sb.String()), &tgate.Keyboard{InlineKeyboard: rows}
}

func (b *Bot) adminNodeDetailScreen(ctx context.Context, aw *adminWords, id int64, withNotice func(string) string) (string, *tgate.Keyboard) {
	nodes, err := b.d.Backend.AdminListNodes(ctx)
	if err != nil {
		return withNotice(fmt.Sprintf(aw.errorNotice, err.Error())), &tgate.Keyboard{
			InlineKeyboard: [][]tgate.Button{{{Text: aw.nodesBtn, CallbackData: "a:n"}}},
		}
	}
	var node *AdminNodeSummary
	for _, n := range nodes {
		if n.ID == id {
			node = &n
			break
		}
	}
	if node == nil {
		return withNotice("Нода не найдена."), &tgate.Keyboard{
			InlineKeyboard: [][]tgate.Button{{{Text: aw.nodesBtn, CallbackData: "a:n"}}},
		}
	}

	stText := aw.statusOnline
	if !node.Enabled {
		stText = aw.statusDisabled
	} else if !node.Online {
		stText = aw.statusOffline
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(aw.nodeDetailTitle, html.EscapeString(node.Name)) + " (ID: <code>" + strconv.FormatInt(node.ID, 10) + "</code>)\n\n")
	sb.WriteString(fmt.Sprintf("%s %s\n", aw.statusLabel, stText))
	sb.WriteString(fmt.Sprintf("📍 Адрес: <code>%s</code>\n", html.EscapeString(node.Address)))
	if node.PublicHost != "" {
		sb.WriteString(fmt.Sprintf("🌐 Public Host: <code>%s</code>\n", html.EscapeString(node.PublicHost)))
	}
	if node.Version != "" {
		sb.WriteString(fmt.Sprintf("%s %s\n", aw.versionLabel, html.EscapeString(node.Version)))
	}
	if node.Online {
		sb.WriteString(fmt.Sprintf("%s %.1f%%\n", aw.cpuLabel, node.CPUPercent))
		sb.WriteString(fmt.Sprintf("%s %s / %s\n", aw.ramLabel, formatBytes(int64(node.MemUsed)), formatBytes(int64(node.MemTotal))))
		sb.WriteString(fmt.Sprintf("%s %s / %s\n", aw.netSpeedLabel, formatSpeed(node.NetRxBps), formatSpeed(node.NetTxBps)))
		sb.WriteString(fmt.Sprintf("%s %d\n", aw.connsLabel, node.ActiveConns))
	} else if node.Error != "" {
		sb.WriteString(fmt.Sprintf("%s %s\n", aw.errorLabel, html.EscapeString(node.Error)))
	}

	rows := [][]tgate.Button{
		{{Text: aw.refreshBtn, CallbackData: fmt.Sprintf("a:nd:%d", id)}},
		{{Text: aw.nodesBtn, CallbackData: "a:n"}},
	}
	return withNotice(sb.String()), &tgate.Keyboard{InlineKeyboard: rows}
}

// adminAct executes an admin action and returns the destination screen and notice.
func (b *Bot) adminAct(ctx context.Context, chat int64, data string) (screen, notice string) {
	cfg := b.config(ctx)
	aw := adminWordsFor(cfg.Lang)

	parts := strings.Split(data, ":")
	if len(parts) < 2 {
		return "a:m", ""
	}

	switch parts[1] {
	case "m":
		b.setAdminState(chat, "")
		return "a:m", ""
	case "u":
		if len(parts) >= 3 {
			switch parts[2] {
			case "search":
				b.setAdminState(chat, "search")
				return "a:u:search", ""
			case "new":
				b.setAdminState(chat, "new_user")
				return "a:u:new", ""
			case "cauto":
				b.setAdminState(chat, "")
				u, err := b.d.Backend.AdminCreateUser(ctx, "")
				if err != nil {
					return "a:m", fmt.Sprintf(aw.errorNotice, err.Error())
				}
				return fmt.Sprintf("a:ud:%d", u.ID), aw.userCreatedNotice
			}
		}
		b.setAdminState(chat, "")
		return data, ""
	case "ue":
		// a:ue:<id>:<days>
		if len(parts) >= 4 {
			id, _ := strconv.ParseInt(parts[2], 10, 64)
			days, _ := strconv.ParseInt(parts[3], 10, 64)
			if err := b.d.Backend.AdminExtendUser(ctx, id, days); err != nil {
				return fmt.Sprintf("a:ud:%d", id), fmt.Sprintf(aw.errorNotice, err.Error())
			}
			return fmt.Sprintf("a:ud:%d", id), fmt.Sprintf(aw.extendedNotice, days)
		}
	case "ur":
		// a:ur:<id>
		if len(parts) >= 3 {
			id, _ := strconv.ParseInt(parts[2], 10, 64)
			if err := b.d.Backend.AdminResetTraffic(ctx, id); err != nil {
				return fmt.Sprintf("a:ud:%d", id), fmt.Sprintf(aw.errorNotice, err.Error())
			}
			return fmt.Sprintf("a:ud:%d", id), aw.trafficResetNotice
		}
	case "ut":
		// a:ut:<id>
		if len(parts) >= 3 {
			id, _ := strconv.ParseInt(parts[2], 10, 64)
			st, err := b.d.Backend.AdminToggleUser(ctx, id)
			if err != nil {
				return fmt.Sprintf("a:ud:%d", id), fmt.Sprintf(aw.errorNotice, err.Error())
			}
			return fmt.Sprintf("a:ud:%d", id), fmt.Sprintf(aw.statusNotice, st)
		}
	case "dd":
		// a:dd:<userID>:<deviceID>
		if len(parts) >= 4 {
			userID, _ := strconv.ParseInt(parts[2], 10, 64)
			devID, _ := strconv.ParseInt(parts[3], 10, 64)
			if err := b.d.Backend.AdminDeleteDevice(ctx, userID, devID); err != nil {
				return fmt.Sprintf("a:dev:%d", userID), fmt.Sprintf(aw.errorNotice, err.Error())
			}
			return fmt.Sprintf("a:dev:%d", userID), aw.deviceUnboundNotice
		}
	case "us":
		// a:us:<id> (send link to chat)
		if len(parts) >= 3 {
			id, _ := strconv.ParseInt(parts[2], 10, 64)
			u, err := b.d.Backend.AdminGetUser(ctx, id)
			if err == nil && u.SubURL != "" {
				b.d.Outbox.Reply(chat, "", 1, func(ctx context.Context, c *tgate.Client) error {
					_, err := c.Send(ctx, chat, fmt.Sprintf("🔗 <b>%s</b>:\n<code>%s</code>", html.EscapeString(u.Name), html.EscapeString(u.SubURL)), nil, false)
					return err
				})
				return fmt.Sprintf("a:ud:%d", id), aw.linkSentNotice
			}
			return fmt.Sprintf("a:ud:%d", id), fmt.Sprintf(aw.errorNotice, "no link")
		}
	}

	b.setAdminState(chat, "")
	return data, ""
}
