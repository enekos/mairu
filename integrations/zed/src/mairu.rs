use schemars::JsonSchema;
use serde::Deserialize;
use zed::http_client::{HttpMethod, HttpRequest, RedirectPolicy};
use zed::settings::ContextServerSettings;
use zed_extension_api::{
    self as zed, serde_json, Command, ContextServerConfiguration, ContextServerId, Project, Result,
    SlashCommand, SlashCommandOutput, SlashCommandOutputSection, Worktree,
};

const DEFAULT_API_URL: &str = "http://localhost:8788";
const DEFAULT_DASHBOARD_URL: &str = "http://localhost:5173";

struct MairuExtension;

#[derive(Debug, Default, Deserialize, JsonSchema)]
struct MairuContextServerSettings {
    #[serde(default)]
    path: Option<String>,
    #[serde(default)]
    default_project: Option<String>,
    #[serde(default)]
    api_url: Option<String>,
    #[serde(default)]
    auth_token: Option<String>,
}

fn parse_settings(project: &Project) -> MairuContextServerSettings {
    let raw = ContextServerSettings::for_project("mairu", project).ok();
    raw.and_then(|s| s.settings)
        .and_then(|v| serde_json::from_value::<MairuContextServerSettings>(v).ok())
        .unwrap_or_default()
}

fn first_non_empty(values: &[Option<String>]) -> Option<String> {
    for v in values {
        if let Some(s) = v {
            let t = s.trim();
            if !t.is_empty() {
                return Some(t.to_string());
            }
        }
    }
    None
}

fn shell_env_get(worktree: &Worktree, key: &str) -> Option<String> {
    worktree
        .shell_env()
        .into_iter()
        .find(|(k, _)| k == key)
        .map(|(_, v)| v)
}

fn worktree_basename(worktree: &Worktree) -> Option<String> {
    let root = worktree.root_path();
    root.trim_end_matches('/')
        .rsplit('/')
        .next()
        .filter(|s| !s.is_empty())
        .map(|s| s.to_string())
}

/// Resolves the active project for slash commands. Order:
/// 1. `MAIRU_DEFAULT_PROJECT` from the worktree shell env
/// 2. basename of the worktree root
fn resolve_project_from_worktree(worktree: Option<&Worktree>) -> Option<String> {
    let wt = worktree?;
    if let Some(v) = shell_env_get(wt, "MAIRU_DEFAULT_PROJECT") {
        let t = v.trim();
        if !t.is_empty() {
            return Some(t.to_string());
        }
    }
    worktree_basename(wt)
}

fn resolve_api_url(worktree: Option<&Worktree>) -> String {
    if let Some(wt) = worktree {
        if let Some(v) = shell_env_get(wt, "MAIRU_API_URL")
            .or_else(|| shell_env_get(wt, "MAIRU_CONTEXT_SERVER_URL"))
        {
            let t = v.trim();
            if !t.is_empty() {
                return t.trim_end_matches('/').to_string();
            }
        }
    }
    DEFAULT_API_URL.to_string()
}

fn resolve_dashboard_url(worktree: Option<&Worktree>) -> String {
    if let Some(wt) = worktree {
        if let Some(v) = shell_env_get(wt, "MAIRU_DASHBOARD_URL") {
            let t = v.trim();
            if !t.is_empty() {
                return t.trim_end_matches('/').to_string();
            }
        }
    }
    DEFAULT_DASHBOARD_URL.to_string()
}

fn resolve_auth_token(worktree: Option<&Worktree>) -> Option<String> {
    let wt = worktree?;
    shell_env_get(wt, "MAIRU_CONTEXT_SERVER_TOKEN")
        .or_else(|| shell_env_get(wt, "MAIRU_API_TOKEN"))
        .filter(|s| !s.trim().is_empty())
}

fn url_encode(s: &str) -> String {
    let mut out = String::with_capacity(s.len());
    for b in s.as_bytes() {
        match *b {
            b'A'..=b'Z' | b'a'..=b'z' | b'0'..=b'9' | b'-' | b'_' | b'.' | b'~' => {
                out.push(*b as char)
            }
            _ => out.push_str(&format!("%{:02X}", b)),
        }
    }
    out
}

fn http_get_json(url: &str, auth_token: Option<&str>) -> Result<serde_json::Value, String> {
    let mut builder = HttpRequest::builder()
        .method(HttpMethod::Get)
        .url(url)
        .redirect_policy(RedirectPolicy::FollowAll);
    if let Some(tok) = auth_token {
        builder = builder.header("X-Context-Token", tok);
    }
    let req = builder.build()?;
    let resp = req.fetch()?;
    serde_json::from_slice::<serde_json::Value>(&resp.body)
        .map_err(|e| format!("invalid JSON from {}: {}", url, e))
}

fn build_search_url(api_url: &str, query: &str, search_type: &str, project: Option<&str>) -> String {
    let mut url = format!(
        "{}/api/search?q={}&type={}&topK=5",
        api_url.trim_end_matches('/'),
        url_encode(query),
        search_type
    );
    if let Some(p) = project {
        if !p.is_empty() {
            url.push_str("&project=");
            url.push_str(&url_encode(p));
        }
    }
    url
}

fn format_search_results(value: &serde_json::Value, kind: &str) -> String {
    let arr = value
        .get("results")
        .and_then(|v| v.as_array())
        .or_else(|| value.as_array());

    let Some(items) = arr else {
        return format!("(no {} results)\n", kind);
    };
    if items.is_empty() {
        return format!("(no {} results)\n", kind);
    }

    let mut out = String::new();
    for (i, item) in items.iter().enumerate() {
        let title = item
            .get("name")
            .or_else(|| item.get("uri"))
            .or_else(|| item.get("title"))
            .and_then(|v| v.as_str())
            .unwrap_or("(untitled)");

        let body = item
            .get("content")
            .or_else(|| item.get("abstract"))
            .or_else(|| item.get("overview"))
            .and_then(|v| v.as_str())
            .unwrap_or("");

        let score = item
            .get("score")
            .and_then(|v| v.as_f64())
            .map(|s| format!(" (score {:.2})", s))
            .unwrap_or_default();

        out.push_str(&format!("{}. {}{}\n", i + 1, title, score));
        if !body.is_empty() {
            let trimmed: String = body.chars().take(400).collect();
            out.push_str(&format!("   {}\n", trimmed.replace('\n', " ")));
        }
    }
    out
}

fn slash_text_section(text: String, label: String) -> SlashCommandOutput {
    let len = text.len() as u32;
    SlashCommandOutput {
        sections: vec![SlashCommandOutputSection { range: (0..len).into(), label }],
        text,
    }
}

fn run_search(
    args: Vec<String>,
    worktree: Option<&Worktree>,
    search_type: &str,
    label_prefix: &str,
) -> Result<SlashCommandOutput, String> {
    if args.is_empty() {
        return Err("usage: /mairu-... <query>".into());
    }
    let query = args.join(" ");
    let api_url = resolve_api_url(worktree);
    let project = resolve_project_from_worktree(worktree);
    let token = resolve_auth_token(worktree);
    let url = build_search_url(&api_url, &query, search_type, project.as_deref());
    let value = http_get_json(&url, token.as_deref())?;
    let body = format_search_results(&value, search_type);
    let header = format!(
        "## {} for \"{}\"{}\n\n",
        label_prefix,
        query,
        project
            .as_deref()
            .map(|p| format!(" (project: {})", p))
            .unwrap_or_default()
    );
    Ok(slash_text_section(
        format!("{}{}", header, body),
        format!("Mairu {}: {}", label_prefix.to_lowercase(), query),
    ))
}

fn run_recall(
    args: Vec<String>,
    worktree: Option<&Worktree>,
) -> Result<SlashCommandOutput, String> {
    if args.is_empty() {
        return Err("usage: /mairu-recall <query>".into());
    }
    let query = args.join(" ");
    let api_url = resolve_api_url(worktree);
    let project = resolve_project_from_worktree(worktree);
    let token = resolve_auth_token(worktree);

    let mem = http_get_json(
        &build_search_url(&api_url, &query, "memory", project.as_deref()),
        token.as_deref(),
    )
    .ok();
    let nodes = http_get_json(
        &build_search_url(&api_url, &query, "context", project.as_deref()),
        token.as_deref(),
    )
    .ok();

    let mut text = format!(
        "## Mairu recall for \"{}\"{}\n\n",
        query,
        project
            .as_deref()
            .map(|p| format!(" (project: {})", p))
            .unwrap_or_default()
    );
    text.push_str("### Memories\n");
    text.push_str(&match mem {
        Some(v) => format_search_results(&v, "memory"),
        None => "(memory search failed)\n".into(),
    });
    text.push_str("\n### Context nodes\n");
    text.push_str(&match nodes {
        Some(v) => format_search_results(&v, "context"),
        None => "(node search failed)\n".into(),
    });
    Ok(slash_text_section(
        text,
        format!("Mairu recall: {}", query),
    ))
}

fn run_dashboard(
    args: Vec<String>,
    worktree: Option<&Worktree>,
) -> Result<SlashCommandOutput, String> {
    let dashboard = resolve_dashboard_url(worktree);
    let project = resolve_project_from_worktree(worktree);
    let mut url = dashboard.clone();
    let mut params: Vec<String> = Vec::new();
    if let Some(p) = &project {
        params.push(format!("project={}", url_encode(p)));
    }
    if !args.is_empty() {
        params.push(format!("q={}", url_encode(&args.join(" "))));
    }
    if !params.is_empty() {
        url.push('?');
        url.push_str(&params.join("&"));
    }
    let text = format!("Mairu dashboard: {}\n", url);
    Ok(slash_text_section(text, "Mairu dashboard".into()))
}

fn run_doctor(worktree: Option<&Worktree>) -> Result<SlashCommandOutput, String> {
    let api_url = resolve_api_url(worktree);
    let project = resolve_project_from_worktree(worktree);
    let token = resolve_auth_token(worktree);

    let health_url = format!("{}/api/health", api_url.trim_end_matches('/'));
    let health = http_get_json(&health_url, token.as_deref());

    let mut text = String::new();
    text.push_str(&format!("## Mairu doctor\n\n"));
    text.push_str(&format!("- API URL: `{}`\n", api_url));
    text.push_str(&format!(
        "- Project: `{}`\n",
        project.as_deref().unwrap_or("(none)")
    ));
    text.push_str(&format!(
        "- Auth token: {}\n",
        if token.is_some() { "set" } else { "not set" }
    ));
    text.push_str(&match health {
        Ok(v) => format!("- Health: ok — `{}`\n", v),
        Err(e) => format!("- Health: **error** — {}\n", e),
    });
    Ok(slash_text_section(text, "Mairu doctor".into()))
}

impl zed::Extension for MairuExtension {
    fn new() -> Self {
        Self
    }

    fn context_server_command(
        &mut self,
        _context_server_id: &ContextServerId,
        project: &Project,
    ) -> Result<Command> {
        let settings = parse_settings(project);

        let command = first_non_empty(&[settings.path.clone()]).unwrap_or_else(|| "mairu".into());

        let mut env: Vec<(String, String)> = Vec::new();
        if let Some(p) = first_non_empty(&[settings.default_project.clone()]) {
            env.push(("MAIRU_DEFAULT_PROJECT".into(), p));
        }
        if let Some(api) = first_non_empty(&[settings.api_url.clone()]) {
            env.push(("MAIRU_CONTEXT_SERVER_URL".into(), api));
        }
        if let Some(tok) = first_non_empty(&[settings.auth_token.clone()]) {
            env.push(("MAIRU_CONTEXT_SERVER_TOKEN".into(), tok));
        }

        Ok(Command {
            command,
            args: vec!["mcp".into()],
            env,
        })
    }

    fn context_server_configuration(
        &mut self,
        _context_server_id: &ContextServerId,
        _project: &Project,
    ) -> Result<Option<ContextServerConfiguration>> {
        let installation_instructions =
            include_str!("../configuration/installation_instructions.md").to_string();
        let default_settings = include_str!("../configuration/default_settings.jsonc").to_string();
        let settings_schema =
            serde_json::to_string(&schemars::schema_for!(MairuContextServerSettings))
                .map_err(|e| e.to_string())?;

        Ok(Some(ContextServerConfiguration {
            installation_instructions,
            default_settings,
            settings_schema,
        }))
    }

    fn run_slash_command(
        &self,
        command: SlashCommand,
        args: Vec<String>,
        worktree: Option<&Worktree>,
    ) -> Result<SlashCommandOutput, String> {
        match command.name.as_str() {
            "mairu-search" => run_search(args, worktree, "memory", "Memories"),
            "mairu-nodes" => run_search(args, worktree, "context", "Context nodes"),
            "mairu-recall" => run_recall(args, worktree),
            "mairu-dashboard" => run_dashboard(args, worktree),
            "mairu-doctor" => run_doctor(worktree),
            other => Err(format!("unknown slash command: {}", other)),
        }
    }
}

zed::register_extension!(MairuExtension);
