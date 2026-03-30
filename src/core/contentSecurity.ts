export interface ScanResult {
  safe: boolean;
  warnings: string[];
}

// eslint-disable-next-line no-misleading-character-class
const INVISIBLE_UNICODE = /[\u200B\u200C\u200D\u200E\u200F\u202A-\u202E\u2060\u2066-\u2069\uFEFF\uFE00-\uFE0F]/;

const INJECTION_PATTERNS = [
  /ignore\s+(?:all\s+)?previous\s+instructions/i,
  /disregard\s+(?:all\s+)?(?:previous|prior|above)/i,
  /you\s+are\s+now\s+a/i,
  /override\s+your\s+(?:instructions|rules|guidelines)/i,
  /forget\s+everything\s+(?:you|and)/i,
  /new\s+instructions\s*:/i,
];

const EXFILTRATION_TOOL = /\b(?:curl|wget)\b|fetch\s*\(/i;
const EXFILTRATION_SECRET = /\$[A-Z_]*(?:SECRET|KEY|TOKEN|PASSWORD)|process\.env\b|\.env\b/i;

const LONG_BASE64 = /[A-Za-z0-9+/=]{100,}/;

export function scanContent(content: string): ScanResult {
  const warnings: string[] = [];

  if (INVISIBLE_UNICODE.test(content)) {
    warnings.push("Invisible unicode characters detected (zero-width, directional override, or variation selector)");
  }

  for (const pattern of INJECTION_PATTERNS) {
    if (pattern.test(content)) {
      warnings.push(`Possible prompt injection pattern: ${pattern.source}`);
      break;
    }
  }

  if (EXFILTRATION_TOOL.test(content) && EXFILTRATION_SECRET.test(content)) {
    warnings.push("Possible exfiltration attempt: HTTP tool combined with secret/env variable reference");
  }

  if (LONG_BASE64.test(content)) {
    warnings.push("Suspicious encoded payload: long base64-like string (100+ chars)");
  }

  return { safe: warnings.length === 0, warnings };
}
