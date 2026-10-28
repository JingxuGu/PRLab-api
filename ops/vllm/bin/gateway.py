"""Single-worker local OpenAI proxy with four admitted generation requests."""
import hmac
import json
from contextlib import asynccontextmanager
from pathlib import Path

import httpx
from fastapi import FastAPI, HTTPException, Request
from fastapi.responses import JSONResponse, StreamingResponse
from starlette.background import BackgroundTask

BASE = Path('/home/llmops/llm')
BACKEND = 'http://127.0.0.1:8001'
LIMIT = 4
MAX_BODY_BYTES = 8 * 1024**2
active = 0
client = None
keys = []
backend_key = ''

@asynccontextmanager
async def lifespan(app):
    global client, keys, backend_key
    keys = list(json.loads((BASE / 'config/client-keys.json').read_text()).values())
    backend_key = (BASE / 'config/backend-key').read_text().strip()
    client = httpx.AsyncClient(timeout=httpx.Timeout(connect=3, read=1200, write=30, pool=3),
                               limits=httpx.Limits(max_connections=12, max_keepalive_connections=8),
                               trust_env=False)
    yield
    await client.aclose()

app = FastAPI(lifespan=lifespan, docs_url=None, redoc_url=None, openapi_url=None)

def authorize(request):
    header = request.headers.get('authorization', '')
    token = header[7:] if header.startswith('Bearer ') else ''
    if not token or not any(hmac.compare_digest(token, key) for key in keys):
        raise HTTPException(401, 'Invalid API key', headers={'WWW-Authenticate': 'Bearer'})

def unavailable():
    return JSONResponse({'error': {'message': 'Model is starting or its GPU is allocated to research. Retry later.',
                                   'type': 'service_unavailable'}}, status_code=503,
                        headers={'Retry-After': '30'})

@app.get('/livez')
async def live():
    return {'status': 'alive'}

@app.get('/health')
async def health():
    try:
        result = await client.get(BACKEND + '/health', headers={'Authorization': 'Bearer ' + backend_key})
        ready = result.status_code == 200
    except httpx.HTTPError:
        ready = False
    return JSONResponse({'ready': ready, 'active_requests': active, 'max_requests': LIMIT},
                        status_code=200 if ready else 503)

@app.get('/v1/models')
async def models(request: Request):
    authorize(request)
    try:
        response = await client.get(BACKEND + '/v1/models', headers={'Authorization': 'Bearer ' + backend_key})
    except httpx.HTTPError:
        return unavailable()
    return JSONResponse(response.json(), status_code=response.status_code)

@app.post('/v1/chat/completions')
@app.post('/v1/completions')
async def generate(request: Request):
    global active
    authorize(request)
    # All requests run in one event loop and one worker: no await between test and increment.
    if active >= LIMIT:
        return JSONResponse({'error': {'message': 'Four requests are already active. Retry later.',
                                       'type': 'concurrency_limit'}}, status_code=429,
                            headers={'Retry-After': '2'})
    active += 1
    upstream = None
    handed_off = False
    released = False

    async def release():
        global active
        nonlocal released
        if released:
            return
        released = True
        active -= 1
        if upstream is not None:
            await upstream.aclose()

    try:
        parts, length = [], 0
        async for part in request.stream():
            length += len(part)
            if length > MAX_BODY_BYTES:
                raise HTTPException(413, 'Request body exceeds 8 MiB')
            parts.append(part)
        try:
            body = json.loads(b''.join(parts))
        except (ValueError, UnicodeDecodeError):
            raise HTTPException(400, 'Invalid JSON')
        if not isinstance(body, dict):
            raise HTTPException(400, 'Expected a JSON object')
        # Avoid one HTTP request expanding into multiple parallel generations.
        if body.get('n', 1) != 1 or body.get('best_of', 1) != 1:
            raise HTTPException(400, 'n and best_of must be 1')
        prompt = body.get('prompt')
        if isinstance(prompt, list) and prompt and not isinstance(prompt[0], int):
            raise HTTPException(400, 'Batched prompts are disabled; send one request per prompt')
        req = client.build_request('POST', BACKEND + request.url.path,
                                   headers={'Authorization': 'Bearer ' + backend_key,
                                            'Content-Type': 'application/json',
                                            'Accept-Encoding': 'identity'}, json=body)
        try:
            upstream = await client.send(req, stream=True)
        except httpx.HTTPError:
            return unavailable()

        async def stream_body():
            try:
                async for chunk in upstream.aiter_bytes():
                    yield chunk
            finally:
                await release()

        headers = {'Cache-Control': 'no-cache', 'X-Accel-Buffering': 'no'}
        if 'content-type' in upstream.headers:
            headers['content-type'] = upstream.headers['content-type']
        handed_off = True
        return StreamingResponse(stream_body(), status_code=upstream.status_code,
                                 headers=headers, background=BackgroundTask(release))
    finally:
        if not handed_off:
            await release()
