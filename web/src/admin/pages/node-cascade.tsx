// Cascades: client → this node → another node → internet. Inbounds pick their exit in
// their own settings; here a node shows where it sends traffic, who comes through it,
// where the traffic other nodes relay through it goes next, and its cascade rules —
// which part of the traffic leaves through which node.
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowRight, ChevronDown, ChevronUp, Plus, RefreshCw, Trash2 } from "lucide-react";
import { useState } from "react";
import { api, errorText, unwrap, type Schemas } from "../../api/client";
import { qk, useInbounds, useNodes } from "../../api/hooks";
import { Drawer } from "../../components/overlay";
import { QueryBoundary } from "../../components/query";
import { useToast } from "../../components/toast";
import { Button, Field, Pill, Segmented, Skeleton } from "../../components/ui";
import { t } from "../../i18n";
import { useDraft } from "../../lib/draft";
import { fieldErrors } from "../../lib/fields";
import { ago } from "../../lib/format";
import { nodeLabel } from "../../lib/node-label";

type Cascade = Schemas["CascadeView"];
type Rule = Schemas["CascadeRuleView"];
type Route = "direct" | "warp" | "node";

export function CascadeDrawer({ node, onClose }: { node: { id: number; name: string } | null; onClose: () => void }) {
  const cascade = useQuery({
    queryKey: qk.cascade(node?.id ?? 0),
    queryFn: ({ signal }) => unwrap(api.GET("/api/v1/nodes/{id}/cascade", { params: { path: { id: node!.id } }, signal })),
    enabled: !!node,
  });
  return (
    <Drawer open={!!node} onOpenChange={(v) => !v && onClose()} title={t("cascade.title")} meta={node?.name}>
      <div className="pt-5">
        <p className="mb-4 text-[13px] text-[var(--ink-600)]">{t("cascade.intro")}</p>
        <QueryBoundary query={cascade} pending={<Skeleton style={{ height: 240, borderRadius: 16 }} />}>
          {(c) => <Body key={node!.id} nodeId={node!.id} c={c} refetch={() => void cascade.refetch()} checking={cascade.isFetching} />}
        </QueryBoundary>
      </div>
    </Drawer>
  );
}

function Body({ nodeId, c, refetch, checking }: { nodeId: number; c: Cascade; refetch: () => void; checking: boolean }) {
  const qc = useQueryClient();
  const toast = useToast();
  const nodes = useNodes();
  const name = (id: number) => {
    const n = nodes.data?.find((x) => x.id === id);
    return n ? nodeLabel(n) : `#${id}`;
  };
  // "Check" refetches `c`: the chosen route survives it.
  const {
    draft: { route, exit },
    setDraft,
  } = useDraft<{ route: Route; exit: number }>({ route: (c.relay?.outbound as Route) ?? "direct", exit: c.relay?.exit_node_id ?? 0 });
  const setRoute = (v: Route) => setDraft((d) => ({ ...d, route: v }));
  const setExit = (v: number) => setDraft((d) => ({ ...d, exit: v }));
  const save = useMutation({
    mutationFn: () =>
      unwrap(api.PATCH("/api/v1/nodes/{id}/cascade", { params: { path: { id: nodeId } }, body: { outbound: route, ...(route === "node" ? { exit_node_id: exit } : {}) } })),
    onSuccess: (v) => {
      qc.setQueryData(qk.cascade(nodeId), v);
      toast.ok(t("cascade.saved"));
    },
  });
  const fields = fieldErrors(save.error);
  const changed = route !== ((c.relay?.outbound as Route) ?? "direct") || (route === "node" && exit !== (c.relay?.exit_node_id ?? 0));

  return (
    <>
      <section className="mb-5">
        <div className="mb-2 flex items-center justify-between gap-2">
          <h3 className="text-[13px] font-semibold">{t("cascade.exits")}</h3>
          {c.exits.length ? (
            <Button size="sm" variant="ghost" loading={checking} onClick={refetch}>
              <RefreshCw size={14} aria-hidden /> {t("cascade.check")}
            </Button>
          ) : null}
        </div>
        {c.exits.length === 0 ? (
          <p className="text-xs text-[var(--ink-500)]">{t("cascade.exitsNone")}</p>
        ) : (
          <ul className="row-list">
            {c.exits.map((e) => (
              <li key={e.node_id} className="py-3">
                <div className="flex flex-wrap items-center gap-2 text-[13px]">
                  <ArrowRight size={14} aria-hidden className="text-[var(--ink-400)]" />
                  <b>{e.name || name(e.node_id)}</b>
                  {e.probe ? (
                    e.probe.ok ? (
                      <Pill tone="ok">{t("cascade.exitIP", { ip: e.probe.ip ?? "" })}</Pill>
                    ) : (
                      <Pill tone="bad">{t("cascade.down")}</Pill>
                    )
                  ) : (
                    <Pill tone="off">{t("cascade.unknown")}</Pill>
                  )}
                </div>
                <div className="mt-1 text-xs text-[var(--ink-500)]">
                  {[...e.inbounds, ...(e.relay ? [t("cascade.relayed")] : [])].join(", ")}
                  {e.probe ? ` · ${ago(e.probe.checked_at)}` : ""}
                </div>
              </li>
            ))}
          </ul>
        )}
        <p className="mt-2 text-xs text-[var(--ink-500)]">{t("cascade.exitsHint")}</p>
      </section>

      <RulesSection nodeId={nodeId} />

      <section className="border-t border-[var(--hairline)] pt-4">
        <h3 className="mb-1 text-[13px] font-semibold">{t("cascade.relay")}</h3>
        <p className="mb-3 text-xs text-[var(--ink-500)]">
          {c.relay && c.relay.sources.length
            ? t("cascade.sources", { list: c.relay.sources.map((s) => s.name || name(s.node_id)).join(", "), port: c.relay.port })
            : t("cascade.sourcesNone")}
        </p>
        {c.relay ? <RelayListener relay={c.relay} /> : null}
        {save.error && !Object.keys(fields).length ? <div className="banner err mb-4">{errorText(save.error)}</div> : null}
        <Field label={t("cascade.route")} hint={t(`cascade.routeHint.${route}`)} error={fields.exit_node_id}>
          <Segmented
            label={t("cascade.route")}
            value={route}
            onChange={setRoute}
            options={[
              { value: "direct", label: t("inbounds.outboundDirect") },
              { value: "warp", label: "WARP" },
              { value: "node", label: t("inbounds.outboundNode") },
            ]}
          />
          {route === "node" ? (
            <select className="input mt-2 max-w-[320px]" value={exit} onChange={(e) => setExit(Number(e.target.value))} aria-label={t("inbounds.exitNode")}>
              <option value={0} disabled>
                {t("inbounds.exitPick")}
              </option>
              {(nodes.data ?? [])
                .filter((n) => n.id !== nodeId)
                .map((n) => (
                  <option key={n.id} value={n.id} disabled={!n.enabled}>
                    {nodeLabel(n)}
                  </option>
                ))}
            </select>
          ) : null}
        </Field>
        <div className="form-actions">
          <Button variant="primary" loading={save.isPending} disabled={!changed || (route === "node" && !exit)} onClick={() => save.mutate()}>
            {t("common.save")}
          </Button>
        </div>
      </section>
    </>
  );
}

// The node's cascade rules: part of a listener's traffic (the match) leaves through
// another node instead of where the listener's own outbound sends the rest. The rules
// are ordered: the first that matches wins.
function RulesSection({ nodeId }: { nodeId: number }) {
  const qc = useQueryClient();
  const toast = useToast();
  const nodes = useNodes();
  const inbounds = useInbounds();
  const rules = useQuery({
    queryKey: qk.cascadeRules(nodeId),
    queryFn: ({ signal }) => unwrap(api.GET("/api/v1/nodes/{id}/cascade/rules", { params: { path: { id: nodeId } }, signal })),
  });
  const setRules = (v: Rule[]) => qc.setQueryData(qk.cascadeRules(nodeId), v);
  const list = rules.data ?? [];
  const mine = (inbounds.data ?? []).filter((i) => i.node_id === nodeId);
  const others = (nodes.data ?? []).filter((n) => n.id !== nodeId);

  const patch = useMutation({
    mutationFn: (vars: { ruleId: number; body: { match?: string; inbound_id?: number; exit_node_id?: number } }) =>
      unwrap(api.PATCH("/api/v1/nodes/{id}/cascade/rules/{rule_id}", { params: { path: { id: nodeId, rule_id: vars.ruleId } }, body: vars.body })),
    onSuccess: (v) => {
      setRules(list.map((r) => (r.id === v.id ? v : r)));
      toast.ok(t("cascade.ruleSaved"));
    },
  });
  const add = useMutation({
    mutationFn: (body: { match: string; inbound_id?: number; exit_node_id: number }) =>
      unwrap(api.POST("/api/v1/nodes/{id}/cascade/rules", { params: { path: { id: nodeId } }, body })),
    onSuccess: (v) => {
      setRules([...list, v]);
      setMatch("");
      toast.ok(t("cascade.ruleAdded"));
    },
  });
  const del = useMutation({
    mutationFn: (ruleId: number) => unwrap(api.DELETE("/api/v1/nodes/{id}/cascade/rules/{rule_id}", { params: { path: { id: nodeId, rule_id: ruleId } } })),
    onSuccess: (_, ruleId) => {
      setRules(list.filter((r) => r.id !== ruleId));
      toast.ok(t("cascade.ruleDeleted"));
    },
  });
  const reorder = useMutation({
    mutationFn: (ids: number[]) => unwrap(api.PUT("/api/v1/nodes/{id}/cascade/rules/order", { params: { path: { id: nodeId } }, body: { ids } })),
    onSuccess: (_, ids) => {
      setRules(ids.map((id, i) => ({ ...list.find((r) => r.id === id)!, position: i + 1 })));
      toast.ok(t("cascade.ruleMoved"));
    },
  });
  const move = (i: number, d: number) => {
    const j = i + d;
    if (j < 0 || j >= list.length) return;
    const ids = list.map((r) => r.id);
    const tmp = ids[i]!;
    ids[i] = ids[j]!;
    ids[j] = tmp;
    reorder.mutate(ids);
  };

  const [match, setMatch] = useState("");
  const [inbound, setInbound] = useState(0);
  const [exit, setExit] = useState(0);
  const addFields = fieldErrors(add.error);
  const canAdd = match.trim() !== "" && exit !== 0;
  const busy = patch.isPending || add.isPending || del.isPending || reorder.isPending;

  const inboundSelect = (value: number, onChange: (v: number) => void, id: string) => (
    <select id={id} className="input" value={value} onChange={(e) => onChange(Number(e.target.value))} aria-label={t("cascade.ruleInbound")}>
      <option value={0}>{t("cascade.ruleAnyInbound")}</option>
      {mine.map((i) => (
        <option key={i.id} value={i.id}>
          {i.display_name || i.name}
        </option>
      ))}
    </select>
  );
  const exitSelect = (value: number, onChange: (v: number) => void, id: string, withPlaceholder = false) => (
    <select id={id} className="input" value={value} onChange={(e) => onChange(Number(e.target.value))} aria-label={t("cascade.ruleExit")}>
      {withPlaceholder ? (
        <option value={0} disabled>
          {t("inbounds.exitPick")}
        </option>
      ) : null}
      {others.map((n) => (
        <option key={n.id} value={n.id} disabled={!n.enabled}>
          {nodeLabel(n)}
        </option>
      ))}
    </select>
  );

  return (
    <section className="mb-5 border-t border-[var(--hairline)] pt-4">
      <h3 className="mb-1 text-[13px] font-semibold">{t("cascade.rules")}</h3>
      <p className="mb-3 text-xs text-[var(--ink-500)]">{t("cascade.rulesHint")}</p>
      {rules.isPending ? (
        <Skeleton style={{ height: 80, borderRadius: 16 }} />
      ) : list.length === 0 ? (
        <p className="text-xs text-[var(--ink-500)]">{t("cascade.ruleEmpty")}</p>
      ) : (
        <ul className="row-list mb-3">
          {list.map((r, i) => (
            <li key={r.id} className="flex flex-wrap items-center gap-2 py-2">
              <input
                key={`${r.id}:${r.match}`}
                className="input mono min-w-0 flex-1"
                defaultValue={r.match}
                spellCheck={false}
                aria-label={t("cascade.ruleMatch")}
                onBlur={(e) => {
                  const v = e.target.value.trim();
                  if (v && v !== r.match) patch.mutate({ ruleId: r.id, body: { match: v } });
                }}
              />
              {inboundSelect(r.inbound_id ?? 0, (v) => patch.mutate({ ruleId: r.id, body: { inbound_id: v } }), `cr-in-${r.id}`)}
              {exitSelect(r.exit_node_id, (v) => patch.mutate({ ruleId: r.id, body: { exit_node_id: v } }), `cr-ex-${r.id}`)}
              <div className="flex items-center">
                <Button size="sm" variant="ghost" disabled={busy || i === 0} onClick={() => move(i, -1)} aria-label={t("cascade.ruleUp")} title={t("cascade.ruleUp")}>
                  <ChevronUp size={14} aria-hidden />
                </Button>
                <Button size="sm" variant="ghost" disabled={busy || i === list.length - 1} onClick={() => move(i, 1)} aria-label={t("cascade.ruleDown")} title={t("cascade.ruleDown")}>
                  <ChevronDown size={14} aria-hidden />
                </Button>
                <Button size="sm" variant="ghost" disabled={busy} onClick={() => del.mutate(r.id)} aria-label={t("cascade.ruleDelete")} title={t("cascade.ruleDelete")}>
                  <Trash2 size={14} aria-hidden className="text-[var(--bad)]" />
                </Button>
              </div>
            </li>
          ))}
        </ul>
      )}
      {(patch.error || add.error) && !Object.keys(fieldErrors(patch.error ?? add.error)).length ? (
        <div className="banner err mb-3">{errorText(patch.error ?? add.error)}</div>
      ) : null}
      <div className="flex flex-wrap items-center gap-2">
        <input
          className="input mono min-w-0 flex-1"
          value={match}
          onChange={(e) => setMatch(e.target.value)}
          placeholder={t("cascade.ruleMatchPh")}
          spellCheck={false}
          aria-label={t("cascade.ruleMatch")}
          aria-invalid={!!addFields.match}
        />
        {inboundSelect(inbound, setInbound, "cr-new-in")}
        {exitSelect(exit, setExit, "cr-new-ex", true)}
        <Button variant="ghost" loading={add.isPending} disabled={!canAdd} onClick={() => add.mutate({ match: match.trim(), ...(inbound ? { inbound_id: inbound } : {}), exit_node_id: exit })}>
          <Plus size={14} aria-hidden /> {t("cascade.ruleAdd")}
        </Button>
      </div>
      {addFields.match || addFields.inbound_id || addFields.exit_node_id ? (
        <p className="mt-2 text-xs text-[var(--berry-600)]" role="alert">
          {[addFields.match, addFields.inbound_id, addFields.exit_node_id].filter(Boolean).join(" · ")}
        </p>
      ) : null}
    </section>
  );
}

// The relay's listener on the node. A port another program holds keeps it from starting,
// and the nodes that leave through it then reach that program instead.
function RelayListener({ relay }: { relay: NonNullable<Cascade["relay"]> }) {
  const state = relay.listener;
  if (state === "unknown") return null;
  return (
    <div className="mb-3">
      <Pill tone={state === "ok" ? "ok" : "bad"}>{t(`cascade.listener.${state}`, { port: relay.port })}</Pill>
      {state === "ok" ? null : (
        <div className="banner err mt-2" role="alert">
          <div>
            {t(state === "busy" ? "cascade.listener.busyText" : "cascade.listener.failedText", { port: relay.port })}
            {relay.error ? <div className="mt-1 break-all font-mono text-xs opacity-80">{relay.error}</div> : null}
          </div>
        </div>
      )}
    </div>
  );
}
