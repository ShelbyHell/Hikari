// Cascade rules of a node, in Settings → Routing: part of a listener's traffic (the
// match) leaves through another node of the panel instead of where the listener's own
// outbound sends the rest. The whole set is edited as JSON or a pasted Mihomo rule list.
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowRight, Check, Copy, FileCode, RotateCcw, Route, Save, Sparkles } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { api, errorText, unwrap, type Schemas } from "../../../api/client";
import { qk, useInbounds, useNodes, useSettings } from "../../../api/hooks";
import { JsonEditor } from "../../../components/json-editor";
import { useToast } from "../../../components/toast";
import { Button, Pill, Skeleton } from "../../../components/ui";
import { t, useLocale } from "../../../i18n";
import { fieldErrors } from "../../../lib/fields";
import { nodeLabel } from "../../../lib/node-label";

type Rule = Schemas["CascadeRuleView"];
type RuleJson = { match: string; inbound_id?: number; exit_node_id: number };

const serialize = (rules: Rule[]) =>
  JSON.stringify(rules.map((r) => ({ match: r.match, ...(r.inbound_id ? { inbound_id: r.inbound_id } : {}), exit_node_id: r.exit_node_id })), null, 2) + "\n";

const KNOWN_KINDS = new Set([
  "DOMAIN",
  "DOMAIN-SUFFIX",
  "DOMAIN-KEYWORD",
  "DOMAIN-REGEX",
  "DST-PORT",
  "SRC-PORT",
  "IP-CIDR",
  "IP-CIDR6",
  "SRC-IP-CIDR",
  "RULE-SET",
]);

const BUILTIN_RULE_SETS = [
  { name: "mikan-ai", ru: "ChatGPT · Claude · Gemini", en: "ChatGPT · Claude · Gemini" },
  { name: "mikan-blocked", ru: "Заблокированные сайты", en: "Blocked sites" },
  { name: "mikan-ru-sites", ru: "Сайты РФ", en: "RU sites" },
  { name: "mikan-ru-apps", ru: "Приложения РФ", en: "RU apps" },
  { name: "mikan-telegram-ip", ru: "Telegram IP", en: "Telegram IP" },
  { name: "mikan-discord-voice", ru: "Discord Voice", en: "Discord Voice" },
  { name: "mikan-games", ru: "Игры", en: "Games" },
  { name: "mikan-torrent-apps", ru: "Торренты", en: "Torrents" },
];

// A pasted mihomo rule list ("- DOMAIN-SUFFIX,example.com", "- RULE-SET,mikan-ai",
// or a YAML "payload:" document with domains or IPs) becomes cascade rules with the chosen exit.
function parseRuleLines(text: string, exitNodeId: number, badLine: (line: number) => string): { rules?: RuleJson[]; error?: string } {
  const rules: RuleJson[] = [];
  const lines = text.split(/\r?\n/);
  for (let i = 0; i < lines.length; i++) {
    let s = lines[i]!.trim();
    if (!s || s.startsWith("#") || s === "payload:") continue;
    if (s.startsWith("-")) s = s.slice(1).trim();
    // Quoted YAML entries are welcome too.
    if ((s.startsWith("'") && s.endsWith("'")) || (s.startsWith('"') && s.endsWith('"'))) s = s.slice(1, -1).trim();
    if (!s || s.startsWith("#")) continue;

    if (s.includes(",")) {
      const parts = s.split(",").map((p) => p.trim());
      const typ = parts[0]!.toUpperCase();
      if (typ === "MATCH" || typ === "FINAL") continue;

      if (typ === "RULE-SET") {
        const name = parts[1];
        if (!name) return { error: badLine(i + 1) };
        const noResolve = parts.slice(2).some((p) => p.toLowerCase() === "no-resolve");
        rules.push({
          match: noResolve ? `RULE-SET,${name},no-resolve` : `RULE-SET,${name}`,
          exit_node_id: exitNodeId,
        });
        continue;
      }

      if (KNOWN_KINDS.has(typ)) {
        let val = parts[1];
        if (!val) return { error: badLine(i + 1) };
        if (typ === "DOMAIN-REGEX" && parts.length > 2) {
          const last = parts[parts.length - 1]!;
          if (!/[{}()[\]^$*+?|\\]/.test(last)) {
            val = parts.slice(1, -1).join(",");
          } else {
            val = parts.slice(1).join(",");
          }
        }
        rules.push({ match: `${typ},${val}`, exit_node_id: exitNodeId });
        continue;
      }

      return { error: badLine(i + 1) };
    }

    // Single token lines from domain/IP payloads (YAML payload or text lists)
    if (s.startsWith("+.")) {
      const d = s.slice(2).trim();
      if (!d) return { error: badLine(i + 1) };
      rules.push({ match: `DOMAIN-SUFFIX,${d}`, exit_node_id: exitNodeId });
    } else if (s.startsWith(".")) {
      const d = s.slice(1).trim();
      if (!d) return { error: badLine(i + 1) };
      rules.push({ match: `DOMAIN-SUFFIX,${d}`, exit_node_id: exitNodeId });
    } else if (s.startsWith("*.")) {
      const d = s.slice(2).trim();
      if (!d) return { error: badLine(i + 1) };
      rules.push({ match: `DOMAIN-SUFFIX,${d}`, exit_node_id: exitNodeId });
    } else if (s.includes("/")) {
      rules.push({ match: `IP-CIDR,${s}`, exit_node_id: exitNodeId });
    } else if (/^\d{1,3}(\.\d{1,3}){3}$/.test(s)) {
      rules.push({ match: `IP-CIDR,${s}/32`, exit_node_id: exitNodeId });
    } else if (s.includes(":")) {
      rules.push({ match: `IP-CIDR,${s}`, exit_node_id: exitNodeId });
    } else if (/^[a-zA-Z0-9_.\-*+?()\[\]{}^$|\\]+$/.test(s)) {
      rules.push({ match: `DOMAIN,${s}`, exit_node_id: exitNodeId });
    } else {
      return { error: badLine(i + 1) };
    }
  }
  return { rules };
}

export function CascadeRulesPanel() {
  const nodes = useNodes();
  // The local node first; the admin may pick any other.
  const [picked, setPicked] = useState<number>();
  const nodeId = picked ?? nodes.data?.find((n) => n.local)?.id ?? nodes.data?.[0]?.id ?? 0;

  if (nodes.isPending) {
    return <Skeleton style={{ height: 320, borderRadius: 20 }} />;
  }
  if (!nodeId) {
    return (
      <section className="card glass reveal">
        <div className="banner info">{t("cascade.ruleEmpty")}</div>
      </section>
    );
  }
  return <Editor key={nodeId} nodeId={nodeId} nodes={nodes.data ?? []} onPick={setPicked} picked={picked} />;
}

function Editor({
  nodeId,
  nodes,
  onPick,
  picked,
}: {
  nodeId: number;
  nodes: NonNullable<ReturnType<typeof useNodes>["data"]>;
  onPick: (id: number | undefined) => void;
  picked: number | undefined;
}) {
  const qc = useQueryClient();
  const toast = useToast();
  const inbounds = useInbounds();
  const settings = useSettings();
  const locale = useLocale();
  const rules = useQuery({
    queryKey: qk.cascadeRules(nodeId),
    queryFn: ({ signal }) => unwrap(api.GET("/api/v1/nodes/{id}/cascade/rules", { params: { path: { id: nodeId } }, signal })),
  });
  const [text, setText] = useState("");
  const [dirty, setDirty] = useState(false);
  const [parseError, setParseError] = useState("");
  const [listExit, setListExit] = useState(0);
  const [copiedKey, setCopiedKey] = useState<string | null>(null);

  // The ids the editor refers to, so the admin does not have to look them up elsewhere.
  const mine = (inbounds.data ?? []).filter((i) => i.node_id === nodeId);
  const others = nodes.filter((n) => n.id !== nodeId);

  const customRuleSets = useMemo(() => {
    const lists = settings.data?.sub_routes?.lists ?? [];
    return lists.map((l) => ({
      name: `own-${l.name}`,
      label: l.name,
    }));
  }, [settings.data?.sub_routes?.lists]);

  const ruleSets = useMemo(
    () => [
      ...customRuleSets,
      ...BUILTIN_RULE_SETS.map((b) => ({
        name: b.name,
        label: locale === "ru" ? b.ru : b.en,
      })),
    ],
    [customRuleSets, locale],
  );

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

  const trimmed = text.trim();
  const isJson = !trimmed || trimmed.startsWith("[");

  // Rule count detection
  const ruleCount = useMemo(() => {
    if (!trimmed) return 0;
    if (isJson) {
      try {
        const parsed = JSON.parse(text);
        return Array.isArray(parsed) ? parsed.length : 0;
      } catch {
        return 0;
      }
    }
    return text
      .split(/\r?\n/)
      .map((l) => l.trim())
      .filter((s) => s && !s.startsWith("#") && s !== "payload:" && !s.toUpperCase().startsWith("MATCH,") && !s.toUpperCase().startsWith("- MATCH,")).length;
  }, [text, trimmed, isJson]);

  const formatJson = () => {
    try {
      const parsed = JSON.parse(text || "[]");
      setText(JSON.stringify(parsed, null, 2) + "\n");
      setDirty(true);
      setParseError("");
    } catch {
      setParseError(t("cascade.rulesFormatErr"));
    }
  };

  const insertExample = () => {
    const defaultExit = others[0]?.id ?? 2;
    const example: RuleJson[] = [
      { match: "RULE-SET,mikan-ai", exit_node_id: defaultExit },
      { match: "DOMAIN-SUFFIX,youtube.com", exit_node_id: defaultExit },
      { match: "DOMAIN-KEYWORD,openai", exit_node_id: defaultExit },
      { match: "IP-CIDR,8.8.8.8/32", exit_node_id: defaultExit },
    ];
    setText(JSON.stringify(example, null, 2) + "\n");
    setDirty(true);
    setParseError("");
  };

  const convertListToJson = () => {
    if (!listExit) {
      setParseError(t("cascade.rulesListExitNeeded"));
      return;
    }
    const { rules: parsed, error } = parseRuleLines(text, listExit, (line) => t("cascade.rulesListBadLine", { line }));
    if (error) {
      setParseError(error);
      return;
    }
    if (parsed) {
      setText(JSON.stringify(parsed, null, 2) + "\n");
      setDirty(true);
      setParseError("");
      toast.ok(t("cascade.rulesConverted"));
    }
  };

  const copyText = (val: string, key: string) => {
    void navigator.clipboard.writeText(val);
    setCopiedKey(key);
    setTimeout(() => setCopiedKey((k) => (k === key ? null : k)), 1800);
    toast.ok(t("cascade.rulesCopied"));
  };

  const apply = () => {
    if (trimmed && !trimmed.startsWith("[")) {
      // Not JSON: a mihomo rule list. Every line takes the exit picked below.
      if (!listExit) {
        setParseError(t("cascade.rulesListExitNeeded"));
        return;
      }
      const { rules: parsed, error } = parseRuleLines(text, listExit, (line) => t("cascade.rulesListBadLine", { line }));
      if (error) {
        setParseError(error);
        return;
      }
      setParseError("");
      save.mutate(parsed ?? []);
      return;
    }
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

  return (
    <section className="card glass reveal" style={{ "--i": 1 } as React.CSSProperties}>
      <div className="card-head">
        <div>
          <div className="flex items-center gap-2">
            <Route size={18} className="text-[var(--mikan-600)]" aria-hidden />
            <h2 className="card-title">{t("cascade.rules")}</h2>
          </div>
          <div className="card-sub">{t("cascade.rulesNodeHint")}</div>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <label className="flex items-center gap-2 text-xs text-[var(--ink-500)]">
            <span>{t("cascade.nodePick")}:</span>
            <select
              className="input max-w-[240px]"
              value={picked ?? nodeId}
              onChange={(e) => onPick(Number(e.target.value))}
              aria-label={t("cascade.nodePick")}
            >
              {nodes.map((n) => (
                <option key={n.id} value={n.id} disabled={!n.enabled && n.id !== nodeId}>
                  {nodeLabel(n)}
                </option>
              ))}
            </select>
          </label>
        </div>
      </div>

      <p className="mb-4 text-xs text-[var(--ink-500)]">{t("cascade.rulesJsonHint")}</p>

      {/* Editor toolbar */}
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="ghost" size="sm" onClick={formatJson} disabled={!isJson}>
            <Sparkles size={14} aria-hidden />
            <span>{t("cascade.rulesFormat")}</span>
          </Button>
          {!trimmed ? (
            <Button variant="ghost" size="sm" onClick={insertExample}>
              <FileCode size={14} aria-hidden />
              <span>{t("cascade.rulesExample")}</span>
            </Button>
          ) : null}
          {ruleCount > 0 ? (
            <Pill tone="ok">{t("cascade.rulesCount", { n: ruleCount })}</Pill>
          ) : (
            <Pill tone="off">{t("cascade.rulesCount", { n: 0 })}</Pill>
          )}
        </div>

        {/* Rule list exit node options */}
        <div className="flex flex-wrap items-center gap-2">
          <label className="flex items-center gap-2 text-xs text-[var(--ink-500)]">
            <span>{t("cascade.rulesListExit")}:</span>
            <select
              className="input input-sm max-w-[190px]"
              value={listExit}
              onChange={(e) => setListExit(Number(e.target.value))}
              aria-label={t("cascade.rulesListExit")}
            >
              <option value={0} disabled>
                {t("inbounds.exitPick")}
              </option>
              {others.map((n) => (
                <option key={n.id} value={n.id} disabled={!n.enabled}>
                  {nodeLabel(n)}
                </option>
              ))}
            </select>
          </label>
          {!isJson && trimmed ? (
            <Button variant="ghost" size="sm" onClick={convertListToJson} disabled={!listExit} title={t("cascade.rulesConvertToJson")}>
              <ArrowRight size={14} aria-hidden />
              <span>{t("cascade.rulesConvertToJson")}</span>
            </Button>
          ) : null}
        </div>
      </div>

      {rules.isPending ? (
        <Skeleton style={{ height: 300, borderRadius: 16 }} />
      ) : (
        <JsonEditor
          value={text}
          onChange={(v) => {
            setText(v);
            setDirty(true);
            setParseError("");
          }}
          label={t("cascade.rules")}
          invalid={!!parseError}
        />
      )}

      {parseError ? (
        <div className="banner err mt-3" role="alert">
          {parseError}
        </div>
      ) : null}

      {save.error ? (
        Object.keys(fields).length ? (
          <div className="banner err mt-3" role="alert">
            {[fields.match, fields.inbound_id, fields.exit_node_id, fields.body].filter(Boolean).join(" · ") || errorText(save.error)}
          </div>
        ) : (
          <div className="banner err mt-3" role="alert">
            {errorText(save.error)}
          </div>
        )
      ) : null}

      {/* ID & RULE-SET quick-reference chips */}
      <div className="mt-4 flex flex-col gap-3 rounded-xl border border-[var(--hairline)] bg-[var(--surface)] p-3">
        {others.length ? (
          <div>
            <div className="mb-1.5 text-xs font-semibold text-[var(--ink-700)]">{t("cascade.rulesLegendNodes")}</div>
            <div className="flex flex-wrap gap-1.5">
              {others.map((n) => {
                const key = `node-${n.id}`;
                const copied = copiedKey === key;
                return (
                  <button
                    key={n.id}
                    type="button"
                    className="chip-btn inline-flex items-center gap-1.5 py-1 px-2.5 text-xs"
                    onClick={() => copyText(String(n.id), key)}
                    title={t("cascade.rulesCopied")}
                  >
                    <span className="font-mono font-semibold text-[var(--mikan-600)]">exit_node_id: {n.id}</span>
                    <span className="text-[var(--ink-600)]">({nodeLabel(n)})</span>
                    {copied ? <Check size={12} className="text-[var(--leaf-600)]" aria-hidden /> : <Copy size={12} className="opacity-50" aria-hidden />}
                  </button>
                );
              })}
            </div>
          </div>
        ) : null}

        {mine.length ? (
          <div>
            <div className="mb-1.5 text-xs font-semibold text-[var(--ink-700)]">{t("cascade.rulesLegendInbounds")}</div>
            <div className="flex flex-wrap gap-1.5">
              {mine.map((i) => {
                const key = `inbound-${i.id}`;
                const copied = copiedKey === key;
                return (
                  <button
                    key={i.id}
                    type="button"
                    className="chip-btn inline-flex items-center gap-1.5 py-1 px-2.5 text-xs"
                    onClick={() => copyText(String(i.id), key)}
                    title={t("cascade.rulesCopied")}
                  >
                    <span className="font-mono font-semibold text-[var(--leaf-600)]">inbound_id: {i.id}</span>
                    <span className="text-[var(--ink-600)]">({i.display_name || i.name})</span>
                    {copied ? <Check size={12} className="text-[var(--leaf-600)]" aria-hidden /> : <Copy size={12} className="opacity-50" aria-hidden />}
                  </button>
                );
              })}
            </div>
          </div>
        ) : null}

        {ruleSets.length ? (
          <div>
            <div className="mb-1.5 text-xs font-semibold text-[var(--ink-700)]">{t("cascade.rulesLegendRuleSets")}</div>
            <div className="flex flex-wrap gap-1.5">
              {ruleSets.map((rs) => {
                const key = `ruleset-${rs.name}`;
                const copied = copiedKey === key;
                return (
                  <button
                    key={rs.name}
                    type="button"
                    className="chip-btn inline-flex items-center gap-1.5 py-1 px-2.5 text-xs"
                    onClick={() => copyText(`RULE-SET,${rs.name}`, key)}
                    title={t("cascade.rulesCopied")}
                  >
                    <span className="font-mono font-semibold text-[var(--lagoon-600)]">RULE-SET,{rs.name}</span>
                    {rs.label ? <span className="text-[var(--ink-600)]">({rs.label})</span> : null}
                    {copied ? <Check size={12} className="text-[var(--leaf-600)]" aria-hidden /> : <Copy size={12} className="opacity-50" aria-hidden />}
                  </button>
                );
              })}
            </div>
          </div>
        ) : null}
      </div>

      {/* Form actions */}
      <div className="mt-4 flex items-center gap-2">
        <Button variant="primary" size="sm" loading={save.isPending} disabled={!dirty} onClick={apply}>
          <Save size={14} aria-hidden />
          <span>{t("cascade.rulesJsonApply")}</span>
        </Button>
        <Button variant="ghost" size="sm" disabled={!dirty || save.isPending} onClick={reset}>
          <RotateCcw size={14} aria-hidden />
          <span>{t("cascade.rulesJsonReset")}</span>
        </Button>
      </div>
    </section>
  );
}
