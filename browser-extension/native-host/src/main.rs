mod http;
mod protocol;

use std::{
    collections::HashMap,
    net::SocketAddr,
    sync::{atomic::AtomicU64, Arc},
    time::Instant,
};
use tokio::{
    io::BufWriter,
    sync::{mpsc, oneshot, Mutex},
};

#[tokio::main(flavor = "multi_thread", worker_threads = 2)]
async fn main() {
    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| tracing_subscriber::EnvFilter::new("info")),
        )
        .with_writer(std::io::stderr)
        .init();

    let port: u16 = std::env::var("MAIRU_EXT_HOST_PORT")
        .ok()
        .and_then(|s| s.parse().ok())
        .unwrap_or(7081);
    let addr: SocketAddr = format!("127.0.0.1:{port}").parse().unwrap();

    let pending: Arc<Mutex<HashMap<String, oneshot::Sender<serde_json::Value>>>> =
        Arc::new(Mutex::new(HashMap::new()));
    let (tx_to_ext, mut rx_to_ext) = mpsc::channel::<serde_json::Value>(64);

    let writer_task = tokio::spawn(async move {
        let mut out = BufWriter::new(tokio::io::stdout());
        while let Some(v) = rx_to_ext.recv().await {
            if let Err(e) = protocol::write_message(&mut out, &v).await {
                tracing::error!("write_message: {e}");
                break;
            }
        }
    });

    let pending_r = pending.clone();
    let reader_task = tokio::spawn(async move {
        let mut stdin = tokio::io::stdin();
        loop {
            match protocol::read_message(&mut stdin).await {
                Ok(msg) => {
                    if let Some(id) = msg.get("id").and_then(|v| v.as_str()).map(str::to_string) {
                        if let Some(tx) = pending_r.lock().await.remove(&id) {
                            let _ = tx.send(msg);
                        }
                    }
                }
                Err(protocol::ProtocolError::Eof) => {
                    tracing::info!("extension stdin closed");
                    break;
                }
                Err(protocol::ProtocolError::TooLarge(n)) => {
                    tracing::warn!("msg too large: {n}");
                    continue;
                }
                Err(e) => {
                    tracing::error!("read_message: {e}");
                    break;
                }
            }
        }
    });

    let state = http::AppState {
        pending,
        to_extension: tx_to_ext,
        started: Instant::now(),
        counter: Arc::new(AtomicU64::new(0)),
    };
    let app = http::router(state);
    let listener = tokio::net::TcpListener::bind(addr)
        .await
        .expect("bind native-host http");
    tracing::info!("native host listening on {addr}");
    let serve = axum::serve(listener, app);

    tokio::select! {
        r = serve => { if let Err(e) = r { tracing::error!("serve: {e}"); } }
        _ = reader_task => {}
        _ = writer_task => {}
    }
}
