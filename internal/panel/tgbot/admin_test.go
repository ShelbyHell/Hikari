package tgbot

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"mikan/internal/panel/settings"
	"mikan/internal/panel/tgate"
)

func TestAdminAuthorization(t *testing.T) {
	e := setup(t)
	ctx := context.Background()

	const normalUser, infraAdmin, otherAdmin = 100, 200, 300

	// Initially neither is admin.
	if e.bot.d.Backend.IsAdmin(ctx, normalUser) {
		t.Fatalf("normalUser should not be admin")
	}
	if e.bot.d.Backend.IsAdmin(ctx, infraAdmin) {
		t.Fatalf("infraAdmin should not be admin before setting")
	}

	// Set infrastructure admin chat.
	if err := settings.Set(ctx, e.set, tgate.KeyInfraAdminChat, int64(infraAdmin)); err != nil {
		t.Fatalf("set KeyInfraAdminChat: %v", err)
	}

	if !e.bot.d.Backend.IsAdmin(ctx, infraAdmin) {
		t.Fatalf("infraAdmin should be admin now")
	}
	if e.bot.d.Backend.IsAdmin(ctx, normalUser) {
		t.Fatalf("normalUser should still not be admin")
	}

	// Add additional admin via telegram_admin_ids.
	if err := settings.Set(ctx, e.set, "telegram_admin_ids", "300, 400"); err != nil {
		t.Fatalf("set telegram_admin_ids: %v", err)
	}

	if !e.bot.d.Backend.IsAdmin(ctx, otherAdmin) {
		t.Fatalf("otherAdmin (300) should be admin")
	}
	if !e.bot.d.Backend.IsAdmin(ctx, 400) {
		t.Fatalf("admin 400 should be admin")
	}
	if e.bot.d.Backend.IsAdmin(ctx, normalUser) {
		t.Fatalf("normalUser should still not be admin")
	}
}

func TestAdminOverviewAndUsers(t *testing.T) {
	e := setup(t)
	ctx := context.Background()

	// Overview
	ov, err := e.bot.d.Backend.AdminOverview(ctx)
	if err != nil {
		t.Fatalf("AdminOverview: %v", err)
	}
	if ov.TotalUsers < 1 {
		t.Fatalf("expected at least 1 user from setup, got %d", ov.TotalUsers)
	}

	// Create user
	created, err := e.bot.d.Backend.AdminCreateUser(ctx, "TestAdminClient")
	if err != nil {
		t.Fatalf("AdminCreateUser: %v", err)
	}
	if created.Name != "TestAdminClient" {
		t.Fatalf("created user name: %q, want TestAdminClient", created.Name)
	}

	// List users
	page, err := e.bot.d.Backend.AdminListUsers(ctx, "", 0, 10)
	if err != nil {
		t.Fatalf("AdminListUsers: %v", err)
	}
	if page.TotalCount < 2 {
		t.Fatalf("expected at least 2 users, got %d", page.TotalCount)
	}

	// Search user
	searchPage, err := e.bot.d.Backend.AdminListUsers(ctx, "TestAdminClient", 0, 10)
	if err != nil {
		t.Fatalf("AdminListUsers search: %v", err)
	}
	if len(searchPage.Users) != 1 || searchPage.Users[0].ID != created.ID {
		t.Fatalf("search did not find created user: %+v", searchPage)
	}

	// Get user
	detail, err := e.bot.d.Backend.AdminGetUser(ctx, created.ID)
	if err != nil {
		t.Fatalf("AdminGetUser: %v", err)
	}
	if detail.ID != created.ID || detail.Name != "TestAdminClient" {
		t.Fatalf("detail mismatch: %+v", detail)
	}

	// Extend user
	if err := e.bot.d.Backend.AdminExtendUser(ctx, created.ID, 30); err != nil {
		t.Fatalf("AdminExtendUser: %v", err)
	}
	detailAfterExtend, err := e.bot.d.Backend.AdminGetUser(ctx, created.ID)
	if err != nil {
		t.Fatalf("AdminGetUser after extend: %v", err)
	}
	if detailAfterExtend.ExpiresAt <= detail.ExpiresAt {
		t.Fatalf("expected expires_at to increase after extend")
	}

	// Reset traffic
	if err := e.bot.d.Backend.AdminResetTraffic(ctx, created.ID); err != nil {
		t.Fatalf("AdminResetTraffic: %v", err)
	}

	// Toggle user active <-> disabled
	newSt, err := e.bot.d.Backend.AdminToggleUser(ctx, created.ID)
	if err != nil {
		t.Fatalf("AdminToggleUser: %v", err)
	}
	if newSt != "disabled" {
		t.Fatalf("expected disabled, got %q", newSt)
	}
	newSt2, err := e.bot.d.Backend.AdminToggleUser(ctx, created.ID)
	if err != nil {
		t.Fatalf("AdminToggleUser 2: %v", err)
	}
	if newSt2 != "active" {
		t.Fatalf("expected active, got %q", newSt2)
	}

	// List nodes
	nodes, err := e.bot.d.Backend.AdminListNodes(ctx)
	if err != nil {
		t.Fatalf("AdminListNodes: %v", err)
	}
	_ = nodes
}

func TestAdminBotFlow(t *testing.T) { testAdminBotFlow(t) }

func TestAdminBotFlowOverRPC(t *testing.T) {
	testAdminBotFlow(t, func(_ *env, w *wiring) { w.RPC = true })
}

func testAdminBotFlow(t *testing.T, with ...func(*env, *wiring)) {
	e := setup(t, with...)
	ctx := context.Background()

	const adminChat = 888
	const normalChat = 999

	if err := settings.Set(ctx, e.set, tgate.KeyInfraAdminChat, int64(adminChat)); err != nil {
		t.Fatalf("set admin chat: %v", err)
	}

	// 1. Non-admin types /admin -> should NOT see admin panel
	n := e.tg.count()
	e.say(normalChat, "/admin")
	calls := e.tg.wait(t, n, "sendMessage")
	send, _ := find(calls, "sendMessage")
	if strings.Contains(text(send), "Панель управления Hikari") || strings.Contains(text(send), "Hikari Admin Panel") {
		t.Fatalf("non-admin got admin panel: %s", text(send))
	}

	// 2. Admin types /admin -> should see admin dashboard
	n = e.tg.count()
	e.say(adminChat, "/admin")
	calls = e.tg.wait(t, n, "sendMessage")
	send, _ = find(calls, "sendMessage")
	if !strings.Contains(text(send), "Панель управления Hikari") {
		t.Fatalf("admin /admin did not show dashboard: %s", text(send))
	}
	btns := buttons(send)
	if btns["👥 Клиенты"] != "a:u:0" || btns["➕ Новый клиент"] != "a:u:new" || btns["🖥 Ноды"] != "a:n" {
		t.Fatalf("admin buttons missing: %+v", btns)
	}

	msgID := int64(1)
	if m, ok := send.body["message_id"].(float64); ok {
		msgID = int64(m)
	}

	// 3. Admin presses "👥 Клиенты" (a:u:0)
	n = e.tg.count()
	e.press(adminChat, msgID, "a:u:0")
	calls = e.tg.wait(t, n, "editMessageText")
	edit, _ := find(calls, "editMessageText")
	if !strings.Contains(text(edit), "Клиенты") {
		t.Fatalf("expected clients screen, got: %s", text(edit))
	}
	clientBtns := buttons(edit)
	if clientBtns["« Главное меню"] != "a:m" {
		t.Fatalf("back button missing: %+v", clientBtns)
	}

	// 4. Admin presses user detail (a:ud:<id>)
	userID := e.user.ID
	n = e.tg.count()
	e.press(adminChat, msgID, "a:ud:"+strconv.FormatInt(userID, 10))
	calls = e.tg.wait(t, n, "editMessageText")
	edit, _ = find(calls, "editMessageText")
	if !strings.Contains(text(edit), "Клиент:") && !strings.Contains(text(edit), "Анна") {
		t.Fatalf("expected user detail card, got: %s", text(edit))
	}
	detailBtns := buttons(edit)
	if detailBtns["⏳ +30 дней"] == "" || detailBtns["🔄 Сброс трафика"] == "" {
		t.Fatalf("action buttons missing on user detail card: %+v", detailBtns)
	}

	// 5. Admin presses extend +30 days (a:ue:<id>:30)
	n = e.tg.count()
	e.press(adminChat, msgID, detailBtns["⏳ +30 дней"])
	calls = e.tg.wait(t, n, "editMessageText")
	edit, _ = find(calls, "editMessageText")
	if !strings.Contains(text(edit), "Срок продлён") {
		t.Fatalf("expected extended notice, got: %s", text(edit))
	}

	// 6. Admin presses "🖥 Ноды" (a:n)
	n = e.tg.count()
	e.press(adminChat, msgID, "a:n")
	calls = e.tg.wait(t, n, "editMessageText")
	edit, _ = find(calls, "editMessageText")
	if !strings.Contains(text(edit), "Серверы и ноды") {
		t.Fatalf("expected nodes screen, got: %s", text(edit))
	}

	// 7. Admin searches with /admin <query>
	n = e.tg.count()
	e.say(adminChat, "/admin Анна")
	calls = e.tg.wait(t, n, "sendMessage")
	send, _ = find(calls, "sendMessage")
	if !strings.Contains(text(send), "Поиск: «Анна»") || !strings.Contains(text(send), "Анна") {
		t.Fatalf("expected search results for Анна, got: %s", text(send))
	}

	// 8. Admin presses "👤 Пользовательский вид" (a:close)
	n = e.tg.count()
	e.press(adminChat, msgID, "a:close")
	calls = e.tg.wait(t, n, "editMessageText")
	edit, _ = find(calls, "editMessageText")
	// Should show standard subscriber menu
	if strings.Contains(text(edit), "Панель управления Hikari") {
		t.Fatalf("expected subscriber view, still got admin panel: %s", text(edit))
	}
}
