import type { NextRequest } from "next/server"

// SSE proxy for Agent Terminal.
//
// Why this route exists: Next's dev-server rewrite layer wraps proxied responses
// in gzip + chunked encoding. SSE frames (`data: ...\n\n`) get held in the
// compression buffer instead of flushing on each event, so the browser's
// EventSource sees nothing and eventually times out. Serving SSE from a local
// Route Handler bypasses the rewrite layer entirely.
//
// Auth: EventSource cannot set headers, so the browser passes the JWT as
// `?token=...`. We forward every incoming query param (including `token`) to
// the upstream URL. forge-core's JWTAuth middleware accepts both
// `Authorization: Bearer` AND `?token=` as fallback, so the query copy is
// sufficient — no header construction required.
export async function GET(
  request: NextRequest,
  ctx: { params: Promise<{ id: string }> },
) {
  const { id } = await ctx.params
  const apiUrl = process.env.API_URL || "http://localhost:8080"
  const incoming = new URL(request.url)

  // Forward all query params (including `token`) to upstream.
  const target = new URL(`${apiUrl}/api/projects/${id}/agent/stream`)
  incoming.searchParams.forEach((v, k) => target.searchParams.set(k, v))

  const headers = new Headers()
  // If an Authorization header was set (non-EventSource caller, e.g. curl),
  // preserve it. EventSource requests will not have one and rely on ?token=.
  const auth = request.headers.get("authorization")
  if (auth) headers.set("Authorization", auth)
  headers.set("Accept", "text/event-stream")
  headers.set("Cache-Control", "no-cache")
  // Critically: tell upstream not to compress so we get raw bytes back.
  headers.set("Accept-Encoding", "identity")

  let upstream: Response
  try {
    upstream = await fetch(target, {
      method: "GET",
      headers,
      signal: request.signal,
      cache: "no-store",
      // @ts-expect-error — undici extension required for streaming responses
      duplex: "half",
    })
  } catch {
    return new Response(
      `event: error\ndata: {"message":"upstream unreachable"}\n\n`,
      {
        status: 502,
        headers: {
          "Content-Type": "text/event-stream",
          "Content-Encoding": "identity",
        },
      },
    )
  }

  if (!upstream.body) {
    return new Response(
      `event: error\ndata: {"message":"empty upstream body"}\n\n`,
      {
        status: 502,
        headers: {
          "Content-Type": "text/event-stream",
          "Content-Encoding": "identity",
        },
      },
    )
  }

  return new Response(upstream.body, {
    status: upstream.status,
    headers: {
      "Content-Type": "text/event-stream",
      "Cache-Control": "no-cache, no-transform",
      Connection: "keep-alive",
      "X-Accel-Buffering": "no",
      // Block any downstream gzip/brotli that would buffer SSE chunks.
      "Content-Encoding": "identity",
    },
  })
}

export const dynamic = "force-dynamic"
export const runtime = "nodejs"
