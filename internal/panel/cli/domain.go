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

// domainCmd prints, changes or clears the panel's domain.
func domainCmd(ctx context.Context, st *store.Store, set *settings.Settings, dataDir string, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("domain", flag.ContinueOnError)
	fs.SetOutput(stderr)
	clear := fs.Bool("clear", false, "remove the domain and switch back to the public IP")
	if err := fs.Parse(args); err != nil {
		return err
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
	cur, err := set.String(ctx, settings.KeyDomain)
	if err != nil {
		return err
	}
	if *clear || (fs.NArg() > 0 && fs.Arg(0) == "clear") {
		if cur == "" {
			fmt.Fprintln(stdout, "No panel domain is currently set (running on IP address).")
			return nil
		}
		if err := settings.Set(ctx, set, settings.KeyDomain, ""); err != nil {
			return err
		}
		_ = audit.Write(ctx, st.Q, time.Now(), audit.Entry{Action: "cli.domain", Details: map[string]any{"domain": "", "previous": cur}})
		reloadPanel(stdout)
		u, err := panelURL(ctx, set)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Panel domain removed (was %s).\nNew admin link:\n%s\n", cur, u)
		return nil
	}
	if fs.NArg() == 0 {
		if cur != "" {
			fmt.Fprintf(stdout, "Panel domain: %s\n", cur)
		} else {
			fmt.Fprintln(stdout, "No panel domain set (running on IP address).")
		}
		u, err := panelURL(ctx, set)
		if err == nil {
			fmt.Fprintf(stdout, "Admin link: %s\n", u)
		}
		return nil
	}
	newDomain := strings.TrimSpace(fs.Arg(0))
	if !hostname.Valid(newDomain) {
		return fmt.Errorf("invalid domain %q: want a host name like vpn.example.com", newDomain)
	}
	if newDomain == cur {
		fmt.Fprintf(stdout, "Panel domain is already %s.\n", cur)
		u, _ := panelURL(ctx, set)
		if u != "" {
			fmt.Fprintf(stdout, "Admin link: %s\n", u)
		}
		return nil
	}
	host, err := set.String(ctx, settings.KeyPublicHost)
	if err != nil {
		return err
	}
	own := dnscheck.Own(host)
	if nodeDNS != nil {
		if err := domain.DomainHere(ctx, nodeDNS, newDomain, own); err != nil {
			var de *domain.DomainError
			if errors.As(err, &de) {
				return fmt.Errorf("domain check: %s", domainText(de))
			}
			return fmt.Errorf("domain check: %w", err)
		}
	}
	if err := settings.Set(ctx, set, settings.KeyDomain, newDomain); err != nil {
		return err
	}
	_ = audit.Write(ctx, st.Q, time.Now(), audit.Entry{Action: "cli.domain", Details: map[string]any{"domain": newDomain, "previous": cur}})
	reloadPanel(stdout)
	u, err := panelURL(ctx, set)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Panel domain changed to %s.\nNew admin link:\n%s\nLet's Encrypt will issue a certificate for %s within a minute.\n", newDomain, u, newDomain)
	return nil
}
