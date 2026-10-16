// External subscriptions (upstream feeds re-served under the panel's own name): each
// belongs to one tariff, and only the subscribers of that tariff get its servers — after
// the panel's own nodes, each feed in the group of its name.
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { RefreshCw, Pencil, Trash2 } from "lucide-react";
import { useEffect, useState, type FormEvent } from "react";
import { api, errorText, unwrap, type Schemas } from "../../api/client";
import { qk, useExternalSubs, useTariffs } from "../../api/hooks";
import { Confirm, Drawer, DrawerFooter } from "../../components/overlay";
import { QueryBoundary } from "../../components/query";
import { useToast } from "../../components/toast";
import { Field, Pill, Skeleton } from "../../components/ui";
import { Switch } from "../../components/switch";
import { t } from "../../i18n";

type ExternalSub = Schemas["ExternalSubView"];
type Tariff = Schemas["TariffView"];

/** The card in Settings → System: the feeds, each tied to the tariff it serves. */
export function ExternalSubsCard() {
  const subs = useExternalSubs();
  const tariffs = useTariffs();
  const qc = useQueryClient();
  const toast = useToast();
  const [creating, setCreating] = useState(false);
  const [edit, setEdit] = useState<ExternalSub | null>(null);
  const [remove, setRemove] = useState<ExternalSub | null>(null);
  const fetch = useMutation({
    mutationFn: (id: number) => unwrap(api.POST("/api/v1/external-subscriptions/{id}/fetch", { params: { path: { id } } })),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: qk.externalSubs });
      toast.ok(t("external.fetched"));
    },
    onError: (e) => toast.error(errorText(e)),
  });

  const tariffName = (id: number) => tariffs.data?.find((x) => x.id === id)?.name ?? t("external.tariffGone");

  return (
    <section className="card glass reveal" style={{ "--i": 7 } as React.CSSProperties}>
      <div className="card-head">
        <div>
          <h2 className="card-title">{t("external.title")}</h2>
          <div className="card-sub">{t("external.sub")}</div>
        </div>
        <button type="button" className="btn btn-glass btn-sm" onClick={() => setCreating(true)}>
          {t("external.add")}
        </button>
      </div>
      <QueryBoundary query={subs} pending={<Skeleton style={{ height: 64 }} />}>
        {(list) =>
          list.length === 0 ? (
            <p className="text-xs text-[var(--ink-500)]">{t("external.none")}</p>
          ) : (
            <ul className="row-list">
              {list.map((s) => (
                <li key={s.id} className="flex flex-wrap items-center justify-between gap-3 py-3">
                  <div className="min-w-0">
                    <div className="text-[13px] font-semibold">{s.name}</div>
                    <div className="text-xs text-[var(--ink-500)]">
                      {tariffName(s.tariff_id)} ·{" "}
                      {s.last_fetch == null
                        ? t("external.neverFetched")
                        : s.last_error
                          ? t("external.fetchError")
                          : t("external.serversN", { n: s.last_count })}
                    </div>
                  </div>
                  <div className="flex shrink-0 items-center gap-2">
                    {s.enabled ? <Pill tone="ok">{t("external.on")}</Pill> : <Pill tone="off">{t("external.off")}</Pill>}
                    <button type="button" className="btn btn-ghost btn-icon" title={t("external.fetchNow")} onClick={() => fetch.mutate(s.id)}>
                      <RefreshCw size={16} aria-hidden />
                    </button>
                    <button type="button" className="btn btn-ghost btn-icon" title={t("external.edit")} onClick={() => setEdit(s)}>
                      <Pencil size={16} aria-hidden />
                    </button>
                    <button type="button" className="btn btn-ghost btn-icon text-[var(--bad)]" onClick={() => setRemove(s)}>
                      <Trash2 size={16} aria-hidden />
                    </button>
                  </div>
                </li>
              ))}
            </ul>
          )
        }
      </QueryBoundary>
      {creating ? <ExternalSubDrawer tariffs={tariffs.data} onClose={() => setCreating(false)} /> : null}
      {edit ? <ExternalSubDrawer tariffs={tariffs.data} sub={edit} onClose={() => setEdit(null)} /> : null}
      <Confirm
        open={remove != null}
        title={t("external.removeTitle")}
        text={remove ? t("external.removeBody", { name: remove.name }) : ""}
        confirm={t("common.delete")}
        danger
        onOpenChange={(open) => {
          if (!open) setRemove(null);
        }}
        onConfirm={() => {
          if (!remove) return;
          void unwrap(api.DELETE("/api/v1/external-subscriptions/{id}", { params: { path: { id: remove.id } } }))
            .then(() => {
              void qc.invalidateQueries({ queryKey: qk.externalSubs });
              setRemove(null);
              toast.ok(t("external.removed"));
            })
            .catch((e) => toast.error(errorText(e)));
        }}
      />
    </section>
  );
}

/** The drawer that adds or edits one feed: the tariff it belongs to, a name and the link. */
function ExternalSubDrawer({ sub, tariffs, onClose }: { sub?: ExternalSub; tariffs?: Tariff[]; onClose: () => void }) {
  const editing = sub != null;
  const toast = useToast();
  const qc = useQueryClient();
  const [name, setName] = useState(sub?.name ?? "");
  const [url, setUrl] = useState(sub?.url ?? "");
  const [tariffId, setTariffId] = useState<number | "">(sub?.tariff_id ?? "");
  const [enabled, setEnabled] = useState(sub?.enabled ?? true);
  const [interval, setIntervalSec] = useState<number>(sub?.interval_sec ?? 0);
  useEffect(() => {
    setName(sub?.name ?? "");
    setUrl(sub?.url ?? "");
    setTariffId(sub?.tariff_id ?? "");
    setEnabled(sub?.enabled ?? true);
    setIntervalSec(sub?.interval_sec ?? 0);
  }, [sub]);
  const save = useMutation({
    mutationFn: () => {
      if (editing) {
        return unwrap(api.PATCH("/api/v1/external-subscriptions/{id}", {
          params: { path: { id: sub.id } },
          body: { name, url, enabled, interval_sec: interval },
        }));
      }
      return unwrap(api.POST("/api/v1/external-subscriptions", {
        body: { name, url, tariff_id: tariffId as number, enabled },
      }));
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: qk.externalSubs });
      toast.ok(t("external.saved"));
      onClose();
    },
    onError: (e) => toast.error(errorText(e)),
  });
  const submit = (e: FormEvent) => {
    e.preventDefault();
    save.mutate();
  };
  return (
    <Drawer
      open
      onOpenChange={(v) => !v && onClose()}
      title={editing ? t("external.editTitle") : t("external.addTitle")}
      footer={
        <DrawerFooter form="external-form" onCancel={onClose} loading={save.isPending} disabled={!name || !url || (!editing && !tariffId)}>
          {t("common.save")}
        </DrawerFooter>
      }
    >
      <form id="external-form" onSubmit={submit} className="pt-5" noValidate>
        <Field label={t("external.tariff")} htmlFor="x-tariff" hint={t("external.tariffHint")}>
          <select id="x-tariff" className="input" value={tariffId} disabled={editing} onChange={(e) => setTariffId(e.target.value ? Number(e.target.value) : "")}>
            <option value="">{t("external.pickTariff")}</option>
            {(tariffs ?? []).map((x) => (
              <option key={x.id} value={x.id}>{x.name}</option>
            ))}
          </select>
        </Field>
        <Field label={t("external.name")} htmlFor="x-name" hint={t("external.nameHint")}>
          <input id="x-name" className="input" value={name} maxLength={200} onChange={(e) => setName(e.target.value)} />
        </Field>
        <Field label={t("external.url")} htmlFor="x-url" hint={t("external.urlHint")}>
          <input id="x-url" className="input" type="url" value={url} maxLength={1000} onChange={(e) => setUrl(e.target.value)} />
        </Field>
        <Field label={t("external.interval")} htmlFor="x-interval" hint={t("external.intervalHint")}>
          <input id="x-interval" className="input" type="number" min={0} max={604800} step={60} value={interval} onChange={(e) => setIntervalSec(Number(e.target.value))} />
        </Field>
        <div className="flex items-center justify-between py-2">
          <span className="text-[13px]">{t("external.enabled")}</span>
          <Switch checked={enabled} onChange={setEnabled} label={t("external.enabled")} />
        </div>
      </form>
    </Drawer>
  );
}