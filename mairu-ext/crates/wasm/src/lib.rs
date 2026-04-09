use wasm_bindgen::prelude::*;

#[wasm_bindgen]
pub fn ping() -> String {
    "mairu-ext ready".to_string()
}
