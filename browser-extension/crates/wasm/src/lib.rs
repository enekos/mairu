// browser-extension/crates/wasm/src/lib.rs
use browser_extension_core::{dedup, extractor, session::SessionManager, types::*};
use std::cell::RefCell;
use wasm_bindgen::prelude::*;

thread_local! {
    static SESSION: RefCell<Option<SessionManager>> = const { RefCell::new(None) };
}

#[wasm_bindgen(start)]
pub fn start() {
    console_error_panic_hook::set_once();
}

#[wasm_bindgen]
pub fn init_session(session_id: &str) {
    SESSION.with(|s| {
        *s.borrow_mut() = Some(SessionManager::new(session_id.to_string()));
    });
}

#[derive(serde::Deserialize)]
pub struct ProcessPageArgs {
    pub url: String,
    pub html: String,
    pub timestamp: u64,
    pub selection: Option<String>,
    pub active_element: Option<String>,
    pub console_errors_json: String,
    pub network_errors_json: String,
    pub visual_rects_json: String,
    pub storage_state_json: String,
    #[serde(default)]
    pub dwell_ms: u64,
    #[serde(default)]
    pub interaction_count: u32,
    #[serde(default)]
    pub iframes_json: String,
    #[serde(default)]
    pub truncated: bool,
}

#[wasm_bindgen]
pub fn process_page(args_val: JsValue) -> JsValue {
    let args: ProcessPageArgs = match serde_wasm_bindgen::from_value(args_val) {
        Ok(a) => a,
        Err(e) => {
            return serde_wasm_bindgen::to_value(&serde_json::json!({
                "ok": false,
                "error": { "code": "bad_args", "message": e.to_string() },
            }))
            .unwrap_or(JsValue::NULL)
        }
    };

    let console_errors: Vec<String> =
        serde_json::from_str(&args.console_errors_json).unwrap_or_default();
    let network_errors: Vec<String> =
        serde_json::from_str(&args.network_errors_json).unwrap_or_default();
    let visual_rects: std::collections::HashMap<String, String> =
        serde_json::from_str(&args.visual_rects_json).unwrap_or_default();
    let storage_state: std::collections::HashMap<String, String> =
        serde_json::from_str(&args.storage_state_json).unwrap_or_default();

    let extracted = extractor::extract(&args.html);
    let content_hash = dedup::simhash(
        &extracted
            .sections
            .iter()
            .map(|s| s.text.as_str())
            .collect::<Vec<_>>()
            .join(" "),
    );

    let snapshot = PageSnapshot {
        url: args.url,
        title: extracted.title,
        timestamp: args.timestamp,
        content_hash,
        sections: extracted.sections,
        metadata: extracted.metadata,
        selection: args.selection,
        active_element: args.active_element,
        console_errors,
        network_errors,
        visual_rects,
        storage_state,
        revision: 0,
        importance_score: 0.0,
        dwell_ms: args.dwell_ms,
        interaction_count: args.interaction_count,
        iframe_content: extract_iframes(&args.iframes_json),
        truncated: args.truncated,
    };

    let section_count = snapshot.sections.len();
    let result = SESSION.with(|s| {
        let mut s = s.borrow_mut();
        match s.as_mut() {
            Some(mgr) => mgr.add_page(snapshot),
            None => AddPageResult::Duplicate,
        }
    });

    let status = match result {
        AddPageResult::Added => "added",
        AddPageResult::Updated => "updated",
        AddPageResult::Duplicate => "duplicate",
    };

    serde_wasm_bindgen::to_value(&serde_json::json!({
        "ok": true,
        "status": status,
        "is_new": result == AddPageResult::Added,
        "content_hash": content_hash,
        "section_count": section_count,
    }))
    .unwrap_or(JsValue::NULL)
}

/// Parse iframes JSON from the content script and run the extractor on same-origin HTML.
#[derive(serde::Deserialize)]
struct IframeInput {
    src: String,
    #[serde(default)]
    title: Option<String>,
    #[serde(default)]
    is_same_origin: bool,
    #[serde(default)]
    html: Option<String>,
}

fn extract_iframes(iframes_json: &str) -> Vec<IframeContent> {
    if iframes_json.is_empty() {
        return vec![];
    }
    let inputs: Vec<IframeInput> = match serde_json::from_str(iframes_json) {
        Ok(v) => v,
        Err(_) => return vec![],
    };
    inputs
        .into_iter()
        .map(|f| {
            let sections = if f.is_same_origin {
                f.html
                    .as_deref()
                    .map(|html| extractor::extract(html).sections)
                    .unwrap_or_default()
            } else {
                vec![]
            };
            IframeContent {
                src: f.src,
                title: f.title,
                is_same_origin: f.is_same_origin,
                sections,
            }
        })
        .collect()
}

#[wasm_bindgen]
pub fn get_current() -> JsValue {
    SESSION.with(|s| {
        let s = s.borrow();
        match s.as_ref().and_then(|mgr| mgr.current_page()) {
            Some(page) => serde_wasm_bindgen::to_value(page).unwrap_or(JsValue::NULL),
            None => JsValue::NULL,
        }
    })
}

#[wasm_bindgen]
pub fn get_history() -> JsValue {
    SESSION.with(|s| {
        let s = s.borrow();
        match s.as_ref() {
            Some(mgr) => serde_wasm_bindgen::to_value(mgr.history()).unwrap_or(JsValue::NULL),
            None => JsValue::NULL,
        }
    })
}

#[wasm_bindgen]
pub fn search_session(query: &str, limit: usize) -> JsValue {
    SESSION.with(|s| {
        let s = s.borrow();
        match s.as_ref() {
            Some(mgr) => {
                let results = mgr.search(query, limit);
                serde_wasm_bindgen::to_value(&results).unwrap_or(JsValue::NULL)
            }
            None => JsValue::NULL,
        }
    })
}

#[wasm_bindgen]
pub fn get_session_summary() -> JsValue {
    SESSION.with(|s| {
        let s = s.borrow();
        match s.as_ref() {
            Some(mgr) => serde_wasm_bindgen::to_value(&mgr.summary()).unwrap_or(JsValue::NULL),
            None => JsValue::NULL,
        }
    })
}

#[wasm_bindgen]
pub fn get_pending_sync() -> JsValue {
    SESSION.with(|s| {
        let s = s.borrow();
        match s.as_ref() {
            Some(mgr) => {
                let pending: Vec<_> = mgr.pending_sync().into_iter().cloned().collect();
                serde_wasm_bindgen::to_value(&pending).unwrap_or(JsValue::NULL)
            }
            None => JsValue::NULL,
        }
    })
}

#[wasm_bindgen]
pub fn mark_synced(content_hash: u64) {
    SESSION.with(|s| {
        let mut s = s.borrow_mut();
        if let Some(mgr) = s.as_mut() {
            mgr.mark_synced(content_hash);
        }
    });
}

/// Serialize the current session to a JSON string for persistence.
/// Returns null if no session is active.
#[wasm_bindgen]
pub fn export_session() -> Option<String> {
    SESSION.with(|s| {
        let s = s.borrow();
        s.as_ref()?.export_session().ok()
    })
}

/// Restore a previously exported session from a JSON string.
/// Returns true on success, false on failure.
#[wasm_bindgen]
pub fn import_session(json: &str) -> bool {
    match SessionManager::import_session(json) {
        Ok(mgr) => {
            SESSION.with(|s| {
                *s.borrow_mut() = Some(mgr);
            });
            true
        }
        Err(_) => false,
    }
}
