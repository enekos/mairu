// browser-extension/crates/wasm/src/lib.rs
use browser_extension_core::{dedup, extractor, session::SessionManager, types::*};
use std::cell::RefCell;
use wasm_bindgen::prelude::*;

use serde::Serialize;

fn to_js(v: &serde_json::Value) -> JsValue {
    // serde_wasm_bindgen default serializes JSON objects as JS Maps — use the
    // json-compatible serializer so consumers can do `result.status` in JS.
    let ser = serde_wasm_bindgen::Serializer::json_compatible();
    v.serialize(&ser).unwrap_or(JsValue::NULL)
}

thread_local! {
    static SESSION: RefCell<Option<SessionManager>> = const { RefCell::new(None) };
}

#[wasm_bindgen]
pub fn init_session(session_id: &str, started_at_secs: u64) {
    console_error_panic_hook::set_once();
    SESSION.with(|s| {
        *s.borrow_mut() = Some(SessionManager::new_at(session_id.to_string(), started_at_secs));
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

    to_js(&serde_json::json!({
        "ok": true,
        "status": status,
        "is_new": result == AddPageResult::Added,
        "content_hash": format!("{:016x}", content_hash),
        "section_count": section_count,
    }))
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

fn to_js_value<T: Serialize + ?Sized>(v: &T) -> JsValue {
    let ser = serde_wasm_bindgen::Serializer::json_compatible();
    v.serialize(&ser).unwrap_or(JsValue::NULL)
}

#[wasm_bindgen]
pub fn get_current() -> JsValue {
    SESSION.with(|s| {
        let s = s.borrow();
        match s.as_ref().and_then(|mgr| mgr.current_page()) {
            Some(page) => to_js_value(page),
            None => JsValue::NULL,
        }
    })
}

#[wasm_bindgen]
pub fn get_history() -> JsValue {
    SESSION.with(|s| {
        let s = s.borrow();
        match s.as_ref() {
            Some(mgr) => to_js_value(mgr.history()),
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
                to_js_value(&results)
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
            Some(mgr) => to_js_value(&mgr.summary()),
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
                to_js_value(&pending)
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
