import { GoogleGenAI } from "@google/genai";
import * as dotenv from "dotenv";
dotenv.config();

const ai = new GoogleGenAI({ apiKey: process.env.GEMINI_API_KEY });
async function run() {
  const response = await ai.models
    .embedContent({
      model: "text-embedding-004",
      contents: "test",
    })
    .catch((e) => console.error(e));
  console.log(response);

  const response2 = await ai.models
    .embedContent({
      model: "text-embedding-004",
      contents: "test",
    })
    .catch((e) => console.error(e));

  const response3 = await ai.models
    .embedContent({
      model: "gemini-embedding-001",
      contents: "test",
    })
    .catch((e) => console.error(e));

  if (response3) console.log(response3.embeddings?.[0]?.values?.length);
}
run();
