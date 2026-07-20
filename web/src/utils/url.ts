// safeExternalUrl returns the URL only if it is a real http(s) link, else
// null. Attachment/link values that reach an <a href> must pass through here:
// React does not block javascript:/data: URLs in href, so an unchecked value
// would execute in the user's session on click (stored XSS). Defense in depth
// — the server rejects the same schemes on write (chat_service.go).
export function safeExternalUrl(raw: string | null | undefined): string | null {
  if (!raw) return null;
  let parsed: URL;
  try {
    // No base URL: the server accepts only absolute http(s) links with a host,
    // so relative values such as /file.pdf must fail here as well.
    parsed = new URL(raw);
  } catch {
    return null;
  }
  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    return null;
  }
  return parsed.href;
}
