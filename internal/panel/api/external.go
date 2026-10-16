package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"mikan/internal/panel/external"
	"mikan/internal/panel/store/db"
	"mikan/internal/panel/subs"
)

// ExternalSubView is one upstream subscription as the admin panel shows it.
type ExternalSubView struct {
	ID          int64      `json:"id"`
	TariffID    int64      `json:"tariff_id" doc:"Тариф, подписчикам которого раздаются серверы этой подписки"`
	Name        string     `json:"name"`
	URL         string     `json:"url"`
	Enabled     bool       `json:"enabled"`
	IntervalSec int64      `json:"interval_sec" doc:"Интервал автообновления, секунды; 0 — час по умолчанию, минимум 300 сек"`
	LastFetch   *time.Time `json:"last_fetch,omitempty"`
	LastError   string     `json:"last_error,omitempty"`
	LastCount   int64      `json:"last_count"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// ExternalServerView is one server of an upstream subscription.
type ExternalServerView struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Link      string    `json:"link,omitempty"`
	Enabled   bool      `json:"enabled"`
	Proto     string    `json:"proto"`
	Host      string    `json:"host"`
	CreatedAt time.Time `json:"created_at"`
}

type externalSubsOutput struct{ Body []ExternalSubView }
type externalSubOutput struct{ Body ExternalSubView }
type externalServersOutput struct{ Body []ExternalServerView }

type createExternalSubInput struct {
	Body struct {
		TariffID int64  `json:"tariff_id" minimum:"1" doc:"Тариф, к которому относится внешняя подписка"`
		Name     string `json:"name" maxLength:"200" doc:"Группа в подписке клиента"`
		URL      string `json:"url" maxLength:"1000" doc:"Ссылка на чужую подписку (https)"`
		Enabled  *bool  `json:"enabled,omitempty" doc:"Включена ли подписка; по умолчанию да"`
	}
}

type patchExternalSubInput struct {
	ID   int64 `path:"id" minimum:"1"`
	Body struct {
		Name        *string `json:"name,omitempty" maxLength:"200"`
		URL         *string `json:"url,omitempty" maxLength:"1000"`
		Enabled     *bool   `json:"enabled,omitempty"`
		IntervalSec *int64  `json:"interval_sec,omitempty" minimum:"0" maximum:"604800" doc:"Как часто обновлять подписку, секунды; 0 — час по умолчанию"`
	}
}

type patchExternalServerInput struct {
	ID       int64 `path:"id" minimum:"1"`
	ServerID int64 `path:"server_id" minimum:"1"`
	Body     struct {
		Enabled *bool `json:"enabled,omitempty"`
	}
}

type enableExternalServersInput struct {
	ID   int64 `path:"id" minimum:"1"`
	Body struct {
		Enabled bool `json:"enabled" doc:"true — включить все серверы, false — выключить"`
	}
}

func (h *handlers) registerExternalSubscriptions() {
	huma.Register(h.api, huma.Operation{OperationID: "list-external-subscriptions", Method: http.MethodGet, Path: "/api/v1/external-subscriptions", Summary: "Внешние подписки", Tags: []string{"external"}}, h.listExternalSubscriptions)
	huma.Register(h.api, huma.Operation{OperationID: "create-external-subscription", Metadata: sessionOnly, Extensions: sessionOnlyExt, Method: http.MethodPost, Path: "/api/v1/external-subscriptions", Summary: "Добавить внешнюю подписку", Tags: []string{"external"}, DefaultStatus: http.StatusCreated}, h.createExternalSubscription)
	huma.Register(h.api, huma.Operation{OperationID: "update-external-subscription", Metadata: sessionOnly, Extensions: sessionOnlyExt, Method: http.MethodPatch, Path: "/api/v1/external-subscriptions/{id}", Summary: "Изменить внешнюю подписку", Tags: []string{"external"}}, h.updateExternalSubscription)
	huma.Register(h.api, huma.Operation{OperationID: "delete-external-subscription", Metadata: sessionOnly, Extensions: sessionOnlyExt, Method: http.MethodDelete, Path: "/api/v1/external-subscriptions/{id}", Summary: "Удалить внешнюю подписку", Tags: []string{"external"}, DefaultStatus: http.StatusNoContent}, h.deleteExternalSubscription)
	huma.Register(h.api, huma.Operation{OperationID: "fetch-external-subscription", Metadata: sessionOnly, Extensions: sessionOnlyExt, Method: http.MethodPost, Path: "/api/v1/external-subscriptions/{id}/fetch", Summary: "Обновить внешнюю подписку сейчас", Tags: []string{"external"}}, h.fetchExternalSubscription)
	huma.Register(h.api, huma.Operation{OperationID: "list-external-servers", Method: http.MethodGet, Path: "/api/v1/external-subscriptions/{id}/servers", Summary: "Серверы внешней подписки", Tags: []string{"external"}}, h.listExternalServers)
	huma.Register(h.api, huma.Operation{OperationID: "update-external-server", Metadata: sessionOnly, Extensions: sessionOnlyExt, Method: http.MethodPatch, Path: "/api/v1/external-subscriptions/{id}/servers/{server_id}", Summary: "Включить или выключить сервер внешней подписки", Tags: []string{"external"}}, h.updateExternalServer)
	huma.Register(h.api, huma.Operation{OperationID: "enable-external-servers", Metadata: sessionOnly, Extensions: sessionOnlyExt, Method: http.MethodPost, Path: "/api/v1/external-subscriptions/{id}/servers/enable-all", Summary: "Включить или выключить все серверы внешней подписки", Tags: []string{"external"}}, h.enableExternalServers)
}

func externalSubView(e db.ExternalSubscription) ExternalSubView {
	v := ExternalSubView{ID: e.ID, TariffID: e.TariffID, Name: e.Name, URL: e.Url, Enabled: e.Enabled, LastError: e.LastError, LastCount: e.LastCount, IntervalSec: e.IntervalSec}
	if e.LastFetch > 0 {
		t := time.Unix(e.LastFetch, 0).UTC()
		v.LastFetch = &t
	}
	v.CreatedAt = time.Unix(e.CreatedAt, 0).UTC()
	v.UpdatedAt = time.Unix(e.UpdatedAt, 0).UTC()
	return v
}

func (h *handlers) listExternalSubscriptions(ctx context.Context, _ *struct{}) (*externalSubsOutput, error) {
	subs, err := h.d.Store.Q.ListAllExternalSubscriptions(ctx)
	if err != nil {
		return nil, err
	}
	out := &externalSubsOutput{Body: make([]ExternalSubView, 0, len(subs))}
	for _, e := range subs {
		out.Body = append(out.Body, externalSubView(e))
	}
	return out, nil
}

func (h *handlers) createExternalSubscription(ctx context.Context, in *createExternalSubInput) (*externalSubOutput, error) {
	b := in.Body
	name, link := strings.TrimSpace(b.Name), strings.TrimSpace(b.URL)
	var details []error
	if err := subs.ValidName(name); err != nil {
		details = append(details, &huma.ErrorDetail{Location: "body.name", Message: err.Error()})
	}
	if u, err := url.Parse(link); err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || len(link) > 1000 {
		details = append(details, &huma.ErrorDetail{Location: "body.url", Message: "external_url_invalid"})
	}
	if len(details) > 0 {
		return nil, huma.Error422UnprocessableEntity("validation", details...)
	}
	enabled := b.Enabled == nil || *b.Enabled
	fetcher := external.NewFetcher(external.FetchTimeout)
	res, fetchErr := fetcher.Fetch(ctx, link)
	if fetchErr != nil {
		return nil, huma.Error502BadGateway("fetch_error", &huma.ErrorDetail{Location: "body.url", Message: fetchErr.Error()})
	}
	now := h.d.Now().Unix()
	// The tariff must exist: the panel refuses a source nothing could ever serve.
	if _, err := h.d.Store.Q.GetTariff(ctx, b.TariffID); err != nil {
		return nil, fieldErr("body.tariff_id", "tariff_not_found")
	}
	e, err := h.d.Store.Q.CreateExternalSubscription(ctx, db.CreateExternalSubscriptionParams{
		TariffID:    b.TariffID,
		Name:        name,
		Url:         link,
		Enabled:     enabled,
		IntervalSec: 0,
		CreatedAt:   now,
	})
	if err != nil {
		return nil, err
	}
	if err := h.storeExternalServers(ctx, e.ID, res.Servers); err != nil {
		return nil, err
	}
	h.nodesChanged()
	h.audit(ctx, sessionOf(ctx).AdminID, "external.create", "external_subscription", strconv.FormatInt(e.ID, 10),
		map[string]any{"name": name, "servers": len(res.Servers)})
	return &externalSubOutput{Body: externalSubView(e)}, nil
}

func (h *handlers) updateExternalSubscription(ctx context.Context, in *patchExternalSubInput) (*externalSubOutput, error) {
	e, err := h.d.Store.Q.GetExternalSubscription(ctx, in.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, huma.Error404NotFound("not_found")
	}
	if err != nil {
		return nil, err
	}
	b := in.Body
	if b.Name != nil {
		name := strings.TrimSpace(*b.Name)
		if err := subs.ValidName(name); err != nil {
			return nil, huma.Error422UnprocessableEntity("validation", &huma.ErrorDetail{Location: "body.name", Message: err.Error()})
		}
		e.Name = name
	}
	if b.URL != nil {
		link := strings.TrimSpace(*b.URL)
		if u, err := url.Parse(link); err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || len(link) > 1000 {
			return nil, huma.Error422UnprocessableEntity("validation", &huma.ErrorDetail{Location: "body.url", Message: "external_url_invalid"})
		}
		e.Url = link
	}
	if b.Enabled != nil {
		e.Enabled = *b.Enabled
	}
	if b.IntervalSec != nil {
		if *b.IntervalSec != 0 && *b.IntervalSec < 300 {
			return nil, huma.Error422UnprocessableEntity("validation", &huma.ErrorDetail{Location: "body.interval_sec", Message: "external_interval"})
		}
		e.IntervalSec = *b.IntervalSec
	}
	e, err = h.d.Store.Q.UpdateExternalSubscription(ctx, db.UpdateExternalSubscriptionParams{
		ID:          e.ID,
		Name:        e.Name,
		Url:         e.Url,
		Enabled:     e.Enabled,
		IntervalSec: e.IntervalSec,
		UpdatedAt:   h.d.Now().Unix(),
	})
	if err != nil {
		return nil, err
	}
	h.nodesChanged()
	h.audit(ctx, sessionOf(ctx).AdminID, "external.update", "external_subscription", strconv.FormatInt(e.ID, 10),
		map[string]any{"name": e.Name, "enabled": e.Enabled})
	return &externalSubOutput{Body: externalSubView(e)}, nil
}

func (h *handlers) deleteExternalSubscription(ctx context.Context, in *nodeIDInput) (*struct{}, error) {
	if err := h.d.Store.Q.DeleteExternalSubscription(ctx, in.ID); err != nil {
		return nil, err
	}
	h.nodesChanged()
	h.audit(ctx, sessionOf(ctx).AdminID, "external.delete", "external_subscription", strconv.FormatInt(in.ID, 10), nil)
	return nil, nil
}

// fetchExternalSubscription pulls a feed now: the servers the feed holds replace the saved ones.
func (h *handlers) fetchExternalSubscription(ctx context.Context, in *nodeIDInput) (*externalSubOutput, error) {
	e, err := h.d.Store.Q.GetExternalSubscription(ctx, in.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, huma.Error404NotFound("not_found")
	}
	if err != nil {
		return nil, err
	}
	fetcher := external.NewFetcher(external.FetchTimeout)
	res, fetchErr := fetcher.Fetch(ctx, e.Url)
	lastErr, count := "", int64(len(res.Servers))
	if fetchErr != nil {
		lastErr = fetchErr.Error()
		if uerr := h.d.Store.Q.SetExternalSubscriptionFetch(ctx, db.SetExternalSubscriptionFetchParams{
			ID:        e.ID,
			LastFetch: h.d.Now().Unix(),
			LastError: lastErr,
			LastCount: 0,
		}); uerr != nil {
			h.d.Log.Warn("external: fetch state", "err", uerr)
		}
		return nil, huma.Error502BadGateway("fetch_error", &huma.ErrorDetail{Location: "body", Message: fetchErr.Error()})
	}
	if err := h.storeExternalServers(ctx, e.ID, res.Servers); err != nil {
		return nil, err
	}
	h.nodesChanged()
	h.audit(ctx, sessionOf(ctx).AdminID, "external.fetch", "external_subscription", strconv.FormatInt(e.ID, 10),
		map[string]any{"servers": len(res.Servers)})
	e.LastError, e.LastCount, e.LastFetch = lastErr, count, h.d.Now().Unix()
	return &externalSubOutput{Body: externalSubView(e)}, nil
}

// storeExternalServers replaces a source's servers with what the feed holds.
func (h *handlers) storeExternalServers(ctx context.Context, id int64, servers []external.Server) error {
	if err := h.d.Store.Q.ReplaceExternalServers(ctx, id); err != nil {
		return err
	}
	for _, s := range servers {
		mihomoJSON, _ := json.Marshal(s.Mihomo)
		_, err := h.d.Store.Q.CreateExternalServer(ctx, db.CreateExternalServerParams{
			ExternalSubID: id,
			Name:          s.Name,
			Link:          s.Link,
			Enabled:       s.Enabled,
			Mihomo:        string(mihomoJSON),
			CreatedAt:     h.d.Now().Unix(),
		})
		if err != nil {
			return err
		}
	}
	return h.d.Store.Q.SetExternalSubscriptionFetch(ctx, db.SetExternalSubscriptionFetchParams{
		ID:        id,
		LastFetch: h.d.Now().Unix(),
		LastError: "",
		LastCount: int64(len(servers)),
	})
}

func (h *handlers) listExternalServers(ctx context.Context, in *nodeIDInput) (*externalServersOutput, error) {
	if _, err := h.d.Store.Q.GetExternalSubscription(ctx, in.ID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("not_found")
		}
		return nil, err
	}
	rows, err := h.d.Store.Q.ListExternalServers(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	out := &externalServersOutput{Body: make([]ExternalServerView, 0, len(rows))}
	for _, r := range rows {
		var m map[string]any
		_ = json.Unmarshal([]byte(r.Mihomo), &m)
		proto, _ := m["type"].(string)
		host, _ := m["server"].(string)
		out.Body = append(out.Body, ExternalServerView{ID: r.ID, Name: r.Name, Link: r.Link, Enabled: r.Enabled,
			Proto: proto, Host: host, CreatedAt: time.Unix(r.CreatedAt, 0).UTC()})
	}
	return out, nil
}

func (h *handlers) updateExternalServer(ctx context.Context, in *patchExternalServerInput) (*struct{}, error) {
	b := in.Body
	if b.Enabled == nil {
		return nil, huma.Error422UnprocessableEntity("validation", &huma.ErrorDetail{Location: "body.enabled", Message: "enabled_required"})
	}
	if err := h.d.Store.Q.SetExternalServerEnabled(ctx, db.SetExternalServerEnabledParams{
		ExternalSubID: in.ID,
		ID:            in.ServerID,
		Enabled:       *b.Enabled,
	}); err != nil {
		return nil, err
	}
	h.nodesChanged()
	h.audit(ctx, sessionOf(ctx).AdminID, "external.server", "external_server", strconv.FormatInt(in.ServerID, 10),
		map[string]any{"enabled": *b.Enabled})
	return nil, nil
}

func (h *handlers) enableExternalServers(ctx context.Context, in *enableExternalServersInput) (*struct{}, error) {
	if err := h.d.Store.Q.SetExternalServersEnabledAll(ctx, db.SetExternalServersEnabledAllParams{
		ExternalSubID: in.ID,
		Enabled:       in.Body.Enabled,
	}); err != nil {
		return nil, err
	}
	h.nodesChanged()
	h.audit(ctx, sessionOf(ctx).AdminID, "external.enable_all", "external_subscription", strconv.FormatInt(in.ID, 10),
		map[string]any{"enabled": in.Body.Enabled})
	return nil, nil
}
