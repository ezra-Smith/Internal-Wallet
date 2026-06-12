/**
 * Cloudflare Worker: admin.zinkapi.com/api/* same-origin proxy -> https://api.zinkapi.com
 *
 * Keeps Admin Web on a static host while preserving same-origin /api to avoid CORS changes.
 */

const TARGET_ORIGIN = "https://api.zinkapi.com";

function isHopByHopHeader(name: string) {
  const n = name.toLowerCase();
  return (
    n === "connection" ||
    n === "keep-alive" ||
    n === "proxy-authenticate" ||
    n === "proxy-authorization" ||
    n === "te" ||
    n === "trailers" ||
    n === "transfer-encoding" ||
    n === "upgrade" ||
    n === "host"
  );
}

export default {
  async fetch(request: Request): Promise<Response> {
    const url = new URL(request.url);

    // Only proxy /api/*
    if (!url.pathname.startsWith("/api/")) {
      return new Response("Not found", { status: 404 });
    }

    const targetUrl = new URL(TARGET_ORIGIN);
    targetUrl.pathname = url.pathname;
    targetUrl.search = url.search;

    const headers = new Headers();
    request.headers.forEach((value, key) => {
      if (isHopByHopHeader(key)) return;
      headers.set(key, value);
    });

    // Preserve original host for backend logging if desired.
    headers.set("X-Forwarded-Host", url.host);
    headers.set("X-Forwarded-Proto", url.protocol.replace(":", ""));

    const init: RequestInit = {
      method: request.method,
      headers,
      redirect: "manual",
    };

    // Only attach body for methods that allow it.
    if (request.method !== "GET" && request.method !== "HEAD") {
      init.body = request.body;
    }

    const resp = await fetch(targetUrl.toString(), init);

    // Do not allow cache poisoning at the edge for API.
    const outHeaders = new Headers(resp.headers);
    outHeaders.set("Cache-Control", "no-store");

    return new Response(resp.body, {
      status: resp.status,
      statusText: resp.statusText,
      headers: outHeaders,
    });
  },
};

