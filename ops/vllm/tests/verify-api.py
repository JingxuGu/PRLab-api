import asyncio
import base64
import io
import json
import time
from pathlib import Path

import httpx
from PIL import Image, ImageDraw

BASE = Path('/home/llmops/llm')
KEYS = list(json.loads((BASE / 'config/client-keys.json').read_text()).values())
MODEL = 'qwen3.8-27b'
URL = 'http://127.0.0.1:8000'

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
    return {'Authorization': 'Bearer ' + KEYS[i]}

async def main():
    result = {}
    async with httpx.AsyncClient(base_url=URL, timeout=240, trust_env=False) as client:
        r = await client.get('/v1/models', headers=auth())
        r.raise_for_status()
        assert MODEL in [m['id'] for m in r.json()['data']]
        r = await client.post('/v1/chat/completions', headers=auth(), json=body('1+1等于几？只输出数字。', 32))
        r.raise_for_status()
        answer = r.json()['choices'][0]['message']['content']
        assert '2' in answer, answer
        result['text'] = {'passed': True, 'answer': answer, 'usage': r.json().get('usage')}
        print('PASS real text generation:', answer, flush=True)
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
            h = (await client.get('/health')).json()
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
        assert (await client.get('/health')).json()['active_requests'] == 0
        result['concurrency'] = {'passed': True, 'maximum_active': maximum, 'fifth_status': fifth_status,
                                 'four_request_seconds': round(time.monotonic() - start, 2),
                                 'usage': [r.json().get('usage') for r in responses]}
        print('PASS four real requests across four API keys; fifth key receives 429', flush=True)

        async with client.stream('POST', '/v1/chat/completions', headers=auth(),
                                 json={**body('请用一句中文介绍你自己。', 64), 'stream': True}) as stream:
            stream.raise_for_status()
            text = ''.join([part async for part in stream.aiter_text()])
        assert '[DONE]' in text
        result['streaming'] = {'passed': True}
        print('PASS real SSE streaming', flush=True)

    (BASE / 'config/api-verification.json').write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n')

asyncio.run(main())
