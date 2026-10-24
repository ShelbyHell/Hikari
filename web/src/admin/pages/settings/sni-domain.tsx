import { useState, type FormEvent } from "react";
import { LoaderCircle, RefreshCw, ShieldCheck } from "lucide-react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import type { Schemas } from "../../../api/client";
import { api, errorText, unwrap } from "../../../api/client";
import { qk } from "../../../api/hooks";
import { AcmeAttempts, AcmeProblem, caName } from "../../../components/acme";
import { CertDrawer, certUntil, type CertInfo } from "../../../components/cert-drawer";
import { FormActions } from "../../../components/layout";
import { useToast } from "../../../components/toast";
import { Button, Field, Pill } from "../../../components/ui";
import { getLocale, t, tMaybe } from "../../../i18n";
import { useDraft } from "../../../lib/draft";
import { fieldErrors } from "../../../lib/fields";
import { useSaveSettings } from "./shared";

// SNI domain: camouflage site and client connections point here.
// The admin panel and subscriptions are not served on this domain (404), keeping them separate.
export function SniDomainCard({ s }: { s: Schemas["SettingsView"] }) {
  const save = useSaveSettings();
  const { draft: domain, setDraft: setDomain } = useDraft(s.sni_domain ?? "");
  const error = fieldErrors(save.error).sni_domain;
  const trimmed = domain.trim();
  const changed = trimmed !== (s.sni_domain ?? "");
  const submit = (e: FormEvent) => {
    e.preventDefault();
    save.mutate({ sni_domain: trimmed });
  };
  return (
    <section className="card glass reveal" style={{ "--i": 1 } as React.CSSProperties}>
      <form onSubmit={submit} noValidate>
        <div className="card-head">
          <div>
            <h2 className="card-title">{t("settings.sniDomain")}</h2>
            <div className="card-sub">{t("settings.sniDomainSub")}</div>
          </div>
          {s.sni_domain ? <Pill tone="ok">{s.sni_domain}</Pill> : null}
        </div>
        <Field label={t("settings.sniDomainField")} htmlFor="s-sni-domain" hint={t("settings.sniDomainHint")} error={error}>
          <input
            id="s-sni-domain"
            className="input mono max-w-[320px]"
            value={domain}
            onChange={(e) => setDomain(e.target.value)}
            placeholder={s.domain || "sni.example.com"}
            aria-invalid={!!error}
          />
        </Field>
        <p className="text-xs text-[var(--ink-500)]">{t("settings.sniDomainNote")}</p>
        <FormActions saving={save.isPending && save.variables?.sni_domain === trimmed} disabled={!changed}>
          {s.sni_domain ? (
            <Button variant="ghost" loading={save.isPending && save.variables?.sni_domain === ""} onClick={() => save.mutate({ sni_domain: "" })}>
              {t("settings.sniDomainOff")}
            </Button>
          ) : null}
        </FormActions>
      </form>
      <SniDomainCert s={s} />
    </section>
  );
}

function SniDomainCert({ s }: { s: Schemas["SettingsView"] }) {
  const qc = useQueryClient();
  const toast = useToast();
  const c = s.sni_certificate;
  if (!c || !s.sni_domain) return null;

  const [outcome, setOutcome] = useState<"got" | "running" | null>(null);
  const renew = useMutation({
    mutationFn: () => unwrap(api.POST("/api/v1/settings/sni-certificate/renew")),
    onMutate: () => setOutcome(null),
    onSuccess: (st) => {
      qc.setQueryData<Schemas["SettingsView"]>(qk.settings, (old) => (old ? { ...old, sni_certificate: st } : old));
      setOutcome(st.ordering ? "running" : st.error ? null : "got");
      if (st.ordering) window.setTimeout(() => void qc.invalidateQueries({ queryKey: qk.settings }), 15_000);
    },
    onError: (e) => toast.error(errorText(e)),
  });

  const [own, setOwn] = useState(false);
  const custom = c.kind === "custom";
  const ok = c.kind === "acme" || custom;
  const until = new Date(c.not_after).toLocaleString(getLocale(), { day: "numeric", month: "long", hour: "2-digit", minute: "2-digit" });
  const sub = custom
    ? t("settings.certCustom", { names: (c.names ?? []).join(", "), until: certUntil(c.not_after) })
    : ok
      ? t("certs.issued", { ca: caName(c.ca), id: c.identifier, until })
      : t("settings.certSelf");

  const current: CertInfo | null = custom
    ? { names: c.names, issuer: c.issuer, not_after: c.not_after, trusted: c.trusted }
    : c.error?.startsWith("custom_")
      ? { error: c.error }
      : null;
  const switching = c.kind === "acme" && !!c.ca && c.ca !== c.ca_wanted;
  const failed = !!c.error && !c.error.startsWith("custom_") && c.error !== "no_public_host";

  return (
    <div className="mt-4 border-t border-[var(--glass-edge)] pt-4">
      <div className="card-head mb-3">
        <div>
          <h3 className="text-sm font-semibold text-[var(--ink-800)]">{t("settings.sniCert")}</h3>
          <div className="text-xs text-[var(--ink-500)]">{sub}</div>
        </div>
        {custom ? <Pill tone="ok">{t("settings.certOwn")}</Pill> : ok ? <Pill tone="ok">{t("settings.certValid")}</Pill> : <Pill tone="warn">{t("settings.certTemp")}</Pill>}
      </div>

      {c.ordering || renew.isPending ? (
        <p className="mb-3 flex items-center gap-2 text-[13px] text-[var(--ink-600)]" role="status">
          <LoaderCircle size={16} className="spin" aria-hidden /> {t("certs.ordering")}
        </p>
      ) : c.error ? (
        c.error.startsWith("custom_") ? (
          <p className="mb-3 text-[13px] text-[var(--berry-600)]" role="alert">
            {tMaybe(`errors.acme.${c.error}`) ?? c.error}
          </p>
        ) : (
          <AcmeProblem code={c.error} detail={c.error_detail} holder={c.holder} retryAt={c.retry_at} host={c.identifier} className="mb-3" />
        )
      ) : outcome ? (
        <p className="mb-3 text-[13px] text-[var(--leaf-700)]" role="status">
          {outcome === "got" ? t("certs.got") : t("certs.stillRunning")}
        </p>
      ) : switching ? (
        <p className="mb-3 text-[13px] text-[var(--ink-600)]" role="status">
          {t("certs.switching", { ca: caName(c.ca_wanted) })}
        </p>
      ) : null}

      <AcmeAttempts attempts={c.attempts} className="mb-3" />

      <div className="flex flex-wrap gap-2">
        {!custom && c.error !== "no_public_host" ? (
          <Button size="sm" loading={renew.isPending} disabled={renew.isPending || c.ordering} onClick={() => renew.mutate()}>
            <RefreshCw size={16} aria-hidden /> {failed ? t("certs.retry") : t("certs.getNow")}
          </Button>
        ) : null}
        <Button size="sm" onClick={() => setOwn(true)}>
          <ShieldCheck size={16} aria-hidden /> {custom ? t("settings.certReplace") : t("settings.certOwnButton")}
        </Button>
      </div>

      <CertDrawer
        open={own}
        onClose={() => setOwn(false)}
        title={t("cert.sniTitle")}
        lead={t("cert.sniLead")}
        current={current}
        save={(cert, key) => unwrap(api.PUT("/api/v1/settings/sni-certificate", { body: { cert, key } })).then((v) => qc.setQueryData(qk.settings, v))}
        clear={() =>
          unwrap(api.DELETE("/api/v1/settings/sni-certificate")).then(() => {
            void qc.invalidateQueries({ queryKey: qk.settings });
            window.setTimeout(() => void qc.invalidateQueries({ queryKey: qk.settings }), 15_000);
          })
        }
        clearLabel={t("cert.sniClear")}
        clearText={t("cert.sniClearText")}
      />
    </div>
  );
}
