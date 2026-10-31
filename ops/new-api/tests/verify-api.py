import argparse
import asyncio
from contextlib import contextmanager
import http.cookiejar
import os
import secrets
import urllib.request
import base64
import io
import json
import time
from pathlib import Path

import httpx
from PIL import Image, ImageDraw

BASE = Path('/home/llmops/new-api')
# Tests use one short-lived administrator-owned token; no bootstrap users are needed.
API_KEY = None
SMOKE = False

@contextmanager
def maintenance_token():
    op = urllib.request.build_opener(urllib.request.ProxyHandler({}),
        urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
    admin = json.loads((BASE / 'config/initial-credentials.json').read_text())['example-admin']
    uid = None
    name = 'maintenance-' + secrets.token_hex(6)
    tid = None
    def request(path, data=None, method=None):
        headers = {'Content-Type': 'application/json'}
        if uid is not None:
            headers['New-Api-User'] = str(uid)
        q = urllib.request.Request(URL + path, data=None if data is None else json.dumps(data).encode(),
                                   headers=headers, method=method)
        with op.open(q, timeout=30) as r:
            result = json.load(r)
        if not result.get('success'):
            raise RuntimeError('Maintenance operation failed: ' + path)
        return result.get('data')
    def tokens():
        data = request('/api/token/?p=1&page_size=100')
        return data.get('items', []) if isinstance(data, dict) else data
    try:
        login = request('/api/user/login', {'username': admin['username'], 'password': admin['password']})
        uid = login['id']
        if login['role'] != 100:
            raise RuntimeError('Root administrator required')
        request('/api/token/', {'name': name, 'expired_time': int(time.time()) + 900,
            'remain_quota': 100000, 'unlimited_quota': False, 'model_limits_enabled': True,
            'model_limits': MODEL, 'group': 'default', 'cross_group_retry': False})
        tid = next(t['id'] for t in tokens() if t['name'] == name)
        yield 'sk-' + request(f'/api/token/{tid}/key', {})['key']
    finally:
        if uid is not None:
            # Also find a token if creation succeeded but the following response was interrupted.
            if tid is None:
                tid = next((t['id'] for t in tokens() if t['name'] == name), None)
            if tid is not None:
                request(f'/api/token/{tid}', method='DELETE')
            request('/api/user/logout')
MODEL = 'qwen3.8-27b'
URL = 'http://127.0.0.1:3001'

def fixture(large=False, variant=0):
    width, height = (1024, 1024) if large else (384, 192)
    image = Image.new('RGB', (width, height), 'white')
    d = ImageDraw.Draw(image)
    d.rectangle((width//20, height//6, width*2//5, height*5//6), fill='red')
    d.ellipse((width*3//5, height//6, width*19//20, height*5//6), fill='blue')
    d.point((variant, 0), fill='black')
    f = io.BytesIO()
    image.save(f, format='PNG')
    return 'data:image/png;base64,' + base64.b64encode(f.getvalue()).decode()

def body(content, max_tokens=128):
    return {'model': MODEL, 'messages': [{'role': 'user', 'content': content}],
            'max_tokens': max_tokens, 'temperature': 0,
            'chat_template_kwargs': {'enable_thinking': False}}

def auth(i=0):
    return {'Authorization': 'Bearer ' + API_KEY}

async def main():
    result = {}
    async with httpx.AsyncClient(base_url=URL, timeout=240, trust_env=False) as client:
        r = await client.get('/v1/models', headers={'Authorization':'Bearer invalid'})
        assert r.status_code == 401, r.status_code
        result['auth'] = {'passed':True, 'invalid_key_status':401}
        r = await client.get('/v1/models', headers=auth())
        r.raise_for_status()
        assert MODEL in [m['id'] for m in r.json()['data']]
        r = await client.post('/v1/chat/completions', headers=auth(), json=body('1+1等于几？只输出数字。', 32))
        r.raise_for_status()
        answer = r.json()['choices'][0]['message']['content']
        assert '2' in answer, answer
        result['text'] = {'passed': True, 'answer': answer, 'usage': r.json().get('usage')}
        print('PASS real text generation:', answer, flush=True)
        if SMOKE:
            (BASE / 'config/api-smoke-verification.json').write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n')
            return
        r = await client.post('/v1/chat/completions', headers=auth(), json=body([
            {'type': 'image_url', 'image_url': {'url': fixture()}},
            {'type': 'text', 'text': '图片左边的方块是什么颜色，右边的圆是什么颜色？用中文简短回答。'},
        ], 128))
        r.raise_for_status()
        answer = r.json()['choices'][0]['message']['content']
        assert '红' in answer and '蓝' in answer, answer
        result['vision'] = {'passed': True, 'answer': answer, 'usage': r.json().get('usage')}
        print('PASS real image understanding:', answer, flush=True)

        tool_request = body('请调用 get_temperature 查询南京的气温，不要直接猜测气温。', 192)
        tool_request['tools'] = [{'type': 'function', 'function': {
            'name': 'get_temperature', 'description': '查询一个城市的实时气温',
            'parameters': {'type': 'object', 'properties': {'city': {'type': 'string'}}, 'required': ['city']},
        }}]
        tool_request['tool_choice'] = 'auto'
        r = await client.post('/v1/chat/completions', headers=auth(), json=tool_request)
        r.raise_for_status()
        calls = r.json()['choices'][0]['message'].get('tool_calls') or []
        assert calls and calls[0]['function']['name'] == 'get_temperature', r.json()['choices']
        args = json.loads(calls[0]['function']['arguments'])
        assert '南京' in args['city'] or 'nanjing' in args['city'].lower(), args
        result['tool_calling'] = {'passed': True, 'function': 'get_temperature', 'arguments': args}
        print('PASS structured tool call; no external tool executed', flush=True)

        bodies = [body([
            {'type': 'image_url', 'image_url': {'url': fixture(large=True, variant=i)}},
            {'type': 'text', 'text': '请详细描述这张图片，再分条解释其中形状、颜色和位置关系。'},
        ], 192) for i in range(4)]
        start = time.monotonic()
        tasks = [asyncio.create_task(client.post('/v1/chat/completions', headers=auth(i), json=bodies[i])) for i in range(4)]
        maximum = 0
        fifth_status = None
        for _ in range(200):
            h = (await client.get('http://127.0.0.1:8000/health')).json()
            maximum = max(maximum, h['active_requests'])
            if h['active_requests'] == 4:
                fifth = await client.post('/v1/chat/completions', headers=auth(4), json=bodies[0])
                fifth_status = fifth.status_code
                break
            await asyncio.sleep(.05)
        responses = await asyncio.gather(*tasks)
        for response in responses:
            response.raise_for_status()
            assert response.json()['choices'][0]['message']['content']
        assert maximum == 4 and fifth_status == 429, (maximum, fifth_status)
        assert (await client.get('http://127.0.0.1:8000/health')).json()['active_requests'] == 0
        result['concurrency'] = {'passed': True, 'maximum_active': maximum, 'fifth_status': fifth_status,
                                 'four_request_seconds': round(time.monotonic() - start, 2),
                                 'usage': [r.json().get('usage') for r in responses]}
        print('PASS four real requests; fifth request receives 429 (global gateway limit)', flush=True)

        async with client.stream('POST', '/v1/chat/completions', headers=auth(),
                                 json={**body('请用一句中文介绍你自己。', 64), 'stream': True}) as stream:
            stream.raise_for_status()
            text = ''.join([part async for part in stream.aiter_text()])
        assert '[DONE]' in text
        result['streaming'] = {'passed': True}
        print('PASS real SSE streaming', flush=True)

    (BASE / 'config/api-verification.json').write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n')

if __name__ == '__main__':
    os.umask(0o077)
    parser = argparse.ArgumentParser(description='Uses real inference; no demo users are created.')
    parser.add_argument('--smoke', action='store_true', help='Only authentication, models and one short text request')
    SMOKE = parser.parse_args().smoke
    with maintenance_token() as key:
        API_KEY = key
        asyncio.run(main())
    API_KEY = None
    print('PASS: temporary maintenance token revoked; no user created', flush=True)
