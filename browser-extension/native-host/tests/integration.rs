use serde_json::{json, Value};
use std::process::Stdio;
use std::time::Duration;
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::process::{Child, Command};

async fn spawn_host(port: u16) -> Child {
    Command::new(env!("CARGO_BIN_EXE_browser-extension-host"))
        .env("MAIRU_EXT_HOST_PORT", port.to_string())
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::inherit())
        .kill_on_drop(true)
        .spawn()
        .expect("spawn")
}

async fn wait_ready(port: u16) {
    let client = reqwest::Client::new();
    for _ in 0..50 {
        if client
            .get(format!("http://127.0.0.1:{port}/healthz"))
            .send()
            .await
            .is_ok()
        {
            return;
        }
        tokio::time::sleep(Duration::from_millis(50)).await;
    }
    panic!("host did not become ready on port {port}");
}

async fn read_frame<R: AsyncReadExt + Unpin>(r: &mut R) -> Value {
    let mut len = [0u8; 4];
    r.read_exact(&mut len).await.unwrap();
    let n = u32::from_ne_bytes(len) as usize;
    let mut buf = vec![0u8; n];
    r.read_exact(&mut buf).await.unwrap();
    serde_json::from_slice(&buf).unwrap()
}

async fn write_frame<W: AsyncWriteExt + Unpin>(w: &mut W, v: &Value) {
    let body = serde_json::to_vec(v).unwrap();
    let n = (body.len() as u32).to_ne_bytes();
    w.write_all(&n).await.unwrap();
    w.write_all(&body).await.unwrap();
    w.flush().await.unwrap();
}

#[tokio::test(flavor = "multi_thread", worker_threads = 2)]
async fn healthz_responds() {
    let port = 17081;
    let mut child = spawn_host(port).await;
    wait_ready(port).await;
    let r = reqwest::get(format!("http://127.0.0.1:{port}/healthz"))
        .await
        .unwrap();
    assert!(r.status().is_success());
    let body: Value = r.json().await.unwrap();
    assert_eq!(body["ok"], true);
    assert!(body["version"].is_string());
    let _ = child.kill().await;
}

#[tokio::test(flavor = "multi_thread", worker_threads = 2)]
async fn execute_roundtrip() {
    let port = 17082;
    let mut child = spawn_host(port).await;
    wait_ready(port).await;
    let mut stdin = child.stdin.take().unwrap();
    let mut stdout = child.stdout.take().unwrap();

    let client = tokio::spawn(async move {
        let r = reqwest::Client::new()
            .post(format!("http://127.0.0.1:{port}/execute"))
            .json(&json!({ "command": "click", "selector": "#x" }))
            .send()
            .await
            .unwrap();
        r.json::<Value>().await.unwrap()
    });

    let framed = read_frame(&mut stdout).await;
    let id = framed["id"].as_str().unwrap().to_string();
    assert_eq!(framed["type"], "execute");
    write_frame(
        &mut stdin,
        &json!({ "id": id, "success": true, "message": "clicked" }),
    )
    .await;

    let resp = client.await.unwrap();
    assert_eq!(resp["success"], true);
    let _ = child.kill().await;
}

#[tokio::test(flavor = "multi_thread", worker_threads = 2)]
async fn execute_times_out_when_extension_does_not_reply() {
    let port = 17083;
    let mut child = spawn_host(port).await;
    wait_ready(port).await;
    // Hold stdin open but don't respond. Use a short timeout via a fake environment:
    // the default timeout for /execute is 5s, so we need to wait — keep this test lean.
    let start = std::time::Instant::now();
    let r = reqwest::Client::new()
        .post(format!("http://127.0.0.1:{port}/execute"))
        .json(&json!({ "command": "click", "selector": "#x" }))
        .send()
        .await
        .unwrap();
    assert_eq!(r.status().as_u16(), 504);
    assert!(start.elapsed() >= Duration::from_secs(4));
    let body: Value = r.json().await.unwrap();
    assert_eq!(body["error"]["code"], "timeout");
    let _ = child.kill().await;
}

#[tokio::test(flavor = "multi_thread", worker_threads = 2)]
async fn bad_origin_is_forbidden() {
    let port = 17084;
    let mut child = spawn_host(port).await;
    wait_ready(port).await;
    let r = reqwest::Client::new()
        .post(format!("http://127.0.0.1:{port}/execute"))
        .header("origin", "http://evil.example")
        .json(&json!({ "command": "click", "selector": "#x" }))
        .send()
        .await
        .unwrap();
    assert_eq!(r.status().as_u16(), 403);
    let _ = child.kill().await;
}
