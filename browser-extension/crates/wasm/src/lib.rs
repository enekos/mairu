// browser-extension/crates/wasm/src/lib.rs
use browser_extension_core::{dedup, extractor, session::SessionManager, types::*};
use std::cell::RefCell;
use wasm_bindgen::prelude::*;

thread_local! {
    static SESSION: RefCell<Option<SessionManager>> = const { RefCell::new(None) };
}

#[wasm_bindgen]
pub fn init_session(session_id: &str) {
    SESSION.with(|s| {
        *s.borrow_mut() = Some(SessionManager::new(session_id.to_string()));
    });
}

#[wasm_bindgen]
pub fn process_page(
    url: &str,
    html: &str,
    timestamp: u64,
    selection: Option<String>,
    active_element: Option<String>,
    console_errors_json: &str,
    network_errors_json: &str,
    visual_rects_json: &str,
    storage_state_json: &str,
) -> JsValue {
    let console_errors: Vec<String> = serde_json::from_str(console_errors_json).unwrap_or_default();
    let network_errors: Vec<String> = serde_json::from_str(network_errors_json).unwrap_or_default();
    let visual_rects: std::collections::HashMap<String, String> =
        serde_json::from_str(visual_rects_json).unwrap_or_default();
    let storage_state: std::collections::HashMap<String, String> =
        serde_json::from_str(storage_state_json).unwrap_or_default();

    let extracted = extractor::extract(html);
    let content_hash = dedup::simhash(
        &extracted
            .sections
            .iter()
            .map(|s| s.text.as_str())
            .collect::<Vec<_>>()
            .join(" "),
    );

    let snapshot = PageSnapshot {
        url: url.to_string(),
        title: extracted.title,
        timestamp,
        content_hash,
        sections: extracted.sections,
        metadata: extracted.metadata,
        selection,
        active_element,
        console_errors,
        network_errors,
        visual_rects,
        storage_state,
    };

    let is_new = SESSION.with(|s| {
        let mut s = s.borrow_mut();
        match s.as_mut() {
            Some(mgr) => mgr.add_page(snapshot.clone()),
            None => false,
        }
    });

    serde_wasm_bindgen::to_value(&serde_json::json!({
        "is_new": is_new,
        "content_hash": content_hash,
        "section_count": snapshot.sections.len(),
    }))
    .unwrap_or(JsValue::NULL)
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
