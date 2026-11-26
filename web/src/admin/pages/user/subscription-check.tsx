import { skipToken, useQuery } from "@tanstack/react-query";
import { ListChecks } from "lucide-react";
import { useId, useState, type FormEvent } from "react";
import { api, unwrap, type Schemas } from "../../../api/client";
import type { operations } from "../../../api/schema";
import { QueryBoundary } from "../../../components/query";
import { Button, Field, Pill, Skeleton, StatePill } from "../../../components/ui";
import { t } from "../../../i18n";

type Request = operations["check-user-subscription"]["parameters"]["query"];
type Report = Schemas["SubscriptionCheck"];

export function SubscriptionCheck({ userID }: { userID: number }) {
  const id = useId();
  const [open, setOpen] = useState(false);
  return (
    <div className="mt-4">
      <Button size="sm" aria-expanded={open} aria-controls={id} onClick={() => setOpen((v) => !v)}>
        <ListChecks size={16} aria-hidden /> {t(open ? "subscriptionCheck.close" : "subscriptionCheck.open")}
      </Button>
      <div id={id}>{open ? <CheckForm userID={userID} /> : null}</div>
    </div>
  );
}

function CheckForm({ userID }: { userID: number }) {
  const id = useId();
  const [app, setApp] = useState<Request["app"]>("happ");
  const [version, setVersion] = useState("");
  const [request, setRequest] = useState<Request | null>(null);
  const check = useQuery({
    queryKey: ["user-subscription-check", userID, request],
    queryFn: request
      ? ({ signal }) => unwrap(api.GET("/api/v1/users/{id}/subscription-check", { params: { path: { id: userID }, query: request }, signal }))
      : skipToken,
    refetchOnWindowFocus: false,
    retry: false,
  });
  const run = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const mihomo_version = app === "clash" && version ? version : undefined;
    if (request?.app === app && request.mihomo_version === mihomo_version) void check.refetch();
    else setRequest({ app, mihomo_version });
  };
  return (
    <div className="mt-3 space-y-3">
      <p className="text-xs text-[var(--ink-500)]">{t("subscriptionCheck.hint")}</p>
      <form className="flex flex-wrap items-start gap-3" onSubmit={run}>
        <Field label={t("subscriptionCheck.app")} htmlFor={`${id}-app`}>
          <select
            id={`${id}-app`}
            className="input"
            value={app}
            onChange={(e) => {
              setApp(e.target.value === "clash" ? "clash" : "happ");
              setRequest(null);
            }}
          >
            <option value="happ">Happ</option>
            <option value="clash">Clash (mihomo)</option>
          </select>
        </Field>
        {app === "clash" ? (
          <Field label={t("subscriptionCheck.version")} htmlFor={`${id}-version`} hint={t("subscriptionCheck.versionHint")}>
            <input
              id={`${id}-version`}
              className="input max-w-[180px]"
              type="text"
              pattern="v?[0-9]{1,4}[.][0-9]{1,4}[.][0-9]{1,4}"
              maxLength={15}
              placeholder="1.19.34"
              autoComplete="off"
              value={version}
              onChange={(e) => {
                setVersion(e.target.value);
                setRequest(null);
              }}
            />
          </Field>
        ) : null}
        <Button variant="primary" size="sm" type="submit" className="mt-5" loading={check.isFetching}>
          {t("subscriptionCheck.run")}
        </Button>
      </form>
      {request ? (
        <div aria-busy={check.isFetching}>
          <QueryBoundary query={check} pending={<Skeleton style={{ height: 88, borderRadius: 14 }} />}>
            {(report) => <CheckResults report={report} />}
          </QueryBoundary>
        </div>
      ) : null}
    </div>
  );
}

function CheckResults({ report }: { report: Report }) {
  const nodes = report.nodes.filter((n) => !n.reason).length;
  const inbounds = report.nodes.reduce((n, node) => n + node.inbounds.filter((inbound) => !inbound.reason).length, 0);
  return (
    <div className="space-y-3">
      <div role="status" className="text-xs font-medium text-[var(--ink-600)]">
        {t("subscriptionCheck.summary", { nodes, total: report.nodes.length, inbounds })}
      </div>
      {report.state !== "active" && report.state !== "expiring" ? (
        <div className="banner warn flex-wrap">
          <StatePill state={report.state} />
          <span>{t("subscriptionCheck.accessLimited")}</span>
        </div>
      ) : null}
      {report.template_fallback ? <div className="banner warn">{t("subscriptionCheck.templateFallback")}</div> : null}
      {report.nodes.length === 0 ? (
        <p className="text-xs text-[var(--ink-500)]">{t("subscriptionCheck.noNodes")}</p>
      ) : (
        <ul className="flex flex-col gap-2">
          {report.nodes.map((node) => (
            <li key={node.id} className="panel-soft rounded-xl p-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <span className="min-w-0 break-words text-sm font-medium">{node.name || `#${node.id}`}</span>
                <Pill tone={node.reason ? "off" : "ok"}>{t(node.reason ? "subscriptionCheck.excluded" : "subscriptionCheck.included")}</Pill>
              </div>
              {node.reason ? <p className="mt-1 text-xs text-[var(--ink-600)]">{t(`subscriptionCheck.reasons.${node.reason}`)}</p> : null}
              {node.inbounds.length > 0 ? (
                <ul className="row-list mt-2 text-[13px]">
                  {node.inbounds.map((inbound) => (
                    <li key={inbound.id} className="flex items-start justify-between gap-2 py-2">
                      <div className="min-w-0 break-words">
                        <span className="font-medium">{inbound.name || `#${inbound.id}`}</span>
                        {inbound.protocol ? <span className="ml-2 text-xs text-[var(--ink-500)]">{inbound.protocol}</span> : null}
                        {inbound.reason ? (
                          <p className="text-xs text-[var(--ink-600)]">{t(`subscriptionCheck.reasons.${inbound.reason}`, { version: inbound.minimum_mihomo ?? "" })}</p>
                        ) : inbound.profile_name && inbound.profile_name !== inbound.name ? (
                          <p className="text-xs text-[var(--ink-600)]">{t("subscriptionCheck.profileName", { name: inbound.profile_name })}</p>
                        ) : null}
                      </div>
                      <Pill tone={inbound.reason ? "off" : "ok"}>{t(inbound.reason ? "subscriptionCheck.excluded" : "subscriptionCheck.included")}</Pill>
                    </li>
                  ))}
                </ul>
              ) : null}
            </li>
          ))}
        </ul>
      )}
      {report.external_included > 0 ? <p className="text-xs text-[var(--ink-600)]">{t("subscriptionCheck.external", { n: report.external_included })}</p> : null}
    </div>
  );
}
