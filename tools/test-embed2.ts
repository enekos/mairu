import { GoogleGenAI } from "@google/genai";
import * as dotenv from "dotenv";
dotenv.config();

const ai = new GoogleGenAI({ apiKey: process.env.GEMINI_API_KEY });
async function run() {
  const response = await ai.models
    .embedContent({
      model: "gemini-embedding-2-preview",
      contents: "test",
    })
    .catch((e) => console.error(e));

  if (response) console.log(response.embeddings?.[0]?.values?.length);
}
run();
