package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"mikan/internal/nodeapi"
	"mikan/internal/panel/domain"
	"mikan/internal/panel/nodesync"
	"mikan/internal/panel/store/db"
)

type CascadeHop struct {
	NodeID int64  `json:"node_id"`
	Name   string `json:"name"`
}

type ProbeView struct {
	OK        bool      `json:"ok"`
	IP        string    `json:"ip,omitempty" doc:"Адрес, который видят сайты"`
	Colo      string    `json:"colo,omitempty"`
	Error     string    `json:"error,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}

type CascadeExit struct {
	CascadeHop
	Inbounds []string   `json:"inbounds" doc:"Подключения этой ноды, которые выходят через ту ноду"`
	Relay    bool       `json:"relay" doc:"Через неё идёт и трафик, который другие ноды передают через эту"`
	Probe    *ProbeView `json:"probe,omitempty" doc:"Проверка с этой ноды: какой IP видят сайты через цепочку"`
}

// The states of a relay's listener on the node (CascadeRelay.Listener).
const (
	relayUp      = "ok"
	relayBusy    = "busy"    // another program holds the port
	relayFailed  = "failed"  // it could not start, for another reason
	relayUnknown = "unknown" // the node does not answer, or has not applied the relay yet
)

type CascadeRelay struct {
	Port       string       `json:"port"`
	Sources    []CascadeHop `json:"sources" doc:"Ноды, которые выходят через эту"`
	Outbound   string       `json:"outbound" enum:"direct,warp,node" doc:"Куда эта нода выпускает их трафик"`
	ExitNodeID *int64       `json:"exit_node_id,omitempty"`
	Listener   string       `json:"listener" enum:"ok,busy,failed,unknown" doc:"Слушает ли нода порт служебного входа: busy, если порт занят другой программой"`
	Error      string       `json:"error,omitempty" doc:"Почему служебный вход не запустился, как сказала нода"`
}

type CascadeView struct {
	Relay *CascadeRelay `json:"relay,omitempty" doc:"Служебный вход для других нод; есть, когда кто-то выходит через эту ноду"`
	Exits []CascadeExit `json:"exits" doc:"Ноды, через которые эта нода выпускает трафик"`
}

type cascadeOutput struct{ Body CascadeView }

type cascadePatchInput struct {
	ID   int64 `path:"id" minimum:"1"`
	Body struct {
		Outbound   string `json:"outbound" enum:"direct,warp,node" doc:"Куда выпускать трафик других нод, пришедший через эту"`
		ExitNodeID int64  `json:"exit_node_id,omitempty" minimum:"0"`
	}
}

func (h *handlers) registerCascade() {
	tags := []string{"node"}
	huma.Register(h.api, huma.Operation{OperationID: "get-node-cascade", Method: http.MethodGet, Path: "/api/v1/nodes/{id}/cascade", Summary: "Каскад ноды: выходы и служебный вход", Tags: tags}, h.getCascade)
	huma.Register(h.api, huma.Operation{OperationID: "update-node-cascade", Metadata: sessionOnly, Extensions: sessionOnlyExt, Method: http.MethodPatch, Path: "/api/v1/nodes/{id}/cascade", Summary: "Куда нода выпускает трафик других нод", Tags: tags}, h.patchCascade)
	huma.Register(h.api, huma.Operation{OperationID: "list-cascade-rules", Method: http.MethodGet, Path: "/api/v1/nodes/{id}/cascade/rules", Summary: "Правила каскада ноды: что на какую ноду уходит", Tags: tags}, h.listCascadeRules)
	huma.Register(h.api, huma.Operation{OperationID: "create-cascade-rule", Metadata: sessionOnly, Extensions: sessionOnlyExt, Method: http.MethodPost, Path: "/api/v1/nodes/{id}/cascade/rules", Summary: "Добавить правило каскада", Tags: tags, DefaultStatus: http.StatusCreated}, h.createCascadeRule)
	huma.Register(h.api, huma.Operation{OperationID: "update-cascade-rule", Metadata: sessionOnly, Extensions: sessionOnlyExt, Method: http.MethodPatch, Path: "/api/v1/nodes/{id}/cascade/rules/{rule_id}", Summary: "Изменить правило каскада", Tags: tags}, h.patchCascadeRule)
	huma.Register(h.api, huma.Operation{OperationID: "delete-cascade-rule", Metadata: sessionOnly, Extensions: sessionOnlyExt, Method: http.MethodDelete, Path: "/api/v1/nodes/{id}/cascade/rules/{rule_id}", Summary: "Удалить правило каскада", Tags: tags, DefaultStatus: http.StatusNoContent}, h.deleteCascadeRule)
	huma.Register(h.api, huma.Operation{OperationID: "order-cascade-rules", Metadata: sessionOnly, Extensions: sessionOnlyExt, Method: http.MethodPut, Path: "/api/v1/nodes/{id}/cascade/rules/order", Summary: "Порядок правил каскада: первое подходящее выигрывает", Tags: tags, DefaultStatus: http.StatusNoContent}, h.orderCascadeRules)
}

// cascadeRefusals are the domain's refusals of an exit the API reports by their own code.
var cascadeRefusals = []error{domain.ErrExitSelf, domain.ErrExitCycle, domain.ErrExitLong, domain.ErrExitOff, domain.ErrNoPort}

// cascadeError maps the domain's refusals to the API's codes.
func cascadeError(err error, field string) error {
	for _, e := range cascadeRefusals {
		if errors.Is(err, e) {
			return huma.Error422UnprocessableEntity("cascade", &huma.ErrorDetail{Location: "body." + field, Message: e.Error()})
		}
	}
	if errors.Is(err, domain.ErrNotFound) {
		return huma.Error422UnprocessableEntity("cascade", &huma.ErrorDetail{Location: "body." + field, Message: "exit_not_found"})
	}
	return err
}

// relayListener is the state of the relay's listener in a health check, and the node's
// words when it failed. A node that runs another port than the row's has not caught up
// with a move yet: what it says is not about this port.
func relayListener(hv nodesync.HealthView, port string) (state, why string) {
	if !hv.OK || hv.Ports[nodeapi.RelayListener] != port {
		return relayUnknown, ""
	}
	for _, l := range hv.Listeners {
		switch {
		case l.Name != nodeapi.RelayListener:
		case l.OK:
			return relayUp, ""
		case l.Busy():
			return relayBusy, l.Error
		default:
			return relayFailed, l.Error
		}
	}
	return relayUnknown, ""
}

// hostOf is what a node last said listens on its server; nil when nobody knows.
func (h *handlers) hostOf(id int64) *nodeapi.HostPorts {
	if h.d.Nodes == nil {
		return nil
	}
	if hv, ok := h.d.Nodes.Health(id); ok {
		return hv.HostPorts()
	}
	return nil
}

func (h *handlers) getCascade(ctx context.Context, in *nodeIDInput) (*cascadeOutput, error) {
	if _, err := h.getNode(ctx, in.ID); err != nil {
		return nil, err
	}
	q := h.d.Store.Q
	nodes, err := q.ListNodes(ctx)
	if err != nil {
		return nil, err
	}
	names := map[int64]string{}
	for _, n := range nodes {
		names[n.ID] = n.Name
	}
	out := &cascadeOutput{Body: CascadeView{Exits: []CascadeExit{}}}
	r, err := q.GetNodeRelay(ctx, in.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		users, err := q.ListRelayUsers(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		v := &CascadeRelay{Port: r.Port, Sources: []CascadeHop{}, Outbound: r.Outbound, Listener: relayUnknown}
		out.Body.Relay = v
		for _, u := range users {
			v.Sources = append(v.Sources, CascadeHop{NodeID: u.SrcNodeID, Name: names[u.SrcNodeID]})
		}
		if r.ExitNodeID.Valid {
			id := r.ExitNodeID.Int64
			v.Outbound, v.ExitNodeID = "node", &id
		}
		if h.d.Nodes != nil {
			if hv, ok := h.d.Nodes.Health(in.ID); ok {
				v.Listener, v.Error = relayListener(hv, r.Port)
			}
		}
	}
	ins, err := q.ListInbounds(ctx)
	if err != nil {
		return nil, err
	}
	idx := map[int64]int{}
	exit := func(id int64) *CascadeExit {
		if i, ok := idx[id]; ok {
			return &out.Body.Exits[i]
		}
		idx[id] = len(out.Body.Exits)
		out.Body.Exits = append(out.Body.Exits, CascadeExit{CascadeHop: CascadeHop{NodeID: id, Name: names[id]}, Inbounds: []string{}})
		return &out.Body.Exits[len(out.Body.Exits)-1]
	}
	for _, i := range ins {
		if i.NodeID == in.ID && i.ExitNodeID.Valid {
			e := exit(i.ExitNodeID.Int64)
			e.Inbounds = append(e.Inbounds, i.Name)
		}
	}
	if r.ExitNodeID.Valid {
		exit(r.ExitNodeID.Int64).Relay = true
	}
	if h.d.Nodes != nil {
		for i := range out.Body.Exits {
			e := &out.Body.Exits[i]
			if p, err := h.d.Nodes.Probe(ctx, in.ID, nodeapi.ExitName(e.NodeID)); err == nil {
				e.Probe = &ProbeView{OK: p.OK, IP: p.IP, Colo: p.Colo, Error: p.Error, CheckedAt: p.CheckedAt}
			}
		}
	}
	return out, nil
}

func (h *handlers) patchCascade(ctx context.Context, in *cascadePatchInput) (*cascadeOutput, error) {
	if _, err := h.getNode(ctx, in.ID); err != nil {
		return nil, err
	}
	b := in.Body
	err := h.d.Store.Tx(ctx, func(q *db.Queries) error {
		// The relay is made even before anyone uses the node as an exit: the route is
		// ready when they do.
		n, err := q.GetNode(ctx, in.ID)
		if err != nil {
			return err
		}
		if _, err := domain.EnsureRelay(ctx, q, n, h.d.Now(), h.hostOf(n.ID)); err != nil {
			return err
		}
		outbound, exit := b.Outbound, sql.NullInt64{}
		if outbound == "node" {
			if b.ExitNodeID == 0 {
				return domain.ErrNotFound
			}
			if err := domain.UseExit(ctx, q, in.ID, b.ExitNodeID, h.d.Now(), h.hostOf); err != nil {
				return err
			}
			outbound, exit = "direct", sql.NullInt64{Int64: b.ExitNodeID, Valid: true}
		}
		return q.SetNodeRelayRoute(ctx, db.SetNodeRelayRouteParams{Outbound: outbound, ExitNodeID: exit, NodeID: in.ID})
	})
	if err != nil {
		return nil, cascadeError(err, "exit_node_id")
	}
	h.d.Changes.SlotsChanged()
	h.audit(ctx, sessionOf(ctx).AdminID, "node.cascade", "node", strconv.FormatInt(in.ID, 10), map[string]any{"outbound": b.Outbound, "exit_node_id": b.ExitNodeID})
	return h.getCascade(ctx, &nodeIDInput{ID: in.ID})
}

// CascadeRuleView is one rule of a node's cascade: traffic the match catches (of the
// inbound, when one is named) leaves through the exit node instead of where the rest of
// the listener goes. Rules are ordered: the first that matches wins.
type CascadeRuleView struct {
	ID         int64  `json:"id"`
	Match      string `json:"match" doc:"Каноническое правило без цели: DOMAIN-SUFFIX,example.com, DST-PORT,25-53, IP-CIDR,203.0.113.0/24"`
	InboundID  *int64 `json:"inbound_id,omitempty" doc:"Подключение этой ноды; пусто — любой слушатель, служебный вход включительно"`
	Inbound    string `json:"inbound,omitempty" doc:"Имя подключения"`
	ExitNodeID int64  `json:"exit_node_id" minimum:"1" doc:"Нода, через которую уходит подходящий трафик"`
	ExitName   string `json:"exit_name" doc:"Имя ноды выхода"`
	Position   int64  `json:"position" doc:"Порядок правил"`
}

type cascadeRulesOutput struct {
	Body []CascadeRuleView `json:"body"`
}

type cascadeRuleOutput struct{ Body CascadeRuleView }

type cascadeRulesInput struct {
	ID int64 `path:"id" minimum:"1"`
}

type cascadeRuleInput struct {
	ID     int64 `path:"id" minimum:"1"`
	RuleID int64 `path:"rule_id" minimum:"1"`
}

type createCascadeRuleInput struct {
	ID   int64 `path:"id" minimum:"1"`
	Body struct {
		Match      string `json:"match" maxLength:"600" doc:"Что ловить: TYPE,VALUE — DOMAIN-SUFFIX,example.com, DOMAIN-KEYWORD,example, DST-PORT,25, IP-CIDR,203.0.113.0/24, DOMAIN-REGEX,…"`
		InboundID  *int64 `json:"inbound_id,omitempty" minimum:"1" doc:"Только трафик этого подключения ноды; пусто — любой слушатель"`
		ExitNodeID int64  `json:"exit_node_id" minimum:"1" doc:"Куда отправлять: другая нода панели"`
	}
}

type patchCascadeRuleInput struct {
	ID     int64 `path:"id" minimum:"1"`
	RuleID int64 `path:"rule_id" minimum:"1"`
	Body   struct {
		Match      *string `json:"match,omitempty" maxLength:"600"`
		InboundID  *int64  `json:"inbound_id,omitempty" minimum:"0" doc:"0 — любой слушатель"`
		ExitNodeID *int64  `json:"exit_node_id,omitempty" minimum:"1"`
	}
}

type orderCascadeRulesInput struct {
	ID   int64 `path:"id" minimum:"1"`
	Body struct {
		IDs []int64 `json:"ids" minItems:"1" maxItems:"200" doc:"Все правила ноды, каждая один раз, в новом порядке"`
	}
}

// ruleViews maps the rows to the view with the names the panel shows; inbound and node
// names come from the same lists getCascade uses.
func (h *handlers) ruleViews(ctx context.Context, q *db.Queries, rows []db.CascadeRule) ([]CascadeRuleView, error) {
	nodes, err := q.ListNodes(ctx)
	if err != nil {
		return nil, err
	}
	nodeNames := map[int64]string{}
	for _, n := range nodes {
		nodeNames[n.ID] = n.Name
	}
	ins, err := q.ListInbounds(ctx)
	if err != nil {
		return nil, err
	}
	inNames := map[int64]string{}
	for _, i := range ins {
		inNames[i.ID] = i.Name
	}
	out := make([]CascadeRuleView, 0, len(rows))
	for _, r := range rows {
		v := CascadeRuleView{ID: r.ID, Match: r.Match, ExitNodeID: r.ExitNodeID, ExitName: nodeNames[r.ExitNodeID], Position: r.Position}
		if r.InboundID.Valid {
			id := r.InboundID.Int64
			v.InboundID = &id
			v.Inbound = inNames[id]
		}
		out = append(out, v)
	}
	return out, nil
}

func (h *handlers) listCascadeRules(ctx context.Context, in *cascadeRulesInput) (*cascadeRulesOutput, error) {
	if _, err := h.getNode(ctx, in.ID); err != nil {
		return nil, err
	}
	q := h.d.Store.Q
	rows, err := q.ListCascadeRules(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	views, err := h.ruleViews(ctx, q, rows)
	if err != nil {
		return nil, err
	}
	return &cascadeRulesOutput{Body: views}, nil
}

// checkRuleInput reads the match and the inbound of a rule body and refuses what the
// node would drop anyway: a match mihomo cannot parse, or an inbound of another node.
func (h *handlers) checkRuleInput(ctx context.Context, q *db.Queries, nodeID int64, match string, inboundID *int64) (string, sql.NullInt64, error) {
	m, ok := nodeapi.CanonMatch(match)
	if !ok {
		return "", sql.NullInt64{}, fieldErr("body.match", "match")
	}
	var in sql.NullInt64
	if inboundID != nil && *inboundID != 0 {
		row, err := q.GetInbound(ctx, *inboundID)
		if errors.Is(err, sql.ErrNoRows) || err == nil && row.NodeID != nodeID {
			return "", sql.NullInt64{}, fieldErr("body.inbound_id", "inbound")
		}
		if err != nil {
			return "", sql.NullInt64{}, err
		}
		in = sql.NullInt64{Int64: *inboundID, Valid: true}
	}
	return m, in, nil
}

func (h *handlers) createCascadeRule(ctx context.Context, in *createCascadeRuleInput) (*cascadeRuleOutput, error) {
	if _, err := h.getNode(ctx, in.ID); err != nil {
		return nil, err
	}
	var row db.CascadeRule
	err := h.d.Store.Tx(ctx, func(q *db.Queries) error {
		match, inbound, err := h.checkRuleInput(ctx, q, in.ID, in.Body.Match, in.Body.InboundID)
		if err != nil {
			return err
		}
		if err := domain.UseExit(ctx, q, in.ID, in.Body.ExitNodeID, h.d.Now(), h.hostOf); err != nil {
			return err
		}
		rows, err := q.ListCascadeRules(ctx, in.ID)
		if err != nil {
			return err
		}
		row, err = q.CreateCascadeRule(ctx, db.CreateCascadeRuleParams{
			NodeID: in.ID, InboundID: inbound, Match: match, ExitNodeID: in.Body.ExitNodeID,
			Position: int64(len(rows)) + 1, CreatedAt: h.d.Now().Unix(),
		})
		return err
	})
	if err != nil {
		return nil, cascadeError(err, "exit_node_id")
	}
	h.d.Changes.SlotsChanged()
	h.audit(ctx, sessionOf(ctx).AdminID, "node.cascadeRule.add", "node", strconv.FormatInt(in.ID, 10), map[string]any{"match": row.Match, "exit_node_id": row.ExitNodeID, "inbound_id": row.InboundID})
	views, err := h.ruleViews(ctx, h.d.Store.Q, []db.CascadeRule{row})
	if err != nil {
		return nil, err
	}
	return &cascadeRuleOutput{Body: views[0]}, nil
}

func (h *handlers) patchCascadeRule(ctx context.Context, in *patchCascadeRuleInput) (*cascadeRuleOutput, error) {
	if _, err := h.getNode(ctx, in.ID); err != nil {
		return nil, err
	}
	b := in.Body
	var row db.CascadeRule
	err := h.d.Store.Tx(ctx, func(q *db.Queries) error {
		cur, err := q.GetCascadeRule(ctx, db.GetCascadeRuleParams{ID: in.RuleID, NodeID: in.ID})
		if err != nil {
			return err
		}
		match, inbound, exit := cur.Match, cur.InboundID, cur.ExitNodeID
		if b.Match != nil {
			m, _, err := h.checkRuleInput(ctx, q, in.ID, *b.Match, nil)
			if err != nil {
				return err
			}
			match = m
		}
		if b.InboundID != nil {
			_, in2, err := h.checkRuleInput(ctx, q, in.ID, match, b.InboundID)
			if err != nil {
				return err
			}
			inbound = in2
		}
		if b.ExitNodeID != nil {
			if err := domain.UseExit(ctx, q, in.ID, *b.ExitNodeID, h.d.Now(), h.hostOf); err != nil {
				return err
			}
			exit = *b.ExitNodeID
		}
		row, err = q.UpdateCascadeRule(ctx, db.UpdateCascadeRuleParams{Match: match, InboundID: inbound, ExitNodeID: exit, ID: in.RuleID, NodeID: in.ID})
		return err
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("not_found")
		}
		return nil, cascadeError(err, "exit_node_id")
	}
	h.d.Changes.SlotsChanged()
	h.audit(ctx, sessionOf(ctx).AdminID, "node.cascadeRule.edit", "node", strconv.FormatInt(in.ID, 10), map[string]any{"rule_id": in.RuleID, "match": row.Match, "exit_node_id": row.ExitNodeID})
	views, err := h.ruleViews(ctx, h.d.Store.Q, []db.CascadeRule{row})
	if err != nil {
		return nil, err
	}
	return &cascadeRuleOutput{Body: views[0]}, nil
}

func (h *handlers) deleteCascadeRule(ctx context.Context, in *cascadeRuleInput) (*struct{}, error) {
	if _, err := h.getNode(ctx, in.ID); err != nil {
		return nil, err
	}
	err := h.d.Store.Tx(ctx, func(q *db.Queries) error {
		if _, err := q.GetCascadeRule(ctx, db.GetCascadeRuleParams{ID: in.RuleID, NodeID: in.ID}); err != nil {
			return err
		}
		return q.DeleteCascadeRule(ctx, db.DeleteCascadeRuleParams{ID: in.RuleID, NodeID: in.ID})
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("not_found")
		}
		return nil, err
	}
	h.d.Changes.SlotsChanged()
	h.audit(ctx, sessionOf(ctx).AdminID, "node.cascadeRule.del", "node", strconv.FormatInt(in.ID, 10), map[string]any{"rule_id": in.RuleID})
	return nil, nil
}

// orderCascadeRules sets the order of the node's rules: the list must be every rule once,
// so a stale page of the admin cannot silently drop or duplicate one.
func (h *handlers) orderCascadeRules(ctx context.Context, in *orderCascadeRulesInput) (*struct{}, error) {
	if _, err := h.getNode(ctx, in.ID); err != nil {
		return nil, err
	}
	ids := in.Body.IDs
	err := h.d.Store.Tx(ctx, func(q *db.Queries) error {
		rows, err := q.ListCascadeRules(ctx, in.ID)
		if err != nil {
			return err
		}
		if len(rows) != len(ids) {
			return fieldErr("body.ids", "ids")
		}
		seen := map[int64]bool{}
		for _, id := range ids {
			if seen[id] {
				return fieldErr("body.ids", "ids")
			}
			seen[id] = true
		}
		for _, r := range rows {
			if !seen[r.ID] {
				return fieldErr("body.ids", "ids")
			}
		}
		for i, id := range ids {
			if err := q.SetCascadeRulePosition(ctx, db.SetCascadeRulePositionParams{Position: int64(i) + 1, ID: id, NodeID: in.ID}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	h.d.Changes.SlotsChanged()
	h.audit(ctx, sessionOf(ctx).AdminID, "node.cascadeRule.order", "node", strconv.FormatInt(in.ID, 10), map[string]any{"ids": ids})
	return nil, nil
}
