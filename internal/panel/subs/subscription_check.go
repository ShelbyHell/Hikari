package subs

import (
	"context"
	"errors"
	"fmt"

	"mikan/internal/panel/domain"
	"mikan/internal/panel/store/db"
)

const (
	reasonNodeExcluded      = "node_excluded"
	reasonInboundDisabled   = "inbound_disabled"
	reasonNotAllowed        = "not_allowed"
	reasonInvalidConfig     = "invalid_config"
	reasonUnsupportedApp    = "unsupported_app"
	reasonCoreUnknown       = "core_unknown"
	reasonCoreTooOld        = "core_too_old"
	reasonSharedNeedsAccess = "shared_needs_access"
	reasonRenderFailed      = "render_failed"
	reasonNoShareLink       = "no_share_link"
	reasonTemplateName      = "template_name_conflict"
)

// SubscriptionCheck describes a profile without returning a subscription URL, keys or configs.
type SubscriptionCheck struct {
	State            string                  `json:"state" enum:"active,expiring,limited,expired,disabled"`
	Nodes            []SubscriptionCheckNode `json:"nodes"`
	ExternalIncluded int                     `json:"external_included" doc:"Сколько сохранённых внешних серверов из тарифа попадёт в профиль, отдельно от нод панели"`
	TemplateFallback bool                    `json:"template_fallback,omitempty" doc:"Пользователь получит встроенный профиль вместо своего шаблона Clash"`
}

type SubscriptionCheckNode struct {
	ID       int64                      `json:"id"`
	Name     string                     `json:"name"`
	Reason   string                     `json:"reason,omitempty" enum:"node_disabled,node_excluded,no_inbounds,no_matching_inbounds" doc:"Почему нода не попала в профиль; не задано, если попало хотя бы одно её подключение"`
	Inbounds []SubscriptionCheckInbound `json:"inbounds"`
}

type SubscriptionCheckInbound struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Protocol      string `json:"protocol,omitempty"`
	ProfileName   string `json:"profile_name,omitempty" doc:"Название подключения в приложении, если оно попало в профиль"`
	Reason        string `json:"reason,omitempty" enum:"node_disabled,node_excluded,inbound_disabled,not_allowed,pool_excluded,pool_exhausted,invalid_config,unsupported_app,core_unknown,core_too_old,shared_needs_access,render_failed,no_share_link,template_name_conflict"`
	MinimumMihomo string `json:"minimum_mihomo,omitempty" doc:"Минимальная версия mihomo для этого подключения, если причина исключения — версия ядра"`
}

// CheckSubscription checks what a Happ or Clash fetch would contain if the app gets a
// profile. It does not go through the public handler: no slot lookup, device binding,
// fetch accounting or delivery of credentials.
func (h *Handler) CheckSubscription(ctx context.Context, u db.User, app App, cfg Config) (SubscriptionCheck, error) {
	now := h.now()
	grants, err := domain.UserGrantsLeft(ctx, h.st.Q, u.ID, now)
	if err != nil {
		return SubscriptionCheck{}, err
	}
	nodes, err := h.st.Q.ListNodes(ctx)
	if err != nil {
		return SubscriptionCheck{}, err
	}
	prof, ins, err := h.profileInputs(ctx, u, cfg, grants)
	if err != nil {
		return SubscriptionCheck{}, err
	}
	return checkProfile(prof, ins, nodes, domain.State(u, grants.Main(u.ID), now), app, cfg.Template)
}

func checkProfile(prof Profile, ins profileInbounds, nodes []db.Node, state string, app App, template string) (SubscriptionCheck, error) {
	out := SubscriptionCheck{State: state, Nodes: make([]SubscriptionCheckNode, 0, len(nodes))}
	byNode := make(map[int64][]*SubscriptionCheckInbound)
	byID := make(map[int64]*SubscriptionCheckInbound, len(ins.all))
	// The report is built from the base eligibility set below; do not retain an
	// inbound slice supplied by a caller that reuses a Profile value.
	prof.Inbounds = nil
	prof.Slot, prof.App = previewSlot, app
	active := domain.CanConnect(state)
	for _, in := range ins.all {
		row := &SubscriptionCheckInbound{ID: in.ID, Name: in.Name, Reason: ins.exclusion(in)}
		if t, err := parseTemplate(in.Config); err == nil {
			row.Protocol = t.Type()
		}
		if row.Reason == "" {
			var need Version
			row.Reason, need = appExclusion(in, app, active)
			if need.Known() {
				row.MinimumMihomo = fmt.Sprintf("%d.%d.%d", need[0], need[1], need[2])
			}
		}
		if row.Reason == "" {
			prof.Inbounds = append(prof.Inbounds, in)
		}
		byNode[in.NodeID] = append(byNode[in.NodeID], row)
		byID[in.ID] = row
	}
	prof.Skip = func(in db.Inbound, _ error) { byID[in.ID].Reason = reasonRenderFailed }
	ps, err := build(prof)
	if err != nil {
		return out, err
	}
	if len(ps) > 0 && prof.usesTemplate(template) {
		_, kept, err := templateConfigOf(prof, template, ps)
		var te *TemplateError
		switch {
		case err == nil:
			// A proxy already in the template takes precedence over a generated one
			// of the same name. Mark the generated proxies until we see which remain.
			for _, p := range ps {
				if row := byID[p.inbound]; row != nil {
					row.Reason = reasonTemplateName
				}
			}
			ps = kept
		case errors.As(err, &te):
			out.TemplateFallback = true // clashProfile serves the built-in profile too
		default:
			return out, err
		}
	}
	for _, p := range ps {
		row := byID[p.inbound]
		if app.Family == FamilyXray && p.uri == "" {
			if row != nil {
				row.Reason = reasonNoShareLink
			}
			continue
		}
		if row == nil {
			out.ExternalIncluded++
		} else {
			row.ProfileName, row.Reason = p.name, ""
		}
	}
	for _, n := range nodes {
		rows := byNode[n.ID]
		v := SubscriptionCheckNode{ID: n.ID, Name: n.Name, Inbounds: make([]SubscriptionCheckInbound, 0, len(rows))}
		included := false
		for _, row := range rows {
			if n.Enabled == 0 {
				row.Reason = "node_disabled"
				row.ProfileName = ""
			}
			v.Inbounds = append(v.Inbounds, *row)
			included = included || row.Reason == ""
		}
		switch {
		case !ins.nodes[n.ID] && n.Enabled == 0:
			v.Reason = "node_disabled"
		case !ins.nodes[n.ID]:
			v.Reason = reasonNodeExcluded
		case len(rows) == 0:
			v.Reason = "no_inbounds"
		case !included:
			v.Reason = "no_matching_inbounds"
		}
		out.Nodes = append(out.Nodes, v)
	}
	return out, nil
}
