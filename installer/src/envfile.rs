//! Configuration and secrets management for Hikari (/opt/mikan).
//! Non-secret configuration is saved to config.json, while secrets
//! (Postgres password, node join key) are saved into /opt/mikan/secrets/
//! with mode 0600. Old .env files are automatically read and migrated.

use std::fs;
use std::io::Write;
use std::os::unix::fs::{DirBuilderExt, OpenOptionsExt, PermissionsExt};
use std::path::{Path, PathBuf};
use std::time::{SystemTime, UNIX_EPOCH};

use anyhow::{Context, Result, bail};

/// Returns true if Hikari configuration exists in root (either config.json or legacy .env).
pub fn is_installed(root: &Path) -> bool {
    root.join("config.json").exists() || root.join(".env").exists()
}

pub struct EnvFile {
    pub(crate) path: PathBuf,
    pub(crate) lines: Vec<String>,
}

impl EnvFile {
    pub fn load(path: impl AsRef<Path>) -> Result<Self> {
        let path = path.as_ref().to_path_buf();
        let dir = if path.is_dir() {
            path.clone()
        } else {
            path.parent().unwrap_or(Path::new(".")).to_path_buf()
        };
        let json_path = dir.join("config.json");
        let env_path = if path.is_dir() || path.file_name().is_some_and(|n| n == "config.json") {
            dir.join(".env")
        } else {
            path.clone()
        };

        if json_path.exists() {
            return Self::load_config_and_secrets(&dir);
        }
        if env_path.is_file() {
            let text = fs::read_to_string(&env_path).with_context(|| format!("read {}", env_path.display()))?;
            return Ok(Self::parse(path, &text));
        }
        if !path.is_dir() && path.exists() {
            let text = fs::read_to_string(&path).with_context(|| format!("read {}", path.display()))?;
            return Ok(Self::parse(path, &text));
        }
        bail!(std::io::Error::new(
            std::io::ErrorKind::NotFound,
            format!("no configuration in {}", dir.display())
        ));
    }

    fn load_config_and_secrets(dir: &Path) -> Result<Self> {
        let json_path = dir.join("config.json");
        let text = fs::read_to_string(&json_path).with_context(|| format!("read {}", json_path.display()))?;
        let val: serde_json::Value = serde_json::from_str(&text).with_context(|| format!("parse {}", json_path.display()))?;
        let mut lines = Vec::new();

        if let serde_json::Value::Object(map) = val {
            for (k, v) in map {
                let env_key = match k.as_str() {
                    "mode" => "MIKAN_MODE",
                    "image" => "MIKAN_IMAGE",
                    "version" => "MIKAN_VERSION",
                    "panel_port" => "PANEL_PORT",
                    "node_api_port" => "NODE_API_PORT",
                    "acme_listen" => "MIKAN_ACME_LISTEN",
                    "auto_update" => "MIKAN_AUTO_UPDATE",
                    "update_channel" => "MIKAN_UPDATE_CHANNEL",
                    "ufw" => "MIKAN_UFW",
                    other => other,
                };
                match v {
                    serde_json::Value::Bool(b) => {
                        lines.push(format!("{env_key}={}", if b { "1" } else { "0" }));
                    }
                    serde_json::Value::Number(n) => {
                        lines.push(format!("{env_key}={n}"));
                    }
                    serde_json::Value::String(s) => {
                        lines.push(format!("{env_key}={s}"));
                    }
                    serde_json::Value::Null => {}
                    _ => {}
                }
            }
        }

        // Load secrets
        let db_pw_path = dir.join("secrets/db_password");
        if db_pw_path.exists() {
            if let Ok(pw) = fs::read_to_string(&db_pw_path) {
                let pw = pw.trim();
                if !pw.is_empty() {
                    lines.push(format!("MIKAN_POSTGRES_PASSWORD={pw}"));
                    lines.push(format!("MIKAN_DATABASE_URL=postgresql://mikan:{pw}@localhost/mikan?host=/run/postgresql"));
                }
            }
        }

        let node_join_path = dir.join("secrets/node_join");
        if node_join_path.exists() {
            if let Ok(key) = fs::read_to_string(&node_join_path) {
                let key = key.trim();
                if !key.is_empty() {
                    lines.push(format!("MIKAN_NODE_JOIN={key}"));
                }
            }
        }

        Ok(Self {
            path: dir.join("config.json"),
            lines,
        })
    }

    pub fn new(path: impl AsRef<Path>) -> Self {
        Self::parse(path.as_ref().to_path_buf(), "")
    }

    /// The file at path as text kept from before (an update's copy of the old .env).
    pub fn from_text(path: impl AsRef<Path>, text: &str) -> Self {
        Self::parse(path.as_ref().to_path_buf(), text)
    }

    fn parse(path: PathBuf, text: &str) -> Self {
        Self { path, lines: text.lines().map(str::to_owned).collect() }
    }

    /// The value as compose reads it: the last of a repeated key, without quotes.
    pub fn get(&self, key: &str) -> Option<&str> {
        let raw = self.lines.iter().rev().find_map(|l| l.strip_prefix(key)?.strip_prefix('='))?;
        let v = ['"', '\''].iter().find_map(|&q| raw.strip_prefix(q)?.strip_suffix(q)).unwrap_or(raw);
        Some(v).filter(|v| !v.is_empty())
    }

    /// Sets key to value, in place when the key is there; a key repeated by a hand edit is
    /// left once, because compose takes the last and a change of the first would not show.
    pub fn set(&mut self, key: &str, value: &str) -> Result<()> {
        let fits = if key == DSN_KEY { plain_dsn(value) } else { plain(value) };
        if !fits {
            bail!("{key}: the value has characters that compose reads differently ($, quotes, spaces, #)");
        }
        let line = format!("{key}={value}");
        let at: Vec<usize> = self
            .lines
            .iter()
            .enumerate()
            .filter(|(_, l)| l.strip_prefix(key).is_some_and(|r| r.starts_with('=')))
            .map(|(i, _)| i)
            .collect();
        match at.split_last() {
            Some((&last, earlier)) => {
                self.lines[last] = line;
                for &i in earlier.iter().rev() {
                    self.lines.remove(i);
                }
            }
            None => self.lines.push(line),
        }
        Ok(())
    }

    /// Removes every line of key.
    pub fn remove(&mut self, key: &str) {
        self.lines.retain(|l| !l.strip_prefix(key).is_some_and(|r| r.starts_with('=')));
    }

    /// Returns all key-value entries currently loaded.
    pub fn entries(&self) -> Vec<(&str, &str)> {
        let mut map = std::collections::BTreeMap::new();
        for line in &self.lines {
            let line = line.trim();
            if line.is_empty() || line.starts_with('#') {
                continue;
            }
            if let Some((k, _)) = line.split_once('=') {
                if let Some(v) = self.get(k) {
                    map.insert(k, v);
                }
            }
        }
        map.into_iter().collect()
    }

    pub fn render(&self) -> String {
        let mut s = self.lines.join("\n");
        s.push('\n');
        s
    }

    /// Writes configuration to config.json and secrets to secrets/ (0600),
    /// removing any legacy .env file.
    pub fn save(&self) -> Result<()> {
        let dir = if self.path.is_dir() {
            self.path.clone()
        } else {
            self.path.parent().unwrap_or(Path::new(".")).to_path_buf()
        };

        if !dir.exists() {
            fs::create_dir_all(&dir)?;
        }

        // 1. Secrets
        let secrets_dir = dir.join("secrets");
        let mut wrote_secret = false;
        let db_pw = self.get("MIKAN_POSTGRES_PASSWORD").map(str::to_owned)
            .or_else(|| self.get("MIKAN_DATABASE_URL").and_then(password_from_dsn));
        if let Some(pw) = db_pw {
            if !secrets_dir.exists() {
                fs::DirBuilder::new().mode(0o700).create(&secrets_dir)?;
            }
            write_private(&secrets_dir.join("db_password"), pw.as_bytes())?;
            wrote_secret = true;
        }
        if let Some(join) = self.get("MIKAN_NODE_JOIN") {
            if !secrets_dir.exists() {
                fs::DirBuilder::new().mode(0o700).create(&secrets_dir)?;
            }
            write_private(&secrets_dir.join("node_join"), join.as_bytes())?;
            wrote_secret = true;
        }
        if wrote_secret && secrets_dir.exists() {
            let _ = fs::set_permissions(&secrets_dir, fs::Permissions::from_mode(0o700));
        }

        // 2. config.json
        let mut map = serde_json::Map::new();
        for line in &self.lines {
            let line = line.trim();
            if line.is_empty() || line.starts_with('#') {
                continue;
            }
            if let Some((k, v)) = line.split_once('=') {
                let v = ['"', '\''].iter().find_map(|&q| v.strip_prefix(q)?.strip_suffix(q)).unwrap_or(v);
                match k {
                    "MIKAN_POSTGRES_PASSWORD" | "MIKAN_NODE_JOIN" | "MIKAN_DATABASE_URL" => {
                        // secrets are kept in secrets/
                    }
                    "MIKAN_MODE" => {
                        map.insert("mode".into(), serde_json::Value::String(v.into()));
                    }
                    "MIKAN_IMAGE" => {
                        map.insert("image".into(), serde_json::Value::String(v.into()));
                    }
                    "MIKAN_VERSION" => {
                        map.insert("version".into(), serde_json::Value::String(v.into()));
                    }
                    "PANEL_PORT" => {
                        if let Ok(p) = v.parse::<u16>() {
                            map.insert("panel_port".into(), serde_json::Value::Number(p.into()));
                        }
                    }
                    "NODE_API_PORT" => {
                        if let Ok(p) = v.parse::<u16>() {
                            map.insert("node_api_port".into(), serde_json::Value::Number(p.into()));
                        }
                    }
                    "MIKAN_ACME_LISTEN" => {
                        map.insert("acme_listen".into(), serde_json::Value::String(v.into()));
                    }
                    "MIKAN_AUTO_UPDATE" => {
                        map.insert("auto_update".into(), serde_json::Value::Bool(v == "1"));
                    }
                    "MIKAN_UPDATE_CHANNEL" => {
                        map.insert("update_channel".into(), serde_json::Value::String(v.into()));
                    }
                    "MIKAN_UFW" => {
                        map.insert("ufw".into(), serde_json::Value::Bool(v == "1"));
                    }
                    other => {
                        map.insert(other.into(), serde_json::Value::String(v.into()));
                    }
                }
            }
        }

        let json_text = serde_json::to_string_pretty(&map)?;
        write_private(&dir.join("config.json"), json_text.as_bytes())?;

        // 3. Delete old .env if present
        let env_file = dir.join(".env");
        if env_file.exists() {
            let _ = fs::remove_file(env_file);
        }

        // If self.path was a custom test file (not config.json and not .env), write it as raw text
        if !self.path.is_dir()
            && self.path.file_name().is_none_or(|n| n != "config.json" && n != ".env")
        {
            write_private(&self.path, self.render().as_bytes())?;
        }

        Ok(())
    }
}

/// A value compose reads as it stands: it interpolates `$`, strips quotes and ends a line
/// at ` #`, so anything but plain image names, versions, ports and keys is refused.
fn plain(value: &str) -> bool {
    value.bytes().all(|b| b.is_ascii_alphanumeric() || b"._:/@+=,-".contains(&b))
}

/// The PostgreSQL address of the panel, the one value that is a URL with a query.
const DSN_KEY: &str = "MIKAN_DATABASE_URL";

/// A DSN also takes its query (`?host=…&sslmode=…`) and percent-escapes: compose gives
/// `?`, `&` and `%` no meaning, only the image names and keys of the rest never have them.
fn plain_dsn(value: &str) -> bool {
    value.bytes().all(|b| b.is_ascii_alphanumeric() || b"._:/@+=,-?&%".contains(&b))
}

/// Extracts password from a PostgreSQL DSN (`postgresql://user:password@host/db...`).
pub fn password_from_dsn(dsn: &str) -> Option<String> {
    let (_, rest) = dsn.split_once("://")?;
    let (user_pass, _) = rest.split_once('@')?;
    let (_, pass) = user_pass.split_once(':')?;
    Some(pass.to_owned()).filter(|p| !p.is_empty())
}

/// Writes a file readable by root only, replacing it at once: a new file under its own
/// name (never a leftover one, never through a link), synced, then renamed over path.
pub fn write_private(path: &Path, data: &[u8]) -> Result<()> {
    let name = path.file_name().map(|n| n.to_string_lossy().into_owned()).unwrap_or_default();
    let nanos = SystemTime::now().duration_since(UNIX_EPOCH).map(|d| d.as_nanos()).unwrap_or(0);
    let tmp = path.with_file_name(format!(".{name}.{}.{nanos}.tmp", std::process::id()));
    let mut f = fs::OpenOptions::new()
        .write(true)
        .create_new(true)
        .mode(0o600)
        .custom_flags(libc::O_NOFOLLOW)
        .open(&tmp)
        .with_context(|| format!("create {}", tmp.display()))?;
    let written = f.write_all(data).and_then(|()| f.sync_all()).and_then(|()| fs::rename(&tmp, path));
    if let Err(e) = written {
        let _ = fs::remove_file(&tmp);
        return Err(e).with_context(|| format!("write {}", path.display()));
    }
    // The rename is only as durable as its directory.
    if let Some(dir) = path.parent().and_then(|d| fs::File::open(d).ok()) {
        let _ = dir.sync_all();
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn keeps_unknown_lines_and_order() {
        let mut e = EnvFile::parse("x".into(), "MIKAN_IMAGE=mikan:0.3.8\n# note\nPANEL_PORT=21355\nMIKAN_UFW=1\n");
        assert_eq!(e.get("MIKAN_IMAGE"), Some("mikan:0.3.8"));
        assert_eq!(e.get("MIKAN"), None);
        e.set("MIKAN_IMAGE", "ghcr.io/shelbyhell/hikari@sha256:abc").unwrap();
        e.set("MIKAN_VERSION", "0.3.9").unwrap();
        assert_eq!(
            e.render(),
            "MIKAN_IMAGE=ghcr.io/shelbyhell/hikari@sha256:abc\n# note\nPANEL_PORT=21355\nMIKAN_UFW=1\nMIKAN_VERSION=0.3.9\n"
        );
        assert!(e.set("X", "a\nb").is_err());
    }

    #[test]
    fn values_compose_reads_as_written() {
        let mut e = EnvFile::parse("x".into(), "");
        for ok in ["ghcr.io/shelbyhell/hikari@sha256:ab12", "0.4.4-rc.1+b2", "mikan1.AbC_-9", "21355", ""] {
            e.set("K", ok).unwrap();
        }
        for bad in ["a$HOME", "${X}", "a b", "a #b", "\"q\"", "'q'", "a`b", "a\\b", "a;b", "a\nb", "img?x=1", "a&b", "a%20b"] {
            assert!(e.set("K", bad).is_err(), "{bad:?} was taken");
        }
    }

    #[test]
    fn the_dsn_takes_its_query_and_nothing_compose_reinterprets() {
        let mut e = EnvFile::parse("x".into(), "");
        for ok in
            ["postgresql://mikan:Ab9@localhost/mikan?host=/run/postgresql", "postgres://u:p%40ss@h/db?sslmode=require&connect_timeout=10"]
        {
            e.set("MIKAN_DATABASE_URL", ok).unwrap();
            assert_eq!(e.get("MIKAN_DATABASE_URL"), Some(ok));
        }
        for bad in [
            "postgres://u:$P@h/db",
            "postgres://u:p@h/db #x",
            "postgres://u:p@h/db?a='b'",
            "postgres://u:p w@h/db",
            "postgres://h/db?a=1;b",
        ] {
            assert!(e.set("MIKAN_DATABASE_URL", bad).is_err(), "{bad:?} was taken");
        }
        assert!(e.set("MIKAN_IMAGE", "postgresql://h/db?host=/run").is_err(), "a query only in the DSN");
    }

    #[test]
    fn a_key_is_removed_and_text_is_parsed() {
        let mut e = EnvFile::from_text("x", "A=1\nMIKAN_DATABASE_URL=postgresql://h/db\nAB=2\nMIKAN_DATABASE_URL=again\n");
        e.remove("MIKAN_DATABASE_URL");
        e.remove("A");
        assert_eq!(e.render(), "AB=2\n");
    }

    #[test]
    fn repeated_keys_and_quotes() {
        let mut e = EnvFile::parse("x".into(), "MIKAN_IMAGE=old\nPANEL_PORT=\"2053\"\nMIKAN_IMAGE=newer\nQ='x'\nE=\n");
        assert_eq!(e.get("MIKAN_IMAGE"), Some("newer"));
        assert_eq!(e.get("PANEL_PORT"), Some("2053"));
        assert_eq!(e.get("Q"), Some("x"));
        assert_eq!(e.get("E"), None);
        e.set("MIKAN_IMAGE", "fresh").unwrap();
        assert_eq!(e.render(), "PANEL_PORT=\"2053\"\nMIKAN_IMAGE=fresh\nQ='x'\nE=\n");
    }

    #[test]
    fn entries_returns_all_keys() {
        let e = EnvFile::parse("x".into(), "A=1\n# comment\nB='2'\nC=\"hello\"\nA=updated\n");
        let entries = e.entries();
        assert_eq!(entries, vec![("A", "updated"), ("B", "2"), ("C", "hello")]);
    }

    #[test]
    fn extracts_password_from_dsn() {
        assert_eq!(
            password_from_dsn("postgresql://mikan:SuperSecret123@localhost/mikan?host=/run/postgresql"),
            Some("SuperSecret123".into())
        );
        assert_eq!(password_from_dsn("invalid"), None);
    }

    #[test]
    fn private_files_are_private_and_leave_no_tmp() {
        use std::os::unix::fs::symlink;
        let d = std::env::temp_dir().join(format!("envfile-{}", std::process::id()));
        let _ = fs::remove_dir_all(&d);
        fs::create_dir_all(&d).unwrap();
        let f = d.join(".env");
        write_private(&f, b"A=1\n").unwrap();
        write_private(&f, b"A=2\n").unwrap();
        assert_eq!(fs::read_to_string(&f).unwrap(), "A=2\n");
        assert_eq!(fs::metadata(&f).unwrap().permissions().mode() & 0o777, 0o600);
        let target = d.join("target");
        fs::write(&target, "keep").unwrap();
        fs::remove_file(&f).unwrap();
        symlink(&target, &f).unwrap();
        write_private(&f, b"A=3\n").unwrap();
        assert_eq!(fs::read_to_string(&target).unwrap(), "keep");
        assert_eq!(fs::read_to_string(&f).unwrap(), "A=3\n");
        assert_eq!(fs::read_dir(&d).unwrap().count(), 2, "no temporary file stays");
        fs::remove_dir_all(&d).unwrap();
    }

    #[test]
    fn saves_config_json_and_secrets_without_env() {
        let d = std::env::temp_dir().join(format!("envfile-save-{}", std::process::id()));
        let _ = fs::remove_dir_all(&d);
        fs::create_dir_all(&d).unwrap();

        // Write a legacy .env
        let env_path = d.join(".env");
        fs::write(&env_path, "MIKAN_MODE=panel\nPANEL_PORT=8080\nMIKAN_POSTGRES_PASSWORD=secret123\n").unwrap();

        // Load legacy .env
        let mut e = EnvFile::load(&d).unwrap();
        assert_eq!(e.get("PANEL_PORT"), Some("8080"));
        assert_eq!(e.get("MIKAN_POSTGRES_PASSWORD"), Some("secret123"));

        // Save: should create config.json and secrets/db_password and remove .env
        e.set("MIKAN_IMAGE", "ghcr.io/shelbyhell/hikari:test").unwrap();
        e.save().unwrap();

        assert!(!env_path.exists(), ".env must be removed");
        assert!(d.join("config.json").exists(), "config.json must exist");
        assert!(d.join("secrets/db_password").exists(), "secrets/db_password must exist");
        assert_eq!(fs::read_to_string(d.join("secrets/db_password")).unwrap(), "secret123");

        let json_text = fs::read_to_string(d.join("config.json")).unwrap();
        assert!(!json_text.contains("secret123"), "password must not be in config.json");
        assert!(json_text.contains("\"panel_port\": 8080"));
        assert!(json_text.contains("\"image\": \"ghcr.io/shelbyhell/hikari:test\""));

        // Load again from directory: should find config.json and secrets
        let loaded = EnvFile::load(&d).unwrap();
        assert_eq!(loaded.get("PANEL_PORT"), Some("8080"));
        assert_eq!(loaded.get("MIKAN_IMAGE"), Some("ghcr.io/shelbyhell/hikari:test"));
        assert_eq!(loaded.get("MIKAN_POSTGRES_PASSWORD"), Some("secret123"));
        assert!(loaded.get("MIKAN_DATABASE_URL").unwrap().contains("secret123"));

        fs::remove_dir_all(&d).unwrap();
    }
}
