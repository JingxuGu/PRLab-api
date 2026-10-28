import os,secrets
from pathlib import Path
os.umask(0o077)
p=Path('/home/llmops/new-api/config/service.env')
if p.exists():
 print('Existing environment retained')
else:
 p.write_text('\n'.join([
 'SESSION_SECRET='+secrets.token_hex(32),
 'CRYPTO_SECRET='+secrets.token_hex(32),
 'SQLITE_PATH=/home/llmops/new-api/data/new-api.db',
 'TZ=Asia/Shanghai','GIN_MODE=release','GOMAXPROCS=2',
 'MEMORY_CACHE_ENABLED=true','BATCH_UPDATE_ENABLED=false',
 'STREAMING_TIMEOUT=300','RELAY_TIMEOUT=360','POLLING_INTERVAL=0',
 'ERROR_LOG_ENABLED=false','DEBUG=false',
 'GENERATE_DEFAULT_TOKEN=false','STREAM_SCANNER_MAX_BUFFER_MB=8',
 'RELAY_MAX_IDLE_CONNS=16','RELAY_MAX_IDLE_CONNS_PER_HOST=8',
 ])+'\n')
 print('Private environment created')
