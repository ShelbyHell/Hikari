package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"mikan/internal/hostname"
	"mikan/internal/panel/audit"
	"mikan/internal/panel/dnscheck"
	"mikan/internal/panel/domain"
	"mikan/internal/panel/settings"
	"mikan/internal/panel/store"
)

// domainCmd prints, changes or clears the panel's domain, SNI domain, or subscription domain.
func domainCmd(ctx context.Context, st *store.Store, set *settings.Settings, dataDir string, args []string, stdout, stderr io.Writer) error {
	return domainTargetCmd(ctx, st, set, dataDir, args, stdout, stderr, "domain")
}

func sniCmd(ctx context.Context, st *store.Store, set *settings.Settings, dataDir string, args []string, stdout, stderr io.Writer) error {
	return domainTargetCmd(ctx, st, set, dataDir, args, stdout, stderr, "sni")
}

func domainTargetCmd(ctx context.Context, st *store.Store, set *settings.Settings, dataDir string, args []string, stdout, stderr io.Writer, targetKind string) error {
	fs := flag.NewFlagSet(targetKind, flag.ContinueOnError)
	fs.SetOutput(stderr)
	clear := fs.Bool("clear", false, "remove the domain and switch back to default")
	force := fs.Bool("force", false, "skip DNS check")
	isSNI := fs.Bool("sni", false, "operate on the SNI domain")
	isSub := fs.Bool("sub", false, "operate on the subscription domain")
	if err := fs.Parse(args); err != nil {
		return err
	}
	key := settings.KeyDomain
	title := "Panel domain"
	auditAction := "cli.domain"
	if targetKind == "sni" || *isSNI {
		key = settings.KeySNIDomain
		title = "SNI domain"
		auditAction = "cli.sni_domain"
	} else if targetKind == "sub" || *isSub {
		key = settings.KeySubDomain
		title = "Subscription domain"
		auditAction = "cli.sub_domain"
	}

	if !*clear && fs.NArg() > 0 && fs.Arg(0) != "clear" {
		newDomain := strings.TrimSpace(fs.Arg(0))
		if !hostname.Valid(newDomain) {
			return fmt.Errorf("invalid domain %q: want a host name like vpn.example.com", newDomain)
		}
	}
	if set == nil {
		return errors.New("settings store is required")
	}
	cur, err := set.String(ctx, key)
	if err != nil {
		return err
	}
	if *clear || (fs.NArg() > 0 && fs.Arg(0) == "clear") {
		if cur == "" {
			fmt.Fprintf(stdout, "No %s is currently set.\n", strings.ToLower(title))
			return nil
		}
		if err := settings.Set(ctx, set, key, ""); err != nil {
			return err
		}
		_ = audit.Write(ctx, st.Q, time.Now(), audit.Entry{Action: auditAction, Details: map[string]any{"domain": "", "previous": cur}})
		reloadPanel(stdout)
		if key == settings.KeyDomain {
			u, err := panelURL(ctx, set)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "Panel domain removed (was %s).\nNew admin link:\n%s\n", cur, u)
		} else {
			fmt.Fprintf(stdout, "%s removed (was %s).\n", title, cur)
		}
		return nil
	}
	if fs.NArg() == 0 {
		if cur != "" {
			fmt.Fprintf(stdout, "%s: %s\n", title, cur)
		} else {
			fmt.Fprintf(stdout, "No %s set.\n", strings.ToLower(title))
		}
		if key == settings.KeyDomain {
			u, err := panelURL(ctx, set)
			if err == nil {
				fmt.Fprintf(stdout, "Admin link: %s\n", u)
			}
		}
		return nil
	}
	newDomain := strings.TrimSpace(fs.Arg(0))
	if !hostname.Valid(newDomain) {
		return fmt.Errorf("invalid domain %q: want a host name like vpn.example.com", newDomain)
	}
	if newDomain == cur {
		fmt.Fprintf(stdout, "%s is already %s.\n", title, cur)
		return nil
	}
	host, err := set.String(ctx, settings.KeyPublicHost)
	if err != nil {
		return err
	}
	own := dnscheck.Own(host)
	if nodeDNS != nil && !*force {
		if err := domain.DomainHere(ctx, nodeDNS, newDomain, own); err != nil {
			var de *domain.DomainError
			if errors.As(err, &de) {
				return fmt.Errorf("domain check: %s (use -f/--force to skip)", domainText(de))
			}
			return fmt.Errorf("domain check: %w (use -f/--force to skip)", err)
		}
	}
	if err := settings.Set(ctx, set, key, newDomain); err != nil {
		return err
	}
	_ = audit.Write(ctx, st.Q, time.Now(), audit.Entry{Action: auditAction, Details: map[string]any{"domain": newDomain, "previous": cur}})
	reloadPanel(stdout)
	if key == settings.KeyDomain {
		u, err := panelURL(ctx, set)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Panel domain changed to %s.\nNew admin link:\n%s\nLet's Encrypt will issue a certificate for %s within a minute.\n", newDomain, u, newDomain)
	} else {
		fmt.Fprintf(stdout, "%s changed to %s.\nLet's Encrypt will issue a certificate for %s within a minute.\n", title, newDomain, newDomain)
	}
	return nil
}
