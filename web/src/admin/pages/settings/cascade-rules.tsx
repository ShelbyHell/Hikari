// Cascade rules of a node, in Settings → Routing: part of a listener's traffic (the
// match) leaves through another node of the panel instead of where the listener's own
// outbound sends the rest. The whole set is edited as JSON; the order of the list is
// the order of the rules — the first that matches wins.
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { RotateCcw, Save } from "lucide-react";
import { useEffect, useState } from "react";
import { api, errorText, unwrap, type Schemas } from "../../../api/client";
import { qk, useInbounds, useNodes } from "../../../api/hooks";
import { useToast } from "../../../components/toast";
import { Button, Skeleton } from "../../../components/ui";
import { t } from "../../../i18n";
import { fieldErrors } from "../../../lib/fields";
import { nodeLabel } from "../../../lib/node-label";

type Rule = Schemas["CascadeRuleView"];
type RuleJson = { match: string; inbound_id?: number; exit_node_id: number };

const serialize = (rules: Rule[]) =>
  JSON.stringify(rules.map((r) => ({ match: r.match, ...(r.inbound_id ? { inbound_id: r.inbound_id } : {}), exit_node_id: r.exit_node_id })), null, 2) + "\n";

export function CascadeRulesPanel() {
  const nodes = useNodes();
  // The local node first; the admin may pick any other.
  const [picked, setPicked] = useState<number>();
  const nodeId = picked ?? nodes.data?.find((n) => n.local)?.id ?? nodes.data?.[0]?.id ?? 0;
  if (!nodeId) return <Skeleton style={{ height: 120, borderRadius: 16 }} />;
  return <Editor key={nodeId} nodeId={nodeId} nodes={nodes.data ?? []} onPick={setPicked} picked={picked} />;
}

function Editor({ nodeId, nodes, onPick, picked }: { nodeId: number; nodes: NonNullable<ReturnType<typeof useNodes>["data"]>; onPick: (id: number | undefined) => void; picked: number | undefined }) {
  const qc = useQueryClient();
  const toast = useToast();
  const inbounds = useInbounds();
  const rules = useQuery({
    queryKey: qk.cascadeRules(nodeId),
    queryFn: ({ signal }) => unwrap(api.GET("/api/v1/nodes/{id}/cascade/rules", { params: { path: { id: nodeId } }, signal })),
  });
  const [text, setText] = useState("");
  const [dirty, setDirty] = useState(false);
  const [parseError, setParseError] = useState("");
  // The server's list feeds the editor until the admin starts typing; a successful save
  // refreshes it with the canonical matches the panel stored.
  useEffect(() => {
    if (rules.data && !dirty) setText(serialize(rules.data));
  }, [rules.data, dirty]);

  const save = useMutation({
    mutationFn: (body: RuleJson[]) => unwrap(api.PUT("/api/v1/nodes/{id}/cascade/rules", { params: { path: { id: nodeId } }, body })),
    onSuccess: (v) => {
      qc.setQueryData(qk.cascadeRules(nodeId), v);
      setText(serialize(v));
      setDirty(false);
      setParseError("");
      toast.ok(t("cascade.rulesJsonSaved"));
    },
  });
  const fields = fieldErrors(save.error);

  const apply = () => {
    let body: RuleJson[];
    try {
      body = JSON.parse(text || "[]");
    } catch (e) {
      setParseError(t("cascade.rulesJsonBad", { err: e instanceof Error ? e.message : String(e) }));
      return;
    }
    if (!Array.isArray(body)) {
      setParseError(t("cascade.rulesJsonArray"));
      return;
    }
    setParseError("");
    save.mutate(body);
  };
  const reset = () => {
    setText(serialize(rules.data ?? []));
    setDirty(false);
    setParseError("");
    save.reset();
  };

  // The ids the JSON refers to, so the admin does not have to look them up elsewhere.
  const mine = (inbounds.data ?? []).filter((i) => i.node_id === nodeId);
  const others = nodes.filter((n) => n.id !== nodeId);
  const legend = [
    ...others.map((n) => `exit_node_id ${n.id} = ${nodeLabel(n)}`),
    ...mine.map((i) => `inbound_id ${i.id} = ${i.display_name || i.name}`),
  ].join("\n");

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
      <p className="mb-2 text-xs text-[var(--ink-500)]">{t("cascade.rulesJsonHint")}</p>
      {rules.isPending ? (
        <Skeleton style={{ height: 200, borderRadius: 16 }} />
      ) : (
        <div className="card rules-editor">
          <textarea
            value={text}
            onChange={(e) => {
              setText(e.target.value);
              setDirty(true);
            }}
            spellCheck={false}
            aria-label={t("cascade.rules")}
            aria-invalid={!!parseError}
          />
        </div>
      )}
      {parseError ? (
        <p className="mt-2 text-xs text-[var(--berry-600)]" role="alert">
          {parseError}
        </p>
      ) : null}
      {save.error ? (
        Object.keys(fields).length ? (
          <p className="mt-2 text-xs text-[var(--berry-600)]" role="alert">
            {[fields.match, fields.inbound_id, fields.exit_node_id, fields.body].filter(Boolean).join(" · ") || errorText(save.error)}
          </p>
        ) : (
          <div className="banner err mt-2">{errorText(save.error)}</div>
        )
      ) : null}
      {legend ? <pre className="mt-2 whitespace-pre-wrap text-xs text-[var(--ink-400)]">{legend}</pre> : null}
      <div className="mt-3 flex items-center gap-2">
        <Button variant="primary" size="sm" loading={save.isPending} disabled={!dirty} onClick={apply}>
          <Save size={14} aria-hidden /> {t("cascade.rulesJsonApply")}
        </Button>
        <Button variant="ghost" size="sm" disabled={!dirty || save.isPending} onClick={reset}>
          <RotateCcw size={14} aria-hidden /> {t("cascade.rulesJsonReset")}
        </Button>
      </div>
    </div>
  );
}
