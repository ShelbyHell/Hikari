<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset=".github/assets/banner-en-dark.svg">
  <img alt="Hikari: پنل VPN روی هستهٔ mihomo" src=".github/assets/banner-en-light.svg" width="100%">
</picture>

<br>

<div dir="rtl">

**یک پنل VPN سریع و زیبا روی هستهٔ [mihomo](https://github.com/MetaCubeX/mihomo): با یک دستور نصب می‌شود و نیازی به مراقبت ندارد.**

</div>

[![Release](https://img.shields.io/github/v/release/ShelbyHell/Hikari?color=f07a2e&label=release&style=flat-square)](https://github.com/ShelbyHell/Hikari/releases)
[![Image](https://img.shields.io/badge/ghcr.io-shelbyhell%2Fhikari-2b8c9e?style=flat-square&logo=docker&logoColor=white)](https://github.com/ShelbyHell/Hikari/pkgs/container/hikari)
[![License](https://img.shields.io/badge/license-GPL--3.0-2f9e6b?style=flat-square)](LICENSE)
[![mihomo](https://img.shields.io/badge/core-mihomo%201.19.32-9e8eff?style=flat-square)](https://github.com/MetaCubeX/mihomo)

[English](README.md) · [Русский](README.ru.md) · [简体中文](README.zh-CN.md) · **فارسی** · [Türkçe](README.tr.md) · [Español](README.es.md)

</div>

---

> [!IMPORTANT]
> **یک فورک غیررسمی.** Hikari وابسته به پروژهٔ mikan نیست و توسط آن پشتیبانی نمی‌شود. پشتیبانی تنها از سوی نویسندهٔ فورک است ([ShelbyHell/Hikari](https://github.com/ShelbyHell/Hikari)). mikan اصلی در [Miroshka000/mikan](https://github.com/Miroshka000/mikan) است.

<div dir="rtl">

## نصب

روی یک سرور تازهٔ Ubuntu 22.04 یا بالاتر، یا Debian 12 یا بالاتر (با معماری amd64 یا arm64):

</div>

```bash
curl -fsSL https://github.com/ShelbyHell/Hikari/releases/latest/download/install.sh | sudo bash
```

<div dir="rtl">

نصب‌کننده زبان پنل و یک دامنهٔ اختیاری را می‌پرسد، بررسی می‌کند که دامنه به سرور اشاره کند، در صورت نیاز Docker را نصب می‌کند، نزدیک سرور شما سایت‌های استتار REALITY را انتخاب می‌کند و لینک مدیر شما را چاپ می‌کند. هر زمان `hikari` را دوباره اجرا کنید تا منوی مدیریت باز شود.

برای افزودن نود به یک پنل موجود: صفحهٔ **نودها** در پنل (یا `hikari node add`) دستور را همراه با کلیدش می‌دهد:

</div>

```bash
curl -fsSL https://github.com/ShelbyHell/Hikari/releases/latest/download/install.sh | sudo bash -s -- --join KEY
```

<div dir="rtl">

بدون پرسش، برای اسکریپت‌ها: `… | sudo bash -s -- --yes --lang en --domain vpn.example.com --email you@example.com` (همهٔ پرچم‌ها: `hikari install --help`).

## چرا Hikari

</div>

<table dir="rtl">
<tr>
<td width="50%" valign="top">

### 🛡️ خودش در برابر مسدودسازی دوام می‌آورد
Hikari بررسی می‌کند که کلاینت‌های واقعی هنوز به هر پروتکل می‌رسند یا نه. وقتی پورتی در مسیر مسدود شود، پروتکل را به یک پورت HTTPS آزاد منتقل می‌کند؛ وقتی یک سایت استتار REALITY از کار بیفتد، سایت جدیدی نزدیک سرور شما انتخاب می‌کند.

</td>
<td width="50%" valign="top">

### 📱 هر برنامه همان چیزی را می‌گیرد که می‌تواند استفاده کند
اشتراک‌ها برنامه را تشخیص می‌دهند (Happ، v2RayTun، Koala Clash، SlothClash، Clash Verge، FlClash، ClashFest، Hiddify، Shadowrocket و غیره) و فقط پروتکل‌هایی را به آن می‌دهند که واقعاً پشتیبانی می‌کند. دیگر خبری از «روی من کار نمی‌کند» نیست.

</td>
</tr>
<tr>
<td valign="top">

### 📊 حسابداری در حد بایت
mihomo به‌صورت کتابخانه تعبیه شده است، پس ترافیک در هر اتصال و برای هر کاربر شمرده می‌شود، نه نمونه‌برداری. پلن‌ها بر اساس زمان و گیگابایت، روزهای صورتحساب، محدودیت دستگاه و اتصال به دستگاه برای جلوگیری از اشتراک‌گذاری کلید.

</td>
<td valign="top">

### 🤖 ربات Telegram و Mini App داخلی
مشترکان پلن، دستگاه‌ها و راهنمای اتصال خود را در رباتی که در پنل راه‌اندازی می‌کنید می‌بینند، صفحهٔ اشتراک را به‌صورت Telegram Mini App باز می‌کنند و پیش از پایان اشتراک اعلان می‌گیرند. می‌توانند خرید و تمدید هم بکنند: Telegram Stars، کارت و SBP از طریق YooKassa، یا ارز دیجیتال از طریق CryptoBot. پنل خودش لینک را تحویل می‌دهد. ارسال همگانی از محدودیت‌های Telegram پیروی می‌کند.

</td>
</tr>
<tr>
<td valign="top">

### 🪶 سبک مثل یک پر
Go و PostgreSQL، سه کانتینر کوچک. روی یک سرور فعال با کلاینت: حدود **14 MB** رم برای پنل، حدود **50 MB** برای پایگاه داده و حدود **40 MB** برای هستهٔ VPN.

</td>
<td valign="top">

### 🔒 به‌طور پیش‌فرض قفل‌شده
لینک مخفی مدیر روی یک پورت تصادفی، HTTPS با Let's Encrypt (حتی برای IP خالی)، argon2id، احراز هویت دومرحله‌ای TOTP، حفاظت CSRF، CSP سخت‌گیرانه، گزارش حسابرسی. کانتینرها بدون root و فقط‌خواندنی اجرا می‌شوند و همهٔ capabilityها حذف شده‌اند.

</td>
</tr>
<tr>
<td valign="top">

### 🌐 WARP و آبشار سرورها
هر نود می‌تواند WARP خودش را داشته باشد، با یک کلیک یا از روی پیکربندی WireGuard شما ثبت می‌شود، و پروتکل‌ها می‌توانند از نود دیگری از همان پنل خارج شوند (کلاینت ← نود A ← نود B ← اینترنت). پروتکل‌ها، دامنه‌ها و شبکه‌های انتخاب‌شده از این مسیرها می‌روند و بقیه مستقیم می‌روند.

</td>
<td valign="top">

### 🧩 API و کلید برای یکپارچه‌سازی
یک REST API که مستندات آن مستقیم در پنل است، با کلیدهای فقط‌خواندنی یا دسترسی کامل برای ربات‌ها، صورتحساب و پایش.

</td>
</tr>
</table>

<div dir="rtl">

## کدهای تبلیغاتی

مدیران می‌توانند کدهای تخفیف و کدهای روز یا ترافیک هدیه بسازند. مشترکان کدها را در Telegram Mini App وارد می‌کنند که تاریخچهٔ استفاده از کدها را هم نشان می‌دهد. ترافیک هدیه می‌تواند به موجودی اصلی یا یک استخر انتخاب‌شده برود. فاکتورهای تخفیف‌دار فقط با روش‌های پرداختی ممکن‌اند که بازپرداخت را پشتیبانی می‌کنند: Telegram Stars و آداپتورهایی که پشتیبانی از بازپرداخت را اعلام می‌کنند. به این ترتیب اگر رزرو کد پیش از تأیید ارائه‌دهندهٔ پرداخت منقضی شود، Hikari می‌تواند پرداخت را برگرداند.

## دسترسی پشتیبانی

اگر به کمک نیاز دارید، خودتان دسترسی موقت را باز می‌کنید: تنظیمات ← پشتیبانی، برای ۲، ۶ یا ۲۴ ساعت، با سطح «مشاهده»، «تغییر» یا «ترمینال». بدون این دکمه هیچ‌کس، حتی توسعه‌دهندگان Hikari، وارد پنل نمی‌شود: پروژه دسترسی دائمی به سرورهای شما ندارد و پنل خودش به جایی وصل نمی‌شود. هر کار اپراتور در گزارش ممیزی ثبت می‌شود، دسترسی با یک دکمه لغو می‌شود و ترمینال فقط با رمز عبور و 2FA باز می‌شود و ضبط می‌شود. برای خاموش کردن کامل این قابلیت، از کلید همان کارت استفاده کنید یا روی سرور <span dir="ltr">`hikari support-access off`</span> را اجرا کنید. قوانین پشتیبانی پولی: [در سایت](https://shelbyhell.github.io/Hikari/support/paid/).

## پروتکل‌ها

هفده پروتکل، هرکدام یک پیش‌تنظیم با کلیدهایی که برایتان ساخته می‌شود. اشتراک به هر برنامه فقط آنچه را می‌دهد که اجرا می‌کند:

</div>

| پروتکل | برنامه‌های Clash<br><sub>هستهٔ mihomo</sub> | برنامه‌های Xray<br><sub>Happ, v2RayTun</sub> | برنامه‌های sing-box<br><sub>Hiddify, Karing</sub> |
|---|:---:|:---:|:---:|
| VLESS · REALITY · Vision | ✅ | ✅ | ✅ |
| VLESS · REALITY · XHTTP | ✅ | ✅ | — |
| VLESS · REALITY · gRPC | ✅ | ✅ | ✅ |
| VLESS PQ (رمزنگاری پساکوانتومی) | ✅ | ✅ | — |
| Trojan · REALITY | ✅ | ✅ | ✅ |
| VLESS · TLS · XHTTP | ✅ | ✅ | — |
| VLESS · TLS · Vision | ✅ | ✅ | ✅ |
| Hysteria2 | ✅ | ✅ | ✅ |
| TUIC v5 | ✅ | — | ✅ |
| AnyTLS | ✅ | — | ✅ |
| TrustTunnel | ✅ | — | — |
| ShadowQUIC | ✅ | — | — |
| Mieru | ✅ | — | — |
| Shadowsocks-2022 ¹ | ✅ | ✅ | ✅ |
| Sudoku ¹ | ✅ | — | — |
| Snell ¹ | ✅ | — | — |
| VMess (پیکربندی سفارشی) | ✅ | ✅ | ✅ |

<div dir="rtl">

<sub>¹ در mihomo برای همهٔ کاربران فقط یک کلید وجود دارد: در این پروتکل‌ها حسابداری و محدودیت برای هر کاربر ممکن نیست. پروتکل‌های جدیدتر فقط وقتی به برنامه‌های Clash فرستاده می‌شوند که هسته‌شان به‌اندازهٔ کافی جدید باشد.</sub>

## نحوهٔ کار

</div>

```mermaid
flowchart LR
    A[Browser] -- HTTPS, secret link --> P[Hikari panel<br>users · plans · subscriptions]
    P -- unix socket / pinned TLS --> N1[hikari node<br>mihomo inside]
    P -- pinned TLS --> N2[remote node]
    C[VPN apps] -- REALITY · QUIC · … --> N1
    C --> N2
    C -. subscription .-> P
```

<div dir="rtl">

پنل و نود دو فرایند از یک ایمیج‌اند. پنل را به‌روزرسانی یا راه‌اندازی مجدد کنید، VPN به کار خود ادامه می‌دهد. نودهای راه‌دور را با کلید عضویت از صفحهٔ **نودها** اضافه کنید.

## مدیریت سرور

برای منو `hikari` را اجرا کنید، یا دستورها را مستقیم به کار ببرید:

| دستور | کار |
|---|---|
| <span dir="ltr">`hikari status`</span> | کانتینرها، نسخه‌ها، وضعیت سلامت، به‌روزرسانی موجود |
| <span dir="ltr">`hikari logs [panel\|node]`</span> | لاگ‌های زنده |
| <span dir="ltr">`hikari url`</span> | لینک مدیر |
| <span dir="ltr">`hikari reset-password` · `reset-path` · `disable-2fa`</span> | بازیابی دسترسی |
| <span dir="ltr">`hikari update`</span> | به‌روزرسانی همین حالا (اول پشتیبان، بازگشت خودکار) |
| <span dir="ltr">`hikari backup` · `restore FILE`</span> | پشتیبان‌ها در <span dir="ltr">`/opt/mikan/backups`</span> |
| <span dir="ltr">`hikari node …` · `hikari inbound …`</span> | نودها و پروتکل‌ها |
| <span dir="ltr">`hikari targets scan` · `apply`</span> | سایت‌های استتار REALITY نزدیک سرور |
| <span dir="ltr">`hikari join KEY`</span> | روی یک نود: گرفتن کلید جدید از پنل |
| <span dir="ltr">`hikari restart`</span> | راه‌اندازی مجدد کانتینرها |
| <span dir="ltr">`hikari doctor`</span> | بررسی سرور: Docker، پورت‌ها، فایروال، ساعت، DNS، دیسک |
| <span dir="ltr">`hikari support CODE`</span> | ارسال عیب‌یابی به تیکت پشتیبانی وقتی پنل در دسترس نیست |
| <span dir="ltr">`hikari support-access on\|off`</span> | اجازه یا منع دسترسی پشتیبانی روی این سرور |
| <span dir="ltr">`hikari uninstall`</span> | توقف و حذف دستور، داده‌ها می‌مانند |

## به‌روزرسانی‌ها

پنل روزی یک بار نسخهٔ جدید را بررسی می‌کند و تغییرات را نشان می‌دهد. **به‌روزرسانی خودکار** را در تنظیمات روشن کنید یا **به‌روزرسانی** را بزنید: به‌روزرسان روی میزبان ایمیج را از GitHub Packages می‌کشد، پشتیبان می‌گیرد، پایگاه داده را مهاجرت می‌دهد، راه‌اندازی مجدد می‌کند و اگر نسخهٔ جدید سالم بالا نیاید، برمی‌گردد. مانیفست هر نسخه امضا شده است.

اگر روی 0.4.4 یا قدیمی‌تر هستید، ابتدا به 0.4.5 به‌روزرسانی کنید؛ گذار به 0.5 و PostgreSQL هم بعد از آن از طریق همان دکمهٔ به‌روزرسانی انجام می‌شود. اگر 0.4.5 را رد کرده‌اید، دستور نصب بالا یک سرور موجود را به‌روزرسانی می‌کند.

## ساخت از سورس

</div>

```bash
docker buildx build -t Hikari:dev .            # panel + node image
cd web && pnpm install && pnpm dev            # admin UI with hot reload
MIKAN_TEST_DATABASE_URL=postgres://… go test ./...   # Go 1.27, a test PostgreSQL 18 and pg_dump
cd installer && cargo test                    # the installer (Rust)
```

<div dir="rtl">

توسعه روی شاخهٔ `dev` انجام می‌شود؛ pull requestهایی که به `main` می‌روند ساخته و تست می‌شوند و نسخه‌ها از روی تگ‌ها منتشر می‌شوند.

## ⛔ فورک‌ها و بیلدهای شخص ثالث

> [!CAUTION]
> **mikan رسمی فقط اینجاست:** مخزن `Miroshka000/mikan`، ایمیج `ghcr.io/miroshka000/mikan` و نسخه‌هایی که با کلید mikan امضا شده‌اند. افزونه‌های رسمی فقط از کاتالوگ `getmikan/marketplace` می‌آیند؛ مخزن جامعه `getmikan/marketplace-community` افزونه‌های شخص ثالث را فهرست می‌کند که mikan آن‌ها را بررسی نمی‌کند و مسئولیتی در قبالشان ندارد. هر چیز دیگری، با هر نامی، mikan نیست.
>
> **پروژه mikan هر فورک و هر بیلدی از منابع دیگر را بالقوه ناامن می‌داند.**
>
> **بیلدهای رسمی قابل بررسی‌اند.** هر نسخه به‌طور خودکار از کد باز همین مخزن ساخته و امضا می‌شود و پنل و نصب‌کننده فقط به‌روزرسانی‌هایی با این امضا را می‌پذیرند. اینکه بیلد دیگران از چه ساخته شده، فقط سازنده‌اش می‌داند.
>
> **فورک به‌روزرسانی‌های ما را دریافت نمی‌کند.** فورک نمی‌تواند نسخه‌هایش را با کلید mikan امضا کند، پس رفع اشکال‌ها و آسیب‌پذیری‌ها به آن نمی‌رسد، مگر اینکه سازنده‌اش آن‌ها را منتقل کند.
>
> **ما از فورک‌ها پشتیبانی نمی‌کنیم.** آن‌ها را بررسی نمی‌کنیم و مسئول اشکال‌ها، آسیب‌پذیری‌ها یا از دست رفتن داده‌ها در آن‌ها نیستیم. پرسش‌ها درباره بیلدهای دیگران در چت و issues بررسی نمی‌شوند.
>
> **نام بخشی از مجوز نیست.** GPL v3 کد را پوشش می‌دهد، نه نام mikan یا لوگوی آن را (بخش ۷ GPL v3، ببینید [NOTICE.md](NOTICE.md) و [TRADEMARK.md](TRADEMARK.md)). فورک باید نام دیگری داشته باشد و خود را mikan معرفی نکند.

## مجوز

Hikari فورکی از mikan است، نرم‌افزار آزاد تحت [GNU GPL v3](LICENSE) با شرایط تکمیلی [NOTICE.md](NOTICE.md) است: حفظ نام سازنده، معرفی نکردن نسخه تغییریافته به‌عنوان نسخه اصلی، و عدم اعطای حق بر نام و لوگو. Copyright (C) 2026 Miroshka and Mikan. [mihomo](https://github.com/MetaCubeX/mihomo) (GPL-3.0) در آن تعبیه شده و بخش‌هایی از [3x-ui](https://github.com/MHSanaei/3x-ui) (GPL-3.0) در آن اقتباس شده است. فونت‌ها: Unbounded، Onest، JetBrains Mono (SIL OFL 1.1).

</div>

<div align="center">
<sub>Hikari — یک فورک غیررسمی از mikan</sub>
</div>
