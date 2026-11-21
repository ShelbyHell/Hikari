import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useSearch } from "@tanstack/react-router";
import { LoaderCircle, ShieldCheck } from "lucide-react";
import { motion } from "motion/react";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { api, ApiError, basePath, errorText, setCsrf, unwrap } from "../../api/client";
import { qk } from "../../api/hooks";
import { Logo } from "../../components/atmosphere";
import { LangSwitch } from "../../components/lang";
import { Button, Field, QR } from "../../components/ui";
import { t } from "../../i18n";
import { localPath } from "../../lib/url";

export function TelegramIcon({ size = 18 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" aria-hidden>
      <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm4.64 6.8c-.15 1.58-.8 5.42-1.13 7.19-.14.75-.42 1-.68 1.03-.58.05-1.02-.38-1.58-.75-.88-.58-1.38-.94-2.23-1.5-.99-.65-.35-1.01.22-1.59.15-.15 2.71-2.48 2.76-2.69.01-.03.01-.14-.07-.19-.08-.05-.19-.02-.27 0-.12.03-1.99 1.27-5.63 3.73-.53.37-1.02.55-1.45.54-.48-.01-1.4-.27-2.09-.49-.84-.27-1.51-.42-1.45-.89.03-.25.38-.51 1.05-.78 4.12-1.79 6.87-2.98 8.24-3.55 3.92-1.63 4.74-1.92 5.26-1.93.12 0 .37.03.54.17.14.12.18.28.2.45-.02.07-.02.19-.04.31z" />
    </svg>
  );
}

export function GitHubIcon({ size = 18 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" aria-hidden>
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.53 1.032 1.53 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z"
      />
    </svg>
  );
}

export function LoginPage() {
  const { next, error: searchError } = useSearch({ from: "/login" });
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [step, setStep] = useState<"password" | "totp" | "telegram">("password");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [code, setCode] = useState("");
  const [error, setError] = useState("");
  const [wait, setWait] = useState(0);
  const [tgData, setTgData] = useState<{ token: string; link: string } | null>(null);
  const codeRef = useRef<HTMLInputElement>(null);

  const { data: providers } = useQuery({
    queryKey: qk.authProviders,
    queryFn: () => unwrap(api.GET("/api/v1/auth/providers")),
  });

  useEffect(() => {
    if (searchError) {
      if (searchError === "github_user_not_allowed") {
        setError(t("login.githubNotAllowed"));
      } else if (searchError === "oauth_state_invalid") {
        setError(t("login.oauthStateInvalid"));
      } else if (searchError === "oauth_token_failed") {
        setError(t("login.oauthTokenFailed"));
      } else if (searchError === "oauth_fetch_user_failed") {
        setError(t("login.oauthUserFailed"));
      } else {
        setError(searchError);
      }
    }
  }, [searchError]);

  useEffect(() => {
    if (wait <= 0) return;
    const timer = window.setTimeout(() => setWait((w) => w - 1), 1000);
    return () => window.clearTimeout(timer);
  }, [wait]);

  useEffect(() => {
    if (step === "totp") codeRef.current?.focus();
  }, [step]);

  const login = useMutation({
    mutationFn: () => unwrap(api.POST("/api/v1/auth/login", { body: { username, password, totp: step === "totp" ? code : undefined } })),
    onSuccess: (me) => {
      setCsrf(me.csrf_token);
      qc.setQueryData(qk.me, me);
      const target = localPath(next);
      void navigate({ to: target });
    },
    onError: (e) => {
      if (e instanceof ApiError && e.status === 401) {
        if (e.detail === "totp_required") {
          setStep("totp");
          setError("");
          return;
        }
        setError(e.detail === "invalid_totp" ? t("login.badCode") : t("login.badCredentials"));
        if (e.detail === "invalid_totp") setCode("");
        return;
      }
      if (e instanceof ApiError && e.status === 429) {
        setWait(e.retryAfter || 60);
        setError("");
        return;
      }
      setError(errorText(e));
    },
  });

  const startTelegram = useMutation({
    mutationFn: () => unwrap(api.POST("/api/v1/auth/telegram/request")),
    onSuccess: (res) => {
      setTgData({ token: res.token, link: res.link });
    },
    onError: (e) => {
      if (e instanceof ApiError && e.status === 429) {
        setWait(e.retryAfter || 60);
        setStep("password");
        return;
      }
      setError(errorText(e));
    },
  });

  // Polling for Telegram approval
  useEffect(() => {
    if (step !== "telegram" || !tgData?.token) return;
    let active = true;

    const timer = setInterval(async () => {
      try {
        const res = await unwrap(api.POST("/api/v1/auth/telegram/check", { body: { token: tgData.token } }));
        if (!active) return;
        if (res.status === "approved" && res.admin && res.csrf_token) {
          clearInterval(timer);
          setCsrf(res.csrf_token);
          qc.setQueryData(qk.me, { admin: res.admin, csrf_token: res.csrf_token });
          const target = localPath(next);
          void navigate({ to: target });
        } else if (res.status === "rejected") {
          clearInterval(timer);
          setError(t("login.telegramRejected"));
        } else if (res.status === "expired") {
          clearInterval(timer);
          setError(t("login.telegramExpired"));
        }
      } catch {
        // Continue polling on transient errors
      }
    }, 2000);

    return () => {
      active = false;
      clearInterval(timer);
    };
  }, [step, tgData?.token, next, navigate, qc]);

  const submit = (ev: FormEvent) => {
    ev.preventDefault();
    if (step === "telegram" || wait > 0 || login.isPending) return;
    login.mutate();
  };

  const hasExternalProviders = Boolean(providers?.telegram.enabled || providers?.github.enabled);

  return (
    <div className="grid min-h-screen place-items-center px-4 py-12">
      <LangSwitch className="fixed top-4 right-4" />
      <motion.form
        onSubmit={submit}
        className="glass-strong w-full max-w-[400px] rounded-[28px] p-8"
        initial={{ opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ type: "spring", stiffness: 300, damping: 30 }}
        noValidate
      >
        <div className="mb-8 flex items-center gap-3">
          <Logo size={40} />
          <div>
            <div className="font-display text-[22px] leading-7 font-semibold tracking-tight">Hikari</div>
            <div className="text-[13px] text-[var(--ink-500)]">{t("login.subtitle")}</div>
          </div>
        </div>

        {step === "password" ? (
          <>
            <Field label={t("login.username")} htmlFor="login-user">
              <input id="login-user" className="input" autoComplete="username" autoCapitalize="none" spellCheck={false} value={username} onChange={(e) => setUsername(e.target.value)} required autoFocus />
            </Field>
            <Field label={t("login.password")} htmlFor="login-pass">
              <input id="login-pass" className="input" type="password" autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} required />
            </Field>
          </>
        ) : step === "totp" ? (
          <>
            <div className="panel-soft mb-4 flex gap-3 p-3 text-[13px] text-[var(--ink-600)]">
              <ShieldCheck size={18} className="mt-0.5 shrink-0 text-[var(--leaf-500)]" aria-hidden />
              {t("login.totpHint")}
            </div>
            <Field label={t("login.code")} htmlFor="login-code">
              <input
                id="login-code"
                ref={codeRef}
                className="input mono text-center text-lg tracking-[0.3em]"
                inputMode="numeric"
                autoComplete="one-time-code"
                maxLength={16}
                value={code}
                onChange={(e) => setCode(e.target.value.trim())}
              />
            </Field>
          </>
        ) : (
          <div className="text-center">
            <h2 className="mb-1 text-base font-semibold text-[var(--ink-800)]">{t("login.telegramLoginTitle")}</h2>
            <p className="mb-5 text-xs text-[var(--ink-500)]">{t("login.telegramLoginHint")}</p>

            {startTelegram.isPending || !tgData ? (
              <div className="flex h-[200px] w-full items-center justify-center">
                <LoaderCircle className="animate-spin text-[var(--mikan-500)]" size={32} />
              </div>
            ) : (
              <div>
                <div className="mx-auto my-3 flex w-fit justify-center rounded-2xl bg-white p-3.5 shadow-sm">
                  <QR value={tgData.link} size={180} />
                </div>
                <a
                  href={tgData.link}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="btn btn-primary mt-4 flex w-full items-center justify-center gap-2"
                >
                  <TelegramIcon size={18} />
                  <span>{t("login.telegramOpenBot")}</span>
                </a>
                <div className="mt-4 flex items-center justify-center gap-2 text-[13px] text-[var(--ink-500)]">
                  <LoaderCircle className="animate-spin" size={14} />
                  <span>{t("login.telegramWaiting")}</span>
                </div>
              </div>
            )}
          </div>
        )}

        {error ? (
          <p className="mt-3 mb-4 text-[13px] text-[var(--berry-600)]" role="alert">
            {error}
          </p>
        ) : null}
        {wait > 0 ? (
          <p className="mt-3 mb-4 text-[13px] text-[var(--honey-600)]" role="alert">
            {t("login.tooMany", { s: wait })}
          </p>
        ) : null}

        {step !== "telegram" ? (
          <Button type="submit" variant="primary" block loading={login.isPending} disabled={wait > 0 || !username || !password || (step === "totp" && code.length < 6)}>
            {step === "password" ? t("login.submit") : t("login.confirm")}
          </Button>
        ) : null}

        {step === "totp" ? (
          <button
            type="button"
            className="link-btn mt-4 block w-full text-center text-[13px]"
            onClick={() => {
              setStep("password");
              setCode("");
              setError("");
            }}
          >
            {t("login.back")}
          </button>
        ) : step === "telegram" ? (
          <button
            type="button"
            className="link-btn mt-4 block w-full text-center text-[13px]"
            onClick={() => {
              setStep("password");
              setError("");
              setTgData(null);
            }}
          >
            {t("login.back")}
          </button>
        ) : (
          <>
            {hasExternalProviders ? (
              <>
                <div className="my-5 flex items-center gap-3">
                  <div className="h-px flex-1 bg-[var(--border-subtle)]" />
                  <span className="text-xs uppercase text-[var(--ink-400)]">{t("login.orDivider")}</span>
                  <div className="h-px flex-1 bg-[var(--border-subtle)]" />
                </div>
                <div className="flex flex-col gap-2.5">
                  {providers?.telegram.enabled ? (
                    <Button
                      type="button"
                      variant="glass"
                      block
                      onClick={() => {
                        setError("");
                        setStep("telegram");
                        startTelegram.mutate();
                      }}
                      className="flex items-center justify-center gap-2"
                    >
                      <TelegramIcon size={18} />
                      <span>{t("login.loginWithTelegram")}</span>
                    </Button>
                  ) : null}
                  {providers?.github.enabled ? (
                    <a
                      href={`${basePath}/api/v1/auth/github/start${next ? "?next=" + encodeURIComponent(next) : ""}`}
                      className="btn btn-glass flex w-full items-center justify-center gap-2"
                    >
                      <GitHubIcon size={18} />
                      <span>{t("login.loginWithGitHub")}</span>
                    </a>
                  ) : null}
                </div>
              </>
            ) : null}
            <p className="mt-6 text-center text-xs text-[var(--ink-500)]">
              {t("login.forgot")} <span className="mono">hikari reset-password</span>
            </p>
          </>
        )}
      </motion.form>
    </div>
  );
}
