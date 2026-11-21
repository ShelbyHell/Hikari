package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"mikan/internal/panel/auth"
	"mikan/internal/panel/secure"
	"mikan/internal/panel/settings"
	"mikan/internal/panel/store/db"
	"mikan/internal/panel/tgate"
)

type ghOAuthState struct {
	ExpiresAt time.Time
	IsBind    bool
	AdminID   int64
}

type AuthProvidersView struct {
	Telegram TelegramProviderView `json:"telegram"`
	GitHub   GitHubProviderView   `json:"github"`
}

type TelegramProviderView struct {
	Enabled     bool   `json:"enabled"`
	BotUsername string `json:"bot_username,omitempty"`
}

type GitHubProviderView struct {
	Enabled bool `json:"enabled"`
}

type authProvidersOutput struct {
	Body AuthProvidersView
}

type ExternalAuthSettingsView struct {
	Telegram TelegramAuthSettings `json:"telegram"`
	GitHub   GitHubAuthSettings   `json:"github"`
}

type TelegramAuthSettings struct {
	Enabled       bool   `json:"enabled"`
	AllowedIDs    string `json:"allowed_ids"`
	BotConfigured bool   `json:"bot_configured"`
	BotUsername   string `json:"bot_username,omitempty"`
}

type GitHubAuthSettings struct {
	Enabled         bool   `json:"enabled"`
	ClientID        string `json:"client_id"`
	ClientSecretSet bool   `json:"client_secret_set"`
	AllowedUsers    string `json:"allowed_users"`
	AllowedOrgs     string `json:"allowed_orgs"`
	CallbackURL     string `json:"callback_url"`
}

type externalAuthSettingsOutput struct {
	Body ExternalAuthSettingsView
}

type patchExternalAuthInput struct {
	Body struct {
		Telegram *struct {
			Enabled    *bool   `json:"enabled,omitempty"`
			AllowedIDs *string `json:"allowed_ids,omitempty"`
		} `json:"telegram,omitempty"`
		GitHub *struct {
			Enabled      *bool   `json:"enabled,omitempty"`
			ClientID     *string `json:"client_id,omitempty"`
			ClientSecret *string `json:"client_secret,omitempty"`
			AllowedUsers *string `json:"allowed_users,omitempty"`
			AllowedOrgs  *string `json:"allowed_orgs,omitempty"`
		} `json:"github,omitempty"`
	}
}

type telegramAuthRequestOutput struct {
	Body struct {
		Token       string `json:"token"`
		Link        string `json:"link"`
		BotUsername string `json:"bot_username"`
		ExpiresIn   int    `json:"expires_in"`
	}
}

type telegramCheckInput struct {
	Body struct {
		Token string `json:"token" minLength:"1" maxLength:"128"`
	}
}

type TelegramCheckBody struct {
	Status    string     `json:"status" enum:"pending,approved,rejected,expired"`
	Admin     *AdminView `json:"admin,omitempty"`
	CSRFToken string     `json:"csrf_token,omitempty"`
}

type telegramCheckOutput struct {
	SetCookie http.Cookie `header:"Set-Cookie"`
	Body      TelegramCheckBody
}

type telegramWidgetInput struct {
	Body struct {
		ID        int64  `json:"id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name,omitempty"`
		Username  string `json:"username,omitempty"`
		PhotoURL  string `json:"photo_url,omitempty"`
		AuthDate  int64  `json:"auth_date"`
		Hash      string `json:"hash" minLength:"1" maxLength:"128"`
	}
}

type telegramBindRequestOutput struct {
	Body struct {
		Token       string `json:"token"`
		Link        string `json:"link"`
		BotUsername string `json:"bot_username"`
		ExpiresIn   int    `json:"expires_in"`
	}
}

type telegramBindCheckInput struct {
	Body struct {
		Token string `json:"token" minLength:"1" maxLength:"128"`
	}
}

type TelegramBindCheckBody struct {
	Status           string `json:"status" enum:"pending,bound,expired"`
	TelegramID       int64  `json:"telegram_id,omitempty"`
	TelegramUsername string `json:"telegram_username,omitempty"`
}

type telegramBindCheckOutput struct {
	Body TelegramBindCheckBody
}

type githubStartInput struct {
	Next   string `query:"next" maxLength:"256"`
	Bind   bool   `query:"bind"`
	Cookie string `header:"Cookie"`
}

type githubStartOutput struct {
	Location  string      `header:"Location"`
	SetCookie http.Cookie `header:"Set-Cookie"`
}

type githubCallbackInput struct {
	Code   string `query:"code"`
	State  string `query:"state"`
	Error  string `query:"error"`
	Cookie string `header:"Cookie"`
}

type githubCallbackOutput struct {
	Location  string      `header:"Location"`
	SetCookie http.Cookie `header:"Set-Cookie"`
}

func (h *handlers) registerAuthExternal() {
	public := map[string]any{"public": true}
	sessionOnly := map[string]any{"sessionOnly": true}
	sessionOnlyExt := map[string]any{"x-session-only": true}

	huma.Register(h.api, huma.Operation{
		OperationID: "auth-providers",
		Method:      http.MethodGet,
		Path:        "/api/v1/auth/providers",
		Summary:     "Доступные способы входа",
		Tags:        []string{"auth"},
		Metadata:    public,
		Security:    []map[string][]string{{}},
	}, h.authProviders)

	huma.Register(h.api, huma.Operation{
		OperationID: "get-external-auth-settings",
		Method:      http.MethodGet,
		Path:        "/api/v1/auth/external/settings",
		Summary:     "Настройки внешних способов входа",
		Tags:        []string{"auth"},
		Metadata:    sessionOnly,
		Extensions:  sessionOnlyExt,
	}, h.getExternalAuthSettings)

	huma.Register(h.api, huma.Operation{
		OperationID: "update-external-auth-settings",
		Method:      http.MethodPatch,
		Path:        "/api/v1/auth/external/settings",
		Summary:     "Изменить настройки внешних способов входа",
		Tags:        []string{"auth"},
		Metadata:    sessionOnly,
		Extensions:  sessionOnlyExt,
	}, h.updateExternalAuthSettings)

	huma.Register(h.api, huma.Operation{
		OperationID: "telegram-auth-request",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/telegram/request",
		Summary:     "Запросить вход через Telegram-бота",
		Tags:        []string{"auth"},
		Metadata:    public,
		Security:    []map[string][]string{{}},
	}, h.telegramAuthRequest)

	huma.Register(h.api, huma.Operation{
		OperationID: "telegram-auth-check",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/telegram/check",
		Summary:     "Проверить статус подтверждения входа через Telegram-бота",
		Tags:        []string{"auth"},
		Metadata:    public,
		Security:    []map[string][]string{{}},
	}, h.telegramAuthCheck)

	huma.Register(h.api, huma.Operation{
		OperationID: "telegram-auth-widget",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/telegram/widget",
		Summary:     "Вход через Telegram Login Widget",
		Tags:        []string{"auth"},
		Metadata:    public,
		Security:    []map[string][]string{{}},
	}, h.telegramAuthWidget)

	huma.Register(h.api, huma.Operation{
		OperationID: "telegram-bind-request",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/telegram/bind-request",
		Summary:     "Создать запрос на привязку Telegram-аккаунта",
		Tags:        []string{"auth"},
		Metadata:    sessionOnly,
		Extensions:  sessionOnlyExt,
	}, h.telegramBindRequest)

	huma.Register(h.api, huma.Operation{
		OperationID: "telegram-bind-check",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/telegram/bind-check",
		Summary:     "Проверить статус привязки Telegram-аккаунта",
		Tags:        []string{"auth"},
		Metadata:    sessionOnly,
		Extensions:  sessionOnlyExt,
	}, h.telegramBindCheck)

	huma.Register(h.api, huma.Operation{
		OperationID:   "github-auth-start",
		Method:        http.MethodGet,
		Path:          "/api/v1/auth/github/start",
		Summary:       "Начать вход или привязку через GitHub OAuth",
		Tags:          []string{"auth"},
		Metadata:      public,
		Security:      []map[string][]string{{}},
		DefaultStatus: http.StatusFound,
	}, h.githubAuthStart)

	huma.Register(h.api, huma.Operation{
		OperationID:   "github-auth-callback",
		Method:        http.MethodGet,
		Path:          "/api/v1/auth/github/callback",
		Summary:       "Обработка перенаправления от GitHub OAuth",
		Tags:          []string{"auth"},
		Metadata:      public,
		Security:      []map[string][]string{{}},
		DefaultStatus: http.StatusFound,
	}, h.githubAuthCallback)
}

func (h *handlers) authProviders(ctx context.Context, _ *struct{}) (*authProvidersOutput, error) {
	tgEnabled, _, _ := settings.Get[bool](ctx, h.d.Settings, settings.KeyAuthTelegramEnabled)
	ghEnabled, _, _ := settings.Get[bool](ctx, h.d.Settings, settings.KeyAuthGitHubEnabled)

	ghClientID, _ := h.d.Settings.String(ctx, settings.KeyAuthGitHubClientID)
	ghClientSecret, _ := h.d.Settings.String(ctx, settings.KeyAuthGitHubClientSecret)
	gitHubReady := ghEnabled && ghClientID != "" && ghClientSecret != ""

	botUsername := ""
	if h.d.Gateway != nil {
		botUsername = h.d.Gateway.Status().Bot.Username
	}
	if botUsername == "" {
		if cachedBot, ok, _ := settings.Get[tgate.User](ctx, h.d.Settings, tgate.KeyBot); ok && cachedBot.Username != "" {
			botUsername = cachedBot.Username
		}
	}
	telegramReady := tgEnabled && botUsername != ""

	return &authProvidersOutput{
		Body: AuthProvidersView{
			Telegram: TelegramProviderView{
				Enabled:     telegramReady,
				BotUsername: botUsername,
			},
			GitHub: GitHubProviderView{
				Enabled: gitHubReady,
			},
		},
	}, nil
}

func (h *handlers) getExternalAuthSettings(ctx context.Context, _ *struct{}) (*externalAuthSettingsOutput, error) {
	tgEnabled, _, _ := settings.Get[bool](ctx, h.d.Settings, settings.KeyAuthTelegramEnabled)
	tgAllowedIDs, _ := h.d.Settings.String(ctx, settings.KeyAuthTelegramIDs)

	ghEnabled, _, _ := settings.Get[bool](ctx, h.d.Settings, settings.KeyAuthGitHubEnabled)
	ghClientID, _ := h.d.Settings.String(ctx, settings.KeyAuthGitHubClientID)
	ghClientSecret, _ := h.d.Settings.String(ctx, settings.KeyAuthGitHubClientSecret)
	ghUsers, _ := h.d.Settings.String(ctx, settings.KeyAuthGitHubUsers)
	ghOrgs, _ := h.d.Settings.String(ctx, settings.KeyAuthGitHubOrgs)

	botToken, _ := h.d.Settings.String(ctx, tgate.KeyToken)
	botConfigured := botToken != ""
	botUsername := ""
	if h.d.Gateway != nil {
		botUsername = h.d.Gateway.Status().Bot.Username
	}
	if botUsername == "" {
		if cachedBot, ok, _ := settings.Get[tgate.User](ctx, h.d.Settings, tgate.KeyBot); ok && cachedBot.Username != "" {
			botUsername = cachedBot.Username
		}
	}

	callbackURL := ""
	ep, err := h.d.Settings.Endpoint(ctx)
	if err == nil && ep.URL() != "" {
		callbackURL = ep.URL() + "/api/v1/auth/github/callback"
	}

	return &externalAuthSettingsOutput{
		Body: ExternalAuthSettingsView{
			Telegram: TelegramAuthSettings{
				Enabled:       tgEnabled,
				AllowedIDs:    tgAllowedIDs,
				BotConfigured: botConfigured,
				BotUsername:   botUsername,
			},
			GitHub: GitHubAuthSettings{
				Enabled:         ghEnabled,
				ClientID:        ghClientID,
				ClientSecretSet: ghClientSecret != "",
				AllowedUsers:    ghUsers,
				AllowedOrgs:     ghOrgs,
				CallbackURL:     callbackURL,
			},
		},
	}, nil
}

func (h *handlers) updateExternalAuthSettings(ctx context.Context, in *patchExternalAuthInput) (*externalAuthSettingsOutput, error) {
	if in.Body.Telegram != nil {
		if in.Body.Telegram.Enabled != nil {
			if err := settings.Set(ctx, h.d.Settings, settings.KeyAuthTelegramEnabled, *in.Body.Telegram.Enabled); err != nil {
				return nil, err
			}
		}
		if in.Body.Telegram.AllowedIDs != nil {
			if err := settings.Set(ctx, h.d.Settings, settings.KeyAuthTelegramIDs, strings.TrimSpace(*in.Body.Telegram.AllowedIDs)); err != nil {
				return nil, err
			}
		}
	}

	if in.Body.GitHub != nil {
		if in.Body.GitHub.Enabled != nil {
			if err := settings.Set(ctx, h.d.Settings, settings.KeyAuthGitHubEnabled, *in.Body.GitHub.Enabled); err != nil {
				return nil, err
			}
		}
		if in.Body.GitHub.ClientID != nil {
			if err := settings.Set(ctx, h.d.Settings, settings.KeyAuthGitHubClientID, strings.TrimSpace(*in.Body.GitHub.ClientID)); err != nil {
				return nil, err
			}
		}
		if in.Body.GitHub.ClientSecret != nil && strings.TrimSpace(*in.Body.GitHub.ClientSecret) != "" {
			if err := settings.Set(ctx, h.d.Settings, settings.KeyAuthGitHubClientSecret, strings.TrimSpace(*in.Body.GitHub.ClientSecret)); err != nil {
				return nil, err
			}
		}
		if in.Body.GitHub.AllowedUsers != nil {
			if err := settings.Set(ctx, h.d.Settings, settings.KeyAuthGitHubUsers, strings.TrimSpace(*in.Body.GitHub.AllowedUsers)); err != nil {
				return nil, err
			}
		}
		if in.Body.GitHub.AllowedOrgs != nil {
			if err := settings.Set(ctx, h.d.Settings, settings.KeyAuthGitHubOrgs, strings.TrimSpace(*in.Body.GitHub.AllowedOrgs)); err != nil {
				return nil, err
			}
		}
	}

	h.audit(ctx, sessionOf(ctx).AdminID, "auth.external_settings_updated", "settings", "external_auth", nil)
	return h.getExternalAuthSettings(ctx, nil)
}

func (h *handlers) telegramAuthRequest(ctx context.Context, _ *struct{}) (*telegramAuthRequestOutput, error) {
	c := clientOf(ctx)
	ipKey := "ip_tg_req:" + auth.ClientKey(c.IP)
	now := h.d.Now()
	okIP, waitIP, _ := h.d.IPLimit.Reserve(ipKey, now)
	if !okIP {
		return nil, tooManyAttempts(waitIP)
	}

	tgEnabled, _, _ := settings.Get[bool](ctx, h.d.Settings, settings.KeyAuthTelegramEnabled)
	if !tgEnabled {
		return nil, huma.Error400BadRequest("telegram_auth_disabled")
	}

	if h.d.Gateway == nil {
		return nil, huma.Error503ServiceUnavailable("telegram_gateway_unavailable")
	}

	req, link, err := h.d.Gateway.CreateAuthRequest(ctx, c.IP, c.UserAgent)
	if err != nil {
		return nil, huma.Error503ServiceUnavailable("bot_not_ready")
	}

	botUsername := h.d.Gateway.Status().Bot.Username
	if botUsername == "" {
		if cachedBot, ok, _ := settings.Get[tgate.User](ctx, h.d.Settings, tgate.KeyBot); ok {
			botUsername = cachedBot.Username
		}
	}

	out := &telegramAuthRequestOutput{}
	out.Body.Token = req.Token
	out.Body.Link = link
	out.Body.BotUsername = botUsername
	out.Body.ExpiresIn = int(time.Until(req.ExpiresAt).Seconds())
	if out.Body.ExpiresIn < 0 {
		out.Body.ExpiresIn = 0
	}
	return out, nil
}

func (h *handlers) telegramAuthCheck(ctx context.Context, in *telegramCheckInput) (*telegramCheckOutput, error) {
	if h.d.Gateway == nil {
		return nil, huma.Error503ServiceUnavailable("telegram_gateway_unavailable")
	}

	req, err := h.d.Gateway.CheckAuthRequest(in.Body.Token)
	if err != nil {
		return &telegramCheckOutput{
			Body: TelegramCheckBody{Status: "expired"},
		}, nil
	}

	if req.Status != "approved" {
		return &telegramCheckOutput{
			Body: TelegramCheckBody{Status: req.Status},
		}, nil
	}

	admins, err := h.d.Store.Q.ListAdmins(ctx)
	if err != nil || len(admins) == 0 {
		return nil, huma.Error500InternalServerError("admin_not_found")
	}
	admin := admins[0]

	c := clientOf(ctx)
	now := h.d.Now()
	identity := ""
	if req.TelegramUser != nil {
		if req.TelegramUser.Username != "" {
			identity = "@" + req.TelegramUser.Username
		} else {
			identity = strconv.FormatInt(req.TelegramUser.ID, 10)
		}
	}

	token, sess, err := h.d.Sessions.Create(ctx, admin.ID, c.IP, c.UserAgent, "telegram", identity)
	if err != nil {
		return nil, err
	}

	_ = h.d.Store.Q.SetAdminLastLogin(ctx, db.SetAdminLastLoginParams{
		LastLoginAt: sql.NullInt64{Int64: now.Unix(), Valid: true},
		ID:          admin.ID,
	})

	var tgDetails map[string]any
	if req.TelegramUser != nil {
		tgDetails = map[string]any{
			"telegram_id":       req.TelegramUser.ID,
			"telegram_username": req.TelegramUser.Username,
			"telegram_name":     req.TelegramUser.FirstName,
		}
	}
	h.audit(ctx, admin.ID, "auth.login_telegram", "admin", admin.Username, tgDetails)

	if h.d.Gateway != nil {
		h.d.Gateway.NotifyLogin(ctx, sess.IDHash, "telegram", identity, c.IP, c.UserAgent)
	}

	v := viewAdmin(admin)
	return &telegramCheckOutput{
		SetCookie: sessionCookie(token, auth.MaxTTL),
		Body: TelegramCheckBody{
			Status:    "approved",
			Admin:     &v,
			CSRFToken: sess.CsrfToken,
		},
	}, nil
}

func (h *handlers) telegramAuthWidget(ctx context.Context, in *telegramWidgetInput) (*loginOutput, error) {
	c := clientOf(ctx)
	now := h.d.Now()
	ipKey := "ip_tg_widget:" + auth.ClientKey(c.IP)
	okIP, waitIP, _ := h.d.IPLimit.Reserve(ipKey, now)
	if !okIP {
		return nil, tooManyAttempts(waitIP)
	}

	tgEnabled, _, _ := settings.Get[bool](ctx, h.d.Settings, settings.KeyAuthTelegramEnabled)
	if !tgEnabled {
		return nil, huma.Error400BadRequest("telegram_auth_disabled")
	}

	botToken, err := h.d.Settings.String(ctx, tgate.KeyToken)
	if err != nil || botToken == "" {
		return nil, huma.Error503ServiceUnavailable("telegram_bot_not_configured")
	}

	params := map[string]string{
		"id":         strconv.FormatInt(in.Body.ID, 10),
		"first_name": in.Body.FirstName,
		"last_name":  in.Body.LastName,
		"username":   in.Body.Username,
		"photo_url":  in.Body.PhotoURL,
		"auth_date":  strconv.FormatInt(in.Body.AuthDate, 10),
		"hash":       in.Body.Hash,
	}

	user, err := tgate.CheckTelegramWidget(botToken, params, now)
	if err != nil {
		h.audit(ctx, 0, "auth.telegram_widget_failed", "admin", "unknown", map[string]any{"reason": "bad_signature"})
		return nil, huma.Error401Unauthorized("invalid_telegram_signature")
	}

	if h.d.Gateway == nil || !h.d.Gateway.IsAuthAdmin(ctx, user.ID, user.Username) {
		h.audit(ctx, 0, "auth.telegram_widget_refused", "admin", user.Username, map[string]any{
			"reason":      "unauthorized_telegram_account",
			"telegram_id": user.ID,
		})
		return nil, huma.Error401Unauthorized("unauthorized_telegram_account")
	}

	admins, err := h.d.Store.Q.ListAdmins(ctx)
	if err != nil || len(admins) == 0 {
		return nil, huma.Error500InternalServerError("admin_not_found")
	}
	admin := admins[0]

	identity := ""
	if user.Username != "" {
		identity = "@" + user.Username
	} else {
		identity = strconv.FormatInt(user.ID, 10)
	}

	token, sess, err := h.d.Sessions.Create(ctx, admin.ID, c.IP, c.UserAgent, "telegram", identity)
	if err != nil {
		return nil, err
	}

	_ = h.d.Store.Q.SetAdminLastLogin(ctx, db.SetAdminLastLoginParams{
		LastLoginAt: sql.NullInt64{Int64: now.Unix(), Valid: true},
		ID:          admin.ID,
	})

	h.audit(ctx, admin.ID, "auth.login_telegram_widget", "admin", admin.Username, map[string]any{
		"telegram_id":       user.ID,
		"telegram_username": user.Username,
	})

	if h.d.Gateway != nil {
		h.d.Gateway.NotifyLogin(ctx, sess.IDHash, "telegram", identity, c.IP, c.UserAgent)
	}

	return &loginOutput{
		SetCookie: sessionCookie(token, auth.MaxTTL),
		Body:      MeBody{Admin: viewAdmin(admin), CSRFToken: sess.CsrfToken},
	}, nil
}

func (h *handlers) telegramBindRequest(ctx context.Context, _ *struct{}) (*telegramBindRequestOutput, error) {
	if h.d.Gateway == nil {
		return nil, huma.Error503ServiceUnavailable("telegram_gateway_unavailable")
	}

	adminID := sessionOf(ctx).AdminID
	req, link, err := h.d.Gateway.CreateBindRequest(ctx, adminID)
	if err != nil {
		return nil, huma.Error503ServiceUnavailable("bot_not_ready")
	}

	botUsername := h.d.Gateway.Status().Bot.Username
	if botUsername == "" {
		if cachedBot, ok, _ := settings.Get[tgate.User](ctx, h.d.Settings, tgate.KeyBot); ok {
			botUsername = cachedBot.Username
		}
	}

	out := &telegramBindRequestOutput{}
	out.Body.Token = req.Token
	out.Body.Link = link
	out.Body.BotUsername = botUsername
	out.Body.ExpiresIn = int(time.Until(req.ExpiresAt).Seconds())
	if out.Body.ExpiresIn < 0 {
		out.Body.ExpiresIn = 0
	}
	return out, nil
}

func (h *handlers) telegramBindCheck(ctx context.Context, in *telegramBindCheckInput) (*telegramBindCheckOutput, error) {
	if h.d.Gateway == nil {
		return nil, huma.Error503ServiceUnavailable("telegram_gateway_unavailable")
	}

	req, err := h.d.Gateway.CheckBindRequest(in.Body.Token)
	if err != nil {
		return &telegramBindCheckOutput{
			Body: TelegramBindCheckBody{Status: "expired"},
		}, nil
	}

	out := &telegramBindCheckOutput{
		Body: TelegramBindCheckBody{Status: req.Status},
	}
	if req.Status == "bound" && req.BoundUser != nil {
		out.Body.TelegramID = req.BoundUser.ID
		out.Body.TelegramUsername = req.BoundUser.Username
		h.audit(ctx, sessionOf(ctx).AdminID, "auth.telegram_bind", "admin", req.BoundUser.Username, map[string]any{
			"telegram_id": req.BoundUser.ID,
		})
	}
	return out, nil
}

func (h *handlers) githubAuthStart(ctx context.Context, in *githubStartInput) (*githubStartOutput, error) {
	c := clientOf(ctx)
	now := h.d.Now()
	ipKey := "ip_gh_start:" + auth.ClientKey(c.IP)
	okIP, waitIP, _ := h.d.IPLimit.Reserve(ipKey, now)
	if !okIP {
		return nil, tooManyAttempts(waitIP)
	}

	ghEnabled, _, _ := settings.Get[bool](ctx, h.d.Settings, settings.KeyAuthGitHubEnabled)
	if !ghEnabled && !in.Bind {
		return nil, huma.Error400BadRequest("github_auth_disabled")
	}

	clientID, _ := h.d.Settings.String(ctx, settings.KeyAuthGitHubClientID)
	clientSecret, _ := h.d.Settings.String(ctx, settings.KeyAuthGitHubClientSecret)
	if clientID == "" || clientSecret == "" {
		return nil, huma.Error400BadRequest("github_not_configured")
	}

	adminID := int64(0)
	if in.Bind {
		sessCookie := parseCookie(in.Cookie, auth.CookieName)
		if sessCookie == "" {
			return nil, huma.Error401Unauthorized("session_required")
		}
		sess, err := h.d.Sessions.Lookup(ctx, sessCookie)
		if err != nil {
			return nil, huma.Error401Unauthorized("session_required")
		}
		adminID = sess.AdminID
	}

	state := secure.Token(32)

	h.ghStateMu.Lock()
	for k, st := range h.ghStates {
		if now.After(st.ExpiresAt) {
			delete(h.ghStates, k)
		}
	}
	h.ghStates[state] = ghOAuthState{
		ExpiresAt: now.Add(10 * time.Minute),
		IsBind:    in.Bind,
		AdminID:   adminID,
	}
	h.ghStateMu.Unlock()

	callbackURL := ""
	ep, err := h.d.Settings.Endpoint(ctx)
	if err == nil && ep.URL() != "" {
		callbackURL = ep.URL() + "/api/v1/auth/github/callback"
	}

	authURL := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&scope=read:user,read:org&state=%s",
		url.QueryEscape(clientID), url.QueryEscape(state),
	)
	if callbackURL != "" {
		authURL += "&redirect_uri=" + url.QueryEscape(callbackURL)
	}

	cookie := http.Cookie{
		Name:     "gh_oauth_state",
		Value:    state,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	}

	return &githubStartOutput{
		Location:  authURL,
		SetCookie: cookie,
	}, nil
}

func parseCookie(rawHeader, name string) string {
	r := &http.Request{Header: http.Header{"Cookie": {rawHeader}}}
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}

type githubTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

type githubUserResponse struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
}

type githubOrgItem struct {
	Login string `json:"login"`
}

func exchangeGitHubCode(ctx context.Context, clientID, clientSecret, code, redirectURI string) (string, error) {
	reqBody, _ := json.Marshal(map[string]string{
		"client_id":     clientID,
		"client_secret": clientSecret,
		"code":          code,
		"redirect_uri":  redirectURI,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://github.com/login/oauth/access_token", bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Hikari-Panel")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var tok githubTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return "", err
	}
	if tok.Error != "" {
		return "", fmt.Errorf("github_oauth_error: %s (%s)", tok.Error, tok.ErrorDesc)
	}
	if tok.AccessToken == "" {
		return "", errors.New("missing_access_token")
	}
	return tok.AccessToken, nil
}

func fetchGitHubUser(ctx context.Context, accessToken string) (githubUserResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return githubUserResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Hikari-Panel")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return githubUserResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return githubUserResponse{}, fmt.Errorf("github api status %d", resp.StatusCode)
	}
	var u githubUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return githubUserResponse{}, err
	}
	if u.Login == "" {
		return githubUserResponse{}, errors.New("empty_github_login")
	}
	return u, nil
}

func fetchGitHubUserOrgs(ctx context.Context, accessToken string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/orgs", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Hikari-Panel")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api orgs status %d", resp.StatusCode)
	}
	var orgs []githubOrgItem
	if err := json.NewDecoder(resp.Body).Decode(&orgs); err != nil {
		return nil, err
	}
	res := make([]string, 0, len(orgs))
	for _, o := range orgs {
		if o.Login != "" {
			res = append(res, o.Login)
		}
	}
	return res, nil
}

func (h *handlers) githubAuthCallback(ctx context.Context, in *githubCallbackInput) (*githubCallbackOutput, error) {
	paths, _ := h.d.Settings.Paths(ctx)
	adminBase := "/"
	if paths.Admin != "" {
		adminBase = "/" + paths.Admin + "/"
	}
	loginURL := adminBase + "login"
	securitySettingsURL := adminBase + "settings/security"

	clearStateCookie := http.Cookie{
		Name:     "gh_oauth_state",
		Value:    "",
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		MaxAge:   -1,
	}

	cookieState := parseCookie(in.Cookie, "gh_oauth_state")
	now := h.d.Now()

	h.ghStateMu.Lock()
	stateInfo, foundInMemory := h.ghStates[in.State]
	if foundInMemory {
		delete(h.ghStates, in.State)
		if now.After(stateInfo.ExpiresAt) {
			foundInMemory = false
		}
	}
	h.ghStateMu.Unlock()

	errorDestURL := loginURL
	if stateInfo.IsBind {
		errorDestURL = securitySettingsURL
	}

	if in.Error != "" {
		return &githubCallbackOutput{
			Location:  errorDestURL + "?error=" + url.QueryEscape("github_"+in.Error),
			SetCookie: clearStateCookie,
		}, nil
	}

	stateValid := foundInMemory || (in.State != "" && secure.Equal(in.State, cookieState))
	if !stateValid || in.Code == "" {
		return &githubCallbackOutput{
			Location:  errorDestURL + "?error=oauth_state_invalid",
			SetCookie: clearStateCookie,
		}, nil
	}

	clientID, _ := h.d.Settings.String(ctx, settings.KeyAuthGitHubClientID)
	clientSecret, _ := h.d.Settings.String(ctx, settings.KeyAuthGitHubClientSecret)
	if clientID == "" || clientSecret == "" {
		return &githubCallbackOutput{
			Location:  errorDestURL + "?error=github_not_configured",
			SetCookie: clearStateCookie,
		}, nil
	}

	callbackURL := ""
	ep, err := h.d.Settings.Endpoint(ctx)
	if err == nil && ep.URL() != "" {
		callbackURL = ep.URL() + "/api/v1/auth/github/callback"
	}

	accessToken, err := exchangeGitHubCode(ctx, clientID, clientSecret, in.Code, callbackURL)
	if err != nil {
		h.d.Log.Error("github oauth exchange code", "err", err)
		return &githubCallbackOutput{
			Location:  errorDestURL + "?error=oauth_token_failed",
			SetCookie: clearStateCookie,
		}, nil
	}

	ghUser, err := fetchGitHubUser(ctx, accessToken)
	if err != nil {
		h.d.Log.Error("github oauth fetch user", "err", err)
		return &githubCallbackOutput{
			Location:  errorDestURL + "?error=oauth_fetch_user_failed",
			SetCookie: clearStateCookie,
		}, nil
	}

	// If this is a bind flow, automatically append to allowed users
	if stateInfo.IsBind {
		currentUsers, _ := h.d.Settings.String(ctx, settings.KeyAuthGitHubUsers)
		parts := strings.Split(currentUsers, ",")
		seen := false
		for _, p := range parts {
			if strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(p, "@")), strings.TrimPrefix(ghUser.Login, "@")) {
				seen = true
				break
			}
		}
		if !seen {
			var newParts []string
			for _, p := range parts {
				t := strings.TrimSpace(p)
				if t != "" {
					newParts = append(newParts, t)
				}
			}
			newParts = append(newParts, "@"+ghUser.Login)
			_ = settings.Set(ctx, h.d.Settings, settings.KeyAuthGitHubUsers, strings.Join(newParts, ", "))
		}
		h.audit(ctx, stateInfo.AdminID, "auth.github_bind", "admin", ghUser.Login, map[string]any{
			"github_login": ghUser.Login,
			"github_id":    ghUser.ID,
		})
		return &githubCallbackOutput{
			Location:  securitySettingsURL + "?github_bound=" + url.QueryEscape(ghUser.Login),
			SetCookie: clearStateCookie,
		}, nil
	}

	// Normal Login: Check allowed users
	allowedUsers, _ := h.d.Settings.String(ctx, settings.KeyAuthGitHubUsers)
	allowed := false
	allowedReason := ""
	for _, part := range strings.Split(allowedUsers, ",") {
		part = strings.TrimSpace(part)
		if part != "" && strings.EqualFold(strings.TrimPrefix(part, "@"), strings.TrimPrefix(ghUser.Login, "@")) {
			allowed = true
			allowedReason = "user"
			break
		}
	}

	// Check allowed organizations if not already allowed
	if !allowed {
		allowedOrgs, _ := h.d.Settings.String(ctx, settings.KeyAuthGitHubOrgs)
		if strings.TrimSpace(allowedOrgs) != "" {
			orgs, err := fetchGitHubUserOrgs(ctx, accessToken)
			if err == nil {
				orgParts := strings.Split(allowedOrgs, ",")
				for _, userOrg := range orgs {
					for _, allowedOrg := range orgParts {
						allowedOrg = strings.TrimSpace(allowedOrg)
						if allowedOrg != "" && strings.EqualFold(strings.TrimPrefix(allowedOrg, "@"), strings.TrimPrefix(userOrg, "@")) {
							allowed = true
							allowedReason = "org:" + userOrg
							break
						}
					}
					if allowed {
						break
					}
				}
			} else {
				h.d.Log.Warn("fetch github user orgs failed", "err", err)
			}
		}
	}

	if !allowed {
		h.audit(ctx, 0, "auth.login_github_refused", "admin", ghUser.Login, map[string]any{
			"reason":       "user_or_org_not_allowed",
			"github_login": ghUser.Login,
			"github_id":    ghUser.ID,
		})
		return &githubCallbackOutput{
			Location:  loginURL + "?error=github_user_not_allowed",
			SetCookie: clearStateCookie,
		}, nil
	}

	admins, err := h.d.Store.Q.ListAdmins(ctx)
	if err != nil || len(admins) == 0 {
		return &githubCallbackOutput{
			Location:  loginURL + "?error=admin_not_found",
			SetCookie: clearStateCookie,
		}, nil
	}
	admin := admins[0]

	c := clientOf(ctx)
	token, sess, err := h.d.Sessions.Create(ctx, admin.ID, c.IP, c.UserAgent, "github", "@"+ghUser.Login)
	if err != nil {
		return &githubCallbackOutput{
			Location:  loginURL + "?error=session_create_failed",
			SetCookie: clearStateCookie,
		}, nil
	}

	_ = h.d.Store.Q.SetAdminLastLogin(ctx, db.SetAdminLastLoginParams{
		LastLoginAt: sql.NullInt64{Int64: now.Unix(), Valid: true},
		ID:          admin.ID,
	})

	h.audit(ctx, admin.ID, "auth.login_github", "admin", admin.Username, map[string]any{
		"github_login":   ghUser.Login,
		"github_id":      ghUser.ID,
		"allowed_reason": allowedReason,
	})

	if h.d.Gateway != nil {
		h.d.Gateway.NotifyLogin(ctx, sess.IDHash, "github", "@"+ghUser.Login, c.IP, c.UserAgent)
	}

	return &githubCallbackOutput{
		Location:  adminBase,
		SetCookie: sessionCookie(token, auth.MaxTTL),
	}, nil
}
