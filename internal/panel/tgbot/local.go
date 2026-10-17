package tgbot

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"mikan/internal/panel/billing"
	"mikan/internal/panel/domain"
	"mikan/internal/panel/nodesync"
	"mikan/internal/panel/panelimport"
	"mikan/internal/panel/settings"
	"mikan/internal/panel/store"
	"mikan/internal/panel/store/db"
	"mikan/internal/panel/tgate"
)

// Local is the Backend inside the panel's process: its store, settings, users, devices and
// billing. It is the only part of the bot that touches them; the bot addon reaches it over
// the Panel API (RPCHandler), and the panel's side of the bot (Panel) calls it directly.
type Local struct {
	Store    *store.Store
	Settings *settings.Settings
	// Binder unbinds devices at their subscriber's request.
	Binder *domain.Devices
	// Users reissues a subscription's link at its subscriber's request.
	Users *domain.Users
	// Nodes provides live metrics and status of nodes.
	Nodes *nodesync.Manager
	// Billing sells tariffs in the menu.
	Billing *billing.Service
	// Gateway reads the link codes it signed and asks who is in the channel.
	Gateway *tgate.Gateway
	// SubBase is https://host:port/<sub path>, "" while the panel has no public address.
	SubBase func(ctx context.Context) string
	// MiniApp: the panel's certificate is one Telegram apps accept (not self-signed).
	MiniApp func() bool
	Now     func() time.Time
}

func (l *Local) Config(ctx context.Context) Config {
	// A setup saved before an option existed gets that option's default. A saved menu
	// replaces the default one whole.
	cfg, lang, saved := tgate.ReadSetup(ctx, l.Settings, func(lang string) Config {
		def := Default(lang)
		def.Buttons = nil
		return def
	})
	if !saved {
		return Default(lang)
	}
	_ = cfg.Validate()
	return cfg
}

// Member is the channel check, for the bot's own channel only: the screens cannot probe
// other chats or reset what the gateway knows about members.
func (l *Local) Member(ctx context.Context, chat string, tgID int64, fresh bool) (bool, error) {
	if ch := l.Gateway.ChannelConfig(ctx).Chat; ch == "" || chat != ch {
		return false, ErrNotChannel
	}
	return l.Gateway.Member(ctx, chat, tgID, fresh)
}

func (l *Local) Brand(ctx context.Context) string {
	s, _ := l.Settings.String(ctx, settings.KeyBrand)
	return s
}

func (l *Local) SupportURL(ctx context.Context) string {
	s, _ := l.Settings.String(ctx, settings.KeySupportURL)
	return s
}

func (l *Local) SubURL(ctx context.Context, u db.User) string {
	if base := l.SubBase(ctx); base != "" {
		return base + "/" + u.SubToken
	}
	return ""
}

func (l *Local) MiniAppURL(ctx context.Context) string {
	base := l.SubBase(ctx)
	if base == "" || l.MiniApp == nil || !l.MiniApp() {
		return ""
	}
	return base + "/tg"
}

func (l *Local) Context(ctx context.Context, tgID int64) (Account, error) {
	list, err := l.Store.Q.ListTgLinksOf(ctx, tgID)
	if err != nil {
		return Account{}, err
	}
	acc := Account{Links: list}
	if ch, err := l.Store.Q.GetTgChat(ctx, tgID); err == nil {
		acc.Chat, acc.HasChat = ch, true
	}
	return acc, nil
}

func (l *Local) Chat(ctx context.Context, tgID int64) (db.TgChat, error) {
	return l.Store.Q.GetTgChat(ctx, tgID)
}

func (l *Local) SetMenu(ctx context.Context, tgID, msgID int64) error {
	return l.Store.Q.SetTgMenu(ctx, db.SetTgMenuParams{MenuMsgID: msgID, TgID: tgID})
}

func (l *Local) SetCurrent(ctx context.Context, tgID, userID int64) error {
	return l.Store.Q.SetTgCurrent(ctx, db.SetTgCurrentParams{Current: userID, TgID: tgID})
}

func (l *Local) UserOfCode(ctx context.Context, code string) (int64, error) {
	return l.Gateway.UserOfCode(ctx, code)
}

// UserOfAddress: the own token is a path segment of 24 letters and digits. An old panel's
// address is taken as the panel serves it (panelimport.LinkToken), and only while old
// links are set up; the signature of a token Marzban or PasarGuard signed is checked as
// there (Verifier.User).
func (l *Local) UserOfAddress(ctx context.Context, urlPath string) (int64, bool) {
	for seg := range strings.SplitSeq(urlPath, "/") {
		if !nativeToken.MatchString(seg) {
			continue
		}
		if u, err := l.Store.Q.GetUserBySubToken(ctx, seg); err == nil {
			return u.ID, true
		}
	}
	legacyPath, verifier, err := panelimport.LoadLegacy(ctx, l.Settings)
	if err != nil {
		return 0, false
	}
	token, _, ok := panelimport.LinkToken(urlPath, legacyPath)
	if !ok {
		return 0, false
	}
	if u, ok := verifier.User(ctx, l.Store.Q, token); ok {
		return u.ID, true
	}
	return 0, false
}

// Link: a subscription has one owner. An owner who cannot be asked, as they blocked the
// bot, is not in the way.
func (l *Local) Link(ctx context.Context, tgID, userID int64) (Linked, error) {
	u, err := l.Store.Q.GetUser(ctx, userID)
	if err != nil {
		return Linked{}, err
	}
	prev, err := l.Store.Q.GetTgLink(ctx, userID)
	switch {
	case err == nil && prev.TgID == tgID:
		_ = l.SetCurrent(ctx, tgID, userID)
		return Linked{Status: LinkAlready, User: u}, nil
	case err != nil && !errors.Is(err, sql.ErrNoRows):
		return Linked{}, err
	}
	if n, err := l.Store.Q.CountTgLinksOf(ctx, tgID); err != nil || n >= tgate.MaxLinks {
		return Linked{Status: LinkLimit, User: u}, nil
	}
	if prev.TgID != 0 && l.reachable(ctx, prev.TgID) {
		return Linked{Status: LinkTaken, User: u, Owner: prev.TgID}, nil
	}
	if err := l.Store.Q.LinkTg(ctx, db.LinkTgParams{UserID: userID, TgID: tgID, CreatedAt: l.Now().Unix()}); err != nil {
		return Linked{}, err
	}
	_ = l.SetCurrent(ctx, tgID, userID)
	return Linked{Status: LinkDone, User: u}, nil
}

// reachable: the account has a chat with the bot and has not blocked it, so it can be asked.
func (l *Local) reachable(ctx context.Context, tgID int64) bool {
	ch, err := l.Store.Q.GetTgChat(ctx, tgID)
	return err == nil && ch.Blocked == 0
}

func (l *Local) Owner(ctx context.Context, userID int64) (int64, error) {
	link, err := l.Store.Q.GetTgLink(ctx, userID)
	return link.TgID, err
}

func (l *Local) Transfer(ctx context.Context, userID, to int64) error {
	if n, err := l.Store.Q.CountTgLinksOf(ctx, to); err != nil || n >= tgate.MaxLinks {
		return ErrLinkLimit
	}
	if err := l.Store.Q.LinkTg(ctx, db.LinkTgParams{UserID: userID, TgID: to, CreatedAt: l.Now().Unix()}); err != nil {
		return err
	}
	_ = l.SetCurrent(ctx, to, userID)
	return nil
}

func (l *Local) Unlink(ctx context.Context, userID int64) error {
	return l.Store.Q.UnlinkTg(ctx, userID)
}

func (l *Local) User(ctx context.Context, id int64) (db.User, error) {
	return l.Store.Q.GetUser(ctx, id)
}

func (l *Local) GrantsLeft(ctx context.Context, userID int64, now time.Time) (domain.GrantsLeft, error) {
	return domain.UserGrantsLeft(ctx, l.Store.Q, userID, now)
}

func (l *Local) Pools(ctx context.Context, userID int64, now time.Time) ([]Pool, error) {
	rows, err := l.Store.Q.ListUserPools(ctx, userID)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	pools, err := l.Store.Q.ListTrafficPools(ctx)
	if err != nil {
		return nil, err
	}
	grants, err := domain.UserGrantsLeft(ctx, l.Store.Q, userID, now)
	if err != nil {
		return nil, err
	}
	names := map[int64]string{}
	for _, p := range pools {
		names[p.ID] = p.Name
	}
	var out []Pool
	for _, r := range rows {
		if !r.TrafficLimit.Valid {
			continue
		}
		extra := grants.Pool(userID, r.PoolID)
		out = append(out, Pool{Name: names[r.PoolID], Used: r.UsedUp + r.UsedDown, Limit: r.TrafficLimit.Int64, Extra: extra,
			Exhausted: domain.PoolExhausted(r, extra)})
	}
	return out, nil
}

func (l *Local) DeviceBinding(ctx context.Context) bool {
	on, _ := l.Settings.On(ctx, settings.DeviceBinding)
	return on
}

func (l *Local) DeviceCount(ctx context.Context, userID int64) (int64, error) {
	return l.Store.Q.CountBoundDevices(ctx, userID)
}

func (l *Local) Devices(ctx context.Context, userID int64) ([]db.BoundDevice, error) {
	return l.Store.Q.ListBoundDevices(ctx, userID)
}

func (l *Local) DeviceTraffic(ctx context.Context, userID int64) (map[int64]int64, error) {
	rows, err := l.Store.Q.ListBoundDeviceTraffic(ctx, userID)
	if err != nil {
		return nil, err
	}
	used := map[int64]int64{}
	for _, r := range rows {
		used[r.DeviceID] = r.Up + r.Down
	}
	return used, nil
}

func (l *Local) UnbindState(ctx context.Context, userID int64) (domain.UnbindState, error) {
	return l.Binder.UnbindState(ctx, userID)
}

func (l *Local) Unbind(ctx context.Context, userID, deviceID int64) error {
	return l.Binder.Unbind(ctx, userID, deviceID, true)
}

func (l *Local) ReissueState(ctx context.Context, userID int64) (domain.ReissueState, error) {
	return l.Users.ReissueState(ctx, userID)
}

func (l *Local) Reissue(ctx context.Context, userID int64) error {
	_, err := l.Users.ReissueByOwner(ctx, userID, "bot")
	return err
}

func (l *Local) Notices(ctx context.Context, now time.Time) ([]db.ListNoticeSubscriptionsRow, error) {
	return l.Store.Q.ListNoticeSubscriptions(ctx, now.Unix())
}

func (l *Local) NoticeSent(ctx context.Context, userID int64, kind string, period int64) (bool, error) {
	return l.Store.Q.TgNoticeSent(ctx, db.TgNoticeSentParams{UserID: userID, Kind: kind, Period: period})
}

func (l *Local) NoticeDelivered(ctx context.Context, userID int64, kind string, period int64, at time.Time) error {
	_, err := l.Store.Q.AddTgNotice(ctx, db.AddTgNoticeParams{UserID: userID, Kind: kind, Period: period, SentAt: at.Unix()})
	return err
}

func (l *Local) PruneNotices(ctx context.Context, before time.Time) error {
	return l.Store.Q.PruneTgNotices(ctx, before.Unix())
}

func (l *Local) BroadcastTargets(ctx context.Context) ([]int64, error) {
	return l.Store.Q.BroadcastTargets(ctx)
}

func (l *Local) Shop() Shop { return l.Billing }

func (l *Local) Tariff(ctx context.Context, id int64) (db.Tariff, error) {
	return l.Store.Q.GetTariff(ctx, id)
}

func (l *Local) IsAdmin(ctx context.Context, tgID int64) bool {
	if tgID == 0 {
		return false
	}
	if adminChat, ok, _ := settings.Get[int64](ctx, l.Settings, tgate.KeyInfraAdminChat); ok && adminChat != 0 && adminChat == tgID {
		return true
	}
	if adminList, ok, _ := settings.Get[string](ctx, l.Settings, "telegram_admin_ids"); ok && adminList != "" {
		for _, part := range strings.Split(adminList, ",") {
			if id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64); err == nil && id == tgID {
				return true
			}
		}
	}
	return false
}

func (l *Local) AdminOverview(ctx context.Context) (AdminOverview, error) {
	now := l.Now()
	states, err := domain.CountStates(ctx, l.Store.Q, now)
	if err != nil {
		return AdminOverview{}, err
	}
	var online24h int64
	since := now.Unix() - 86400
	row := l.Store.DB.QueryRowContext(ctx, "SELECT count(*) FROM users WHERE online_at >= $1", since)
	_ = row.Scan(&online24h)

	nodes, _ := l.Store.Q.ListNodes(ctx)
	totalNodes := len(nodes)
	onlineNodes := 0
	offlineNodes := 0
	for _, n := range nodes {
		var ok bool
		var hv nodesync.HealthView
		if l.Nodes != nil {
			hv, ok = l.Nodes.Health(n.ID)
		}
		if n.Enabled != 0 && ok && hv.OK {
			onlineNodes++
		} else {
			offlineNodes++
		}
	}
	return AdminOverview{
		TotalUsers:    states.Total,
		ActiveUsers:   states.Active,
		ExpiringUsers: states.Expiring,
		LimitedUsers:  states.Limited,
		ExpiredUsers:  states.Expired,
		DisabledUsers: states.Disabled,
		Online24h:     online24h,
		TotalNodes:    totalNodes,
		OnlineNodes:   onlineNodes,
		OfflineNodes:  offlineNodes,
	}, nil
}

func (l *Local) AdminListUsers(ctx context.Context, query string, offset, limit int) (AdminUsersPage, error) {
	if limit <= 0 || limit > 50 {
		limit = 8
	}
	if offset < 0 {
		offset = 0
	}
	q := strings.TrimSpace(query)

	var total int
	countRow := l.Store.DB.QueryRowContext(ctx, `
		SELECT count(*) FROM users
		WHERE ($1 = '' OR name ILIKE '%' || $1 || '%' OR sub_token ILIKE '%' || $1 || '%' OR CAST(id AS TEXT) = $1)
	`, q)
	if err := countRow.Scan(&total); err != nil {
		return AdminUsersPage{}, err
	}

	rows, err := l.Store.DB.QueryContext(ctx, `
		SELECT id, name, status, tariff_id, traffic_limit, device_limit, reset_strategy,
		       period_days, period_start, used_up, used_down, total_up, total_down,
		       expires_at, sub_token, slot_id, online_at, created_at, updated_at, source
		FROM users
		WHERE ($1 = '' OR name ILIKE '%' || $1 || '%' OR sub_token ILIKE '%' || $1 || '%' OR CAST(id AS TEXT) = $1)
		ORDER BY id DESC
		LIMIT $2 OFFSET $3
	`, q, limit, offset)
	if err != nil {
		return AdminUsersPage{}, err
	}
	defer rows.Close()

	now := l.Now()
	var users []AdminUserSummary
	for rows.Next() {
		var u db.User
		if err := rows.Scan(
			&u.ID, &u.Name, &u.Status, &u.TariffID, &u.TrafficLimit, &u.DeviceLimit, &u.ResetStrategy,
			&u.PeriodDays, &u.PeriodStart, &u.UsedUp, &u.UsedDown, &u.TotalUp, &u.TotalDown,
			&u.ExpiresAt, &u.SubToken, &u.SlotID, &u.OnlineAt, &u.CreatedAt, &u.UpdatedAt, &u.Source,
		); err != nil {
			return AdminUsersPage{}, err
		}
		grants, _ := l.GrantsLeft(ctx, u.ID, now)
		st := domain.State(u, grants.Main(u.ID), now)
		var exp int64
		if u.ExpiresAt.Valid {
			exp = u.ExpiresAt.Int64
		}
		var lim int64
		if u.TrafficLimit.Valid {
			lim = u.TrafficLimit.Int64
		}
		users = append(users, AdminUserSummary{
			ID:           u.ID,
			Name:         u.Name,
			Status:       st,
			ExpiresAt:    exp,
			TrafficUsed:  u.UsedUp + u.UsedDown,
			TrafficLimit: lim,
		})
	}
	return AdminUsersPage{
		Users:      users,
		TotalCount: total,
		Offset:     offset,
		Limit:      limit,
	}, nil
}

func (l *Local) AdminGetUser(ctx context.Context, id int64) (AdminUserDetail, error) {
	u, err := l.Users.Get(ctx, id)
	if err != nil {
		return AdminUserDetail{}, err
	}
	now := l.Now()
	grants, _ := l.GrantsLeft(ctx, u.ID, now)
	st := domain.State(u, grants.Main(u.ID), now)

	var tariffName string
	if u.TariffID.Valid {
		if t, err := l.Tariff(ctx, u.TariffID.Int64); err == nil {
			tariffName = t.Name
		}
	}
	devCount, _ := l.DeviceCount(ctx, u.ID)
	var devLimit int64
	if u.DeviceLimit.Valid {
		devLimit = u.DeviceLimit.Int64
	}
	var exp int64
	if u.ExpiresAt.Valid {
		exp = u.ExpiresAt.Int64
	}
	var lim int64
	if u.TrafficLimit.Valid {
		lim = u.TrafficLimit.Int64
	}
	var onlineAt int64
	if u.OnlineAt.Valid {
		onlineAt = u.OnlineAt.Int64
	}
	return AdminUserDetail{
		ID:           u.ID,
		Name:         u.Name,
		Status:       st,
		RawStatus:    u.Status,
		TariffName:   tariffName,
		ExpiresAt:    exp,
		TrafficUsed:  u.UsedUp + u.UsedDown,
		TrafficLimit: lim,
		DeviceCount:  devCount,
		DeviceLimit:  devLimit,
		SubURL:       l.SubURL(ctx, u),
		OnlineAt:     onlineAt,
		CreatedAt:    u.CreatedAt,
	}, nil
}

func (l *Local) AdminCreateUser(ctx context.Context, name string) (AdminUserDetail, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = fmt.Sprintf("user_%d", l.Now().Unix()%1000000)
	}
	tariffs, err := l.Store.Q.ListTariffs(ctx)
	if err != nil {
		return AdminUserDetail{}, err
	}
	if len(tariffs) == 0 {
		return AdminUserDetail{}, errors.New("no tariffs available")
	}
	u, err := l.Users.Create(ctx, domain.CreateInput{
		Name:     name,
		TariffID: tariffs[0].ID,
		Source:   domain.UserFromAdmin,
	})
	if err != nil {
		return AdminUserDetail{}, err
	}
	return l.AdminGetUser(ctx, u.ID)
}

func (l *Local) AdminExtendUser(ctx context.Context, userID int64, days int64) error {
	_, err := l.Users.Extend(ctx, userID, days)
	return err
}

func (l *Local) AdminResetTraffic(ctx context.Context, userID int64) error {
	_, err := l.Users.ResetTraffic(ctx, userID)
	return err
}

func (l *Local) AdminToggleUser(ctx context.Context, userID int64) (string, error) {
	u, err := l.Users.Get(ctx, userID)
	if err != nil {
		return "", err
	}
	nextDisabled := u.Status != "disabled"
	_, err = l.Users.Update(ctx, userID, domain.Patch{Disabled: &nextDisabled})
	next := "active"
	if nextDisabled {
		next = "disabled"
	}
	return next, err
}

func (l *Local) AdminDeleteDevice(ctx context.Context, userID, deviceID int64) error {
	return l.Binder.Unbind(ctx, userID, deviceID, false)
}

func (l *Local) AdminListNodes(ctx context.Context) ([]AdminNodeSummary, error) {
	nodes, err := l.Store.Q.ListNodes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]AdminNodeSummary, 0, len(nodes))
	for _, n := range nodes {
		var hv nodesync.HealthView
		var ok bool
		if l.Nodes != nil {
			hv, ok = l.Nodes.Health(n.ID)
		}
		s := AdminNodeSummary{
			ID:          n.ID,
			Name:        n.Name,
			Address:     n.Address,
			PublicHost:  n.PublicHost,
			Enabled:     n.Enabled != 0,
			Online:      ok && hv.OK,
			Error:       hv.Error,
			Version:     hv.Health.Version,
			ActiveConns: hv.Health.Conns,
			CPUPercent:  hv.Health.System.CPUPercent,
			MemUsed:     hv.Health.System.MemUsed,
			MemTotal:    hv.Health.System.MemTotal,
			NetRxBps:    hv.Health.System.NetRxBps,
			NetTxBps:    hv.Health.System.NetTxBps,
		}
		out = append(out, s)
	}
	return out, nil
}

