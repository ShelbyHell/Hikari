package app

import (
	"context"
	"time"

	"mikan/internal/panel/external"
	"mikan/internal/panel/store/db"
)

const (
	// externalRefreshInterval is how long a feed goes unpulled when the source names no
	// interval of its own: apps fetch subscriptions hourly too, so a change upstream reaches
	// every client within about two hours.
	externalRefreshInterval = time.Hour
)

// externalRefresh pulls every enabled upstream subscription that is due: first pass
// right away, then every minute over the sources' own intervals. A feed that fails keeps
// its old servers: clients never see a hole because an upstream had a bad minute.
func (p *Panel) externalRefresh(ctx context.Context) {
	p.refreshExternal(ctx)
	every(ctx, time.Minute, func() { p.refreshExternal(ctx) })
}

func (p *Panel) refreshExternal(ctx context.Context) {
	subs, err := p.st.Q.ListAllExternalSubscriptions(ctx)
	if err != nil {
		p.log.Error("external subscriptions", "err", err)
		return
	}
	// The fetcher is made once for the pass: it holds no state worth keeping.
	fetcher := external.NewFetcher(external.FetchTimeout)
	now := p.now().Unix()
	for _, e := range subs {
		interval := externalRefreshInterval
		if e.IntervalSec > 0 {
			interval = time.Duration(e.IntervalSec) * time.Second
		}
		if !e.Enabled || now-e.LastFetch < int64(interval/time.Second) {
			continue
		}
		p.refreshExternalSub(ctx, fetcher, e)
	}
}

func (p *Panel) refreshExternalSub(ctx context.Context, fetcher *external.Fetcher, e db.ExternalSubscription) {
	res, err := fetcher.Fetch(ctx, e.Url)
	if err != nil {
		p.log.Warn("external subscription", "name", e.Name, "err", err)
		if uerr := p.st.Q.SetExternalSubscriptionFetch(ctx, db.SetExternalSubscriptionFetchParams{
			ID:        e.ID,
			LastFetch: p.now().Unix(),
			LastError: err.Error(),
			LastCount: 0,
		}); uerr != nil {
			p.log.Error("external subscription state", "name", e.Name, "err", uerr)
		}
		return
	}
	if err := p.replaceExternalServers(ctx, e.ID, res.Servers); err != nil {
		p.log.Error("external subscription", "name", e.Name, "err", err)
		return
	}
	p.log.Info("external subscription refreshed", "name", e.Name, "servers", len(res.Servers))
}

// replaceExternalServers writes what the feed holds: each server a row of its own.
func (p *Panel) replaceExternalServers(ctx context.Context, id int64, servers []external.Server) error {
	if err := p.st.Q.ReplaceExternalServers(ctx, id); err != nil {
		return err
	}
	for _, s := range servers {
		mihomo := external.MihomoJSON(s)
		if _, err := p.st.Q.CreateExternalServer(ctx, db.CreateExternalServerParams{
			ExternalSubID: id,
			Name:          s.Name,
			Link:          s.Link,
			Enabled:       s.Enabled,
			Mihomo:        mihomo,
			CreatedAt:     p.now().Unix(),
		}); err != nil {
			return err
		}
	}
	return p.st.Q.SetExternalSubscriptionFetch(ctx, db.SetExternalSubscriptionFetchParams{
		ID:        id,
		LastFetch: p.now().Unix(),
		LastError: "",
		LastCount: int64(len(servers)),
	})
}
