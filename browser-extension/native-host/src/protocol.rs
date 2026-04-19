use serde_json::Value;
use thiserror::Error;
use tokio::io::{AsyncReadExt, AsyncWriteExt};

pub const MAX_MSG_BYTES: usize = 64 * 1024 * 1024; // 64 MiB

#[derive(Debug, Error)]
pub enum ProtocolError {
    #[error("io: {0}")]
    Io(#[from] std::io::Error),
    #[error("message too large: {0} bytes")]
    TooLarge(usize),
    #[error("json: {0}")]
    Json(#[from] serde_json::Error),
    #[error("eof")]
    Eof,
}

pub async fn read_message<R: AsyncReadExt + Unpin>(r: &mut R) -> Result<Value, ProtocolError> {
    let mut len_bytes = [0u8; 4];
    match r.read_exact(&mut len_bytes).await {
        Ok(_) => {}
        Err(e) if e.kind() == std::io::ErrorKind::UnexpectedEof => return Err(ProtocolError::Eof),
        Err(e) => return Err(ProtocolError::Io(e)),
    }
    let len = u32::from_ne_bytes(len_bytes) as usize;
    if len > MAX_MSG_BYTES {
        // Drain the oversized body so we can resynchronise on the next frame.
        let mut remaining = len;
        let mut scratch = [0u8; 8192];
        while remaining > 0 {
            let take = remaining.min(scratch.len());
            if r.read_exact(&mut scratch[..take]).await.is_err() {
                break;
            }
            remaining -= take;
        }
        return Err(ProtocolError::TooLarge(len));
    }
    let mut buf = vec![0u8; len];
    r.read_exact(&mut buf).await?;
    Ok(serde_json::from_slice(&buf)?)
}

pub async fn write_message<W: AsyncWriteExt + Unpin>(w: &mut W, v: &Value) -> Result<(), ProtocolError> {
    let body = serde_json::to_vec(v)?;
    let len = (body.len() as u32).to_ne_bytes();
    w.write_all(&len).await?;
    w.write_all(&body).await?;
    w.flush().await?;
    Ok(())
}
