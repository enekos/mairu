use axum::{
    extract::State,
    http::{HeaderMap, StatusCode},
    routing::{get, post},
    Json, Router,
};
use serde_json::{json, Value};
use std::{
    collections::HashMap,
    sync::{
        atomic::{AtomicU64, Ordering},
        Arc,
    },
    time::{Duration, Instant},
};
use tokio::sync::{mpsc, oneshot, Mutex};

#[derive(Clone)]
pub struct AppState {
    pub pending: Arc<Mutex<HashMap<String, oneshot::Sender<Value>>>>,
    pub to_extension: mpsc::Sender<Value>,
    pub started: Instant,
    pub counter: Arc<AtomicU64>,
}

pub fn router(state: AppState) -> Router {
    Router::new()
        .route("/healthz", get(healthz))
        .route("/query", post(query_handler))
        .route("/execute", post(execute_handler))
        .route("/screenshot", post(screenshot_handler))
        .with_state(state)
}

async fn healthz(State(s): State<AppState>) -> Json<Value> {
    Json(json!({
        "ok": true,
        "version": env!("CARGO_PKG_VERSION"),
        "uptime_s": s.started.elapsed().as_secs(),
        "pending": s.pending.lock().await.len(),
    }))
}

fn origin_ok(h: &HeaderMap) -> bool {
    match h.get("origin").and_then(|v| v.to_str().ok()) {
        None => true,
        Some("null") => true,
        Some(o) => {
            o.starts_with("http://localhost")
                || o.starts_with("http://127.0.0.1")
                || o.starts_with("http://[::1]")
        }
    }
}

async fn query_handler(
    State(s): State<AppState>,
    headers: HeaderMap,
    Json(body): Json<Value>,
) -> Result<Json<Value>, (StatusCode, Json<Value>)> {
    handle(s, headers, body, "query", 5).await
}

async fn execute_handler(
    State(s): State<AppState>,
    headers: HeaderMap,
    Json(body): Json<Value>,
) -> Result<Json<Value>, (StatusCode, Json<Value>)> {
    handle(s, headers, body, "execute", 5).await
}

async fn screenshot_handler(
    State(s): State<AppState>,
    headers: HeaderMap,
    Json(body): Json<Value>,
) -> Result<Json<Value>, (StatusCode, Json<Value>)> {
    handle(s, headers, body, "screenshot", 15).await
}

async fn handle(
    s: AppState,
    headers: HeaderMap,
    mut body: Value,
    kind: &'static str,
    timeout_s: u64,
) -> Result<Json<Value>, (StatusCode, Json<Value>)> {
    if !origin_ok(&headers) {
        return Err((
            StatusCode::FORBIDDEN,
            Json(json!({"error": {"code":"bad_origin","message":"origin not allowed"}})),
        ));
    }
    let id = format!("req-{}", s.counter.fetch_add(1, Ordering::Relaxed));
    if let Value::Object(ref mut m) = body {
        m.insert("id".into(), Value::String(id.clone()));
        match kind {
            "execute" => {
                m.insert("type".into(), Value::String("execute".into()));
            }
            "screenshot" => {
                m.insert("command".into(), Value::String("screenshot".into()));
            }
            _ => {}
        }
    }
    let (tx, rx) = oneshot::channel();
    s.pending.lock().await.insert(id.clone(), tx);
    if s.to_extension.send(body).await.is_err() {
        s.pending.lock().await.remove(&id);
        return Err((
            StatusCode::SERVICE_UNAVAILABLE,
            Json(json!({"error": {"code":"ext_down","message":"extension not connected"}})),
        ));
    }
    match tokio::time::timeout(Duration::from_secs(timeout_s), rx).await {
        Ok(Ok(v)) => Ok(Json(v)),
        _ => {
            s.pending.lock().await.remove(&id);
            Err((
                StatusCode::GATEWAY_TIMEOUT,
                Json(json!({"error": {"code":"timeout","id":id}})),
            ))
        }
    }
}
