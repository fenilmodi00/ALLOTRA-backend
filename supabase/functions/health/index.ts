import "jsr:@supabase/functions-js/edge-runtime.d.ts"

Deno.serve(async (_req) => {
  const headers = { 'Content-Type': 'application/json' }

  try {
    return new Response(JSON.stringify({
      status: 'ok',
      timestamp: new Date().toISOString(),
      service: 'ipo-backend-health'
    }), { headers })
  } catch (error) {
    return new Response(JSON.stringify({
      status: 'error',
      error: (error as Error).message
    }), { status: 500, headers })
  }
})
