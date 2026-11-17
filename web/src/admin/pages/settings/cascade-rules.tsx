// Cascade rules of a node, in Settings → Routing: part of a listener's traffic (the
// match) leaves through another node of the panel instead of where the listener's own
// outbound sends the rest. The rules are ordered: the first that matches wins.
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ChevronDown, ChevronUp, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { api, errorText, unwrap, type Schemas } from "../../../api/client";
import { qk, useInbounds, useNodes } from "../../../api/hooks";
import { useToast } from "../../../components/toast";
import { Button, Skeleton } from "../../../components/ui";
import { t } from "../../../i18n";
import { fieldErrors } from "../../../lib/fields";
import { nodeLabel } from "../../../lib/node-label";

type Rule = Schemas["CascadeRuleView"];

export function CascadeRulesPanel() {
  const nodes = useNodes();
  // The local node first; the admin may pick any other.
  const [picked, setPicked] = useState<number>();
  const nodeId = picked ?? nodes.data?.find((n) => n.local)?.id ?? nodes.data?.[0]?.id ?? 0;
  if (!nodeId) return <Skeleton style={{ height: 120, borderRadius: 16 }} />;
  return <Rules nodeId={nodeId} nodes={nodes.data ?? []} onPick={setPicked} picked={picked} />;
}

function Rules({ nodeId, nodes, onPick, picked }: { nodeId: number; nodes: NonNullable<ReturnType<typeof useNodes>["data"]>; onPick: (id: number | undefined) => void; picked: number | undefined }) {
  const qc = useQueryClient();
  const toast = useToast();
  const inbounds = useInbounds();
  const rules = useQuery({
    queryKey: qk.cascadeRules(nodeId),
    queryFn: ({ signal }) => unwrap(api.GET("/api/v1/nodes/{id}/cascade/rules", { params: { path: { id: nodeId } }, signal })),
  });
  const setRules = (v: Rule[]) => qc.setQueryData(qk.cascadeRules(nodeId), v);
  const list = rules.data ?? [];
  const mine = (inbounds.data ?? []).filter((i) => i.node_id === nodeId);
  const others = nodes.filter((n) => n.id !== nodeId);

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
    <div className="mt-4">
      <div className="mb-3 flex flex-wrap items-end gap-3">
        <div>
          <div className="mb-1 text-[13px] font-semibold">{t("cascade.rules")}</div>
          <p className="text-xs text-[var(--ink-500)]">{t("cascade.rulesNodeHint")}</p>
        </div>
        <select className="input ml-auto max-w-[280px]" value={picked ?? nodeId} onChange={(e) => onPick(Number(e.target.value))} aria-label={t("cascade.nodePick")}>
          {nodes.map((n) => (
            <option key={n.id} value={n.id} disabled={!n.enabled && n.id !== nodeId}>
              {nodeLabel(n)}
            </option>
          ))}
        </select>
      </div>
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
    </div>
  );
}
