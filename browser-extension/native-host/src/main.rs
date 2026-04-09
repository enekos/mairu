use serde_json::Value;
use std::collections::HashMap;
use std::io::{self, Read, Write};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Duration;
use tiny_http::{Method, Response, Server};

fn read_message() -> Option<Value> {
    let mut len_bytes = [0u8; 4];
    io::stdin().read_exact(&mut len_bytes).ok()?;
    let len = u32::from_ne_bytes(len_bytes) as usize;
    let mut msg_bytes = vec![0u8; len];
    io::stdin().read_exact(&mut msg_bytes).ok()?;
    serde_json::from_slice(&msg_bytes).ok()
}

fn write_message(msg: &Value) -> io::Result<()> {
    let msg_bytes = serde_json::to_vec(msg)?;
    let len = msg_bytes.len() as u32;
    io::stdout().write_all(&len.to_ne_bytes())?;
    io::stdout().write_all(&msg_bytes)?;
    io::stdout().flush()
}

fn main() {
    let server = Server::http("127.0.0.1:7081").unwrap();
    let pending_requests: Arc<Mutex<HashMap<String, Value>>> = Arc::new(Mutex::new(HashMap::new()));
    let pending_clone = pending_requests.clone();

    // Thread to read responses from Chrome extension
    thread::spawn(move || {
        while let Some(msg) = read_message() {
            if let Some(id) = msg.get("id").and_then(|i| i.as_str()) {
                let mut reqs = pending_clone.lock().unwrap();
                reqs.insert(id.to_string(), msg);
            }
        }
        std::process::exit(0);
    });

    let mut request_counter = 0;

    for mut request in server.incoming_requests() {
        if request.method() == &Method::Post
            && (request.url() == "/query"
                || request.url() == "/execute"
                || request.url() == "/screenshot")
        {
            let mut content = String::new();
            request
                .as_reader()
                .read_to_string(&mut content)
                .unwrap_or_default();

            if let Ok(mut json) = serde_json::from_str::<Value>(&content) {
                request_counter += 1;
                let req_id = format!("req-{}", request_counter);

                if let Value::Object(ref mut map) = json {
                    map.insert("id".to_string(), Value::String(req_id.clone()));
                    if request.url() == "/execute" {
                        map.insert("type".to_string(), Value::String("execute".to_string()));
                    } else if request.url() == "/screenshot" {
                        map.insert(
                            "command".to_string(),
                            Value::String("screenshot".to_string()),
                        );
                    }
                }

                if write_message(&json).is_ok() {
                    // Poll for response
                    let mut attempts = 0;
                    let mut response_val = None;

                    while attempts < 50 {
                        // 5 second timeout
                        thread::sleep(Duration::from_millis(100));
                        let mut reqs = pending_requests.lock().unwrap();
                        if let Some(resp) = reqs.remove(&req_id) {
                            response_val = Some(resp);
                            break;
                        }
                        attempts += 1;
                    }

                    if let Some(resp) = response_val {
                        let _ = request.respond(
                            Response::from_string(resp.to_string()).with_header(
                                tiny_http::Header::from_bytes(
                                    &b"Content-Type"[..],
                                    &b"application/json"[..],
                                )
                                .unwrap(),
                            ),
                        );
                        continue;
                    }
                }
            }
        }

        let _ = request.respond(
            Response::from_string("{\"error\": \"timeout or bad request\"}").with_status_code(400),
        );
    }
}
