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
  /system\s+prompt/i,
  /print\s+your\s+instructions/i,
  /reveal\s+(?:your\s+)?(?:secret\s+)?prompt/i,
  /do\s+anything\s+now/i,
  /simulate\s+a\s+(?:developer|admin)/i,
  /developer\s+mode/i,
  /bypass\s+(?:filters|security)/i,
  /ignore\s+(?:the\s+)?above\s+and\s+instead/i,
];

const CREDENTIAL_PATTERNS = [
  { regex: /\bAKIA[0-9A-Z]{16}\b/, name: "AWS Access Key ID" },
  { regex: /\b(?:ghp|gho|ghu|ghs|ghr)_[a-zA-Z0-9]{36}\b/, name: "GitHub Token" },
  { regex: /\bxox[baprs]-[0-9]{10,13}-[0-9]{10,13}[a-zA-Z0-9-]*\b/, name: "Slack Token" },
  { regex: /\b(?:sk|rk)_live_[0-9a-zA-Z]{24}\b/, name: "Stripe Live Key" },
  { regex: /-----BEGIN (?:RSA|OPENSSH|DSA|EC|PGP) PRIVATE KEY-----/, name: "Private Key" },
];

const PII_PATTERNS = [
  // Basic SSN Pattern (not foolproof, but captures typical formatted ones)
  { regex: /\b(?!000|666)[0-8][0-9]{2}-(?!00)[0-9]{2}-(?!0000)[0-9]{4}\b/, name: "Social Security Number" },
  // Basic Credit Card Pattern (Visa, MasterCard, Amex, Discover)
  { regex: /\b(?:4[0-9]{12}(?:[0-9]{3})?|5[1-5][0-9]{14}|3[47][0-9]{13}|6(?:011|5[0-9][0-9])[0-9]{12})\b/, name: "Credit Card Number" },
];

const SYSTEM_INJECTION = [
  // Command Injection
  /(?:;|\||&&|`|\$\()\s*(?:rm\s+-|wget|curl|bash|sh|nc|netcat|nmap|python|perl|ruby|php|node)\b/i,
  // Sensitive Files
  /\/etc\/(?:passwd|shadow|hosts|sudoers)/i,
];

const WEB_INJECTION = [
  // XSS and HTML Injection
  /<script\b[^>]*>[\s\S]*?<\/script>/i,
  /javascript\s*:/i,
  /\bon(?:error|load|click|mouseover|keydown)\s*=/i,
  // SQL Injection (Basic)
  /UNION\s+(?:ALL\s+)?SELECT/i,
  /\bOR\s+['"]?1['"]?\s*=\s*['"]?1['"]?/i,
  /;\s*(?:DROP|ALTER|TRUNCATE)\s+TABLE/i,
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
      break; // Report the first matched injection pattern
    }
  }

  for (const cred of CREDENTIAL_PATTERNS) {
    if (cred.regex.test(content)) {
      warnings.push(`Possible exposed credential: ${cred.name}`);
    }
  }

  for (const pii of PII_PATTERNS) {
    if (pii.regex.test(content)) {
      warnings.push(`Possible Personally Identifiable Information (PII) detected: ${pii.name}`);
    }
  }

  for (const sys of SYSTEM_INJECTION) {
    if (sys.test(content)) {
      warnings.push(`Possible system command injection or sensitive file access: ${sys.source}`);
    }
  }

  for (const web of WEB_INJECTION) {
    if (web.test(content)) {
      warnings.push(`Possible web/SQL injection detected: ${web.source}`);
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
