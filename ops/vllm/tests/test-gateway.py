import asyncio
import json
from pathlib import Path
import httpx
import gateway as g

async def main():
    async with g.lifespan(g.app):
        await g.client.aclose()
        ready = asyncio.Event()

        async def upstream(request):
            assert request.headers['Authorization'] == 'Bearer ' + g.backend_key
            await ready.wait()
            return httpx.Response(200, json={'choices': [{'message': {'content': 'ok'}}]})

        g.client = httpx.AsyncClient(transport=httpx.MockTransport(upstream))
        token = next(iter(json.loads(Path('/home/llmops/llm/config/client-keys.json').read_text()).values()))
        async with httpx.AsyncClient(transport=httpx.ASGITransport(app=g.app), base_url='http://test') as c:
            auth = {'Authorization': 'Bearer ' + token}
            r = await c.post('/v1/chat/completions', json={})
            assert r.status_code == 401
            assert g.active == 0
            tasks = [asyncio.create_task(c.post('/v1/chat/completions', headers=auth, json={'model': 'qwen3.8-27b', 'messages': []})) for _ in range(4)]
            for _ in range(100):
                if g.active == 4:
                    break
                await asyncio.sleep(.01)
            assert g.active == 4, g.active
            fifth = await c.post('/v1/chat/completions', headers=auth, json={})
            assert fifth.status_code == 429
            ready.set()
            result = await asyncio.gather(*tasks)
            assert all(x.status_code == 200 for x in result)
            assert g.active == 0
            print('PASS exactly four admitted requests; fifth receives 429; slots released')
            for body in [{'n': 2}, {'best_of': 2}, {'prompt': ['one', 'two']}]:
                r = await c.post('/v1/completions', headers=auth, json=body)
                assert r.status_code == 400
                assert g.active == 0
            r = await c.post('/v1/chat/completions', headers=auth, content=b'invalid')
            assert r.status_code == 400 and g.active == 0
            r = await c.post('/v1/chat/completions', headers=auth, content=b'x' * (g.MAX_BODY_BYTES + 1))
            assert r.status_code == 413 and g.active == 0
            print('PASS authentication, generation fan-out rejection, invalid JSON and body limit')

            async def unavailable(request):
                raise httpx.ConnectError('backend unavailable')
            await g.client.aclose()
            g.client = httpx.AsyncClient(transport=httpx.MockTransport(unavailable))
            r = await c.post('/v1/chat/completions', headers=auth, json={})
            assert r.status_code == 503 and g.active == 0
            print('PASS backend unavailable returns 503 and releases admission slot')

asyncio.run(main())
