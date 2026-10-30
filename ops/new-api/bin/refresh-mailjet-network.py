#!/usr/bin/env python3
"""Refresh exact Mailjet IP allowances while preserving platform isolation."""
import argparse
import ipaddress
from pathlib import Path
import socket
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request

HOST = 'in-v3.mailjet.com'
UNIT_DIR = '/etc/systemd/system/new-api.service.d'
PROBE = '''import smtplib,ssl
s=smtplib.SMTP("in-v3.mailjet.com",587,timeout=15)
s.ehlo()
assert s.has_extn("starttls")
s.starttls(context=ssl.create_default_context())
s.ehlo()
assert s.has_extn("auth")
s.quit()
print("PASS: Mailjet verified STARTTLS under restricted service network policy")
'''


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--apply', action='store_true', help='Install drop-in and restart only new-api')
    args = parser.parse_args()
    addresses = sorted({item[4][0] for item in socket.getaddrinfo(HOST, 587, type=socket.SOCK_STREAM)})
    if not addresses or any(not ipaddress.ip_address(value).is_global for value in addresses):
        raise RuntimeError('Expected public Mailjet addresses')
    if not args.apply:
        print('Mailjet public addresses: ' + ', '.join(addresses))
        print('Use --apply to refresh the service allowance; this restarts only new-api.')
        return
    subprocess.run(['sudo', '-n', 'true'], check=True)
    user = subprocess.check_output(['systemctl', 'show', 'new-api', '-p', 'User', '--value'], text=True).strip()
    if not user or user == 'root':
        raise RuntimeError('Expected a dedicated non-root platform service user')
    allow = ['IPAddressAllow=localhost', *['IPAddressAllow=' + address for address in addresses]]
    # Check verified TLS with the same restrictive network policy before installing it.
    command = ['sudo', '-n', 'systemd-run', '--quiet', '--wait', '--pipe', '--collect',
               '--property=User=' + user, '--property=PrivateDevices=yes',
               '--property=MemoryMax=128M', '--property=CPUQuota=10%',
               '--property=IPAddressDeny=any']
    for setting in allow:
        command.append('--property=' + setting)
    subprocess.run([*command, '/usr/bin/python3', '-c', PROBE], check=True)
    contents = '[Service]\n# Exact DNS addresses for Mailjet; rerun this tool if they change.\n'
    contents += '\n'.join(allow) + '\n'
    with tempfile.TemporaryDirectory(prefix='prlab-mailjet-') as folder:
        file = Path(folder) / '40-mailjet.conf'
        file.write_text(contents)
        subprocess.run(['sudo', '-n', 'install', '-d', '-m', '0755', UNIT_DIR], check=True)
        subprocess.run(['sudo', '-n', 'install', '-m', '0644', str(file), UNIT_DIR + '/40-mailjet.conf'], check=True)
    subprocess.run(['sudo', '-n', 'systemctl', 'daemon-reload'], check=True)
    subprocess.run(['sudo', '-n', 'systemctl', 'restart', 'new-api'], check=True)
    subprocess.run(['systemctl', 'is-active', '--quiet', 'new-api'], check=True)
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    for attempt in range(15):
        try:
            with opener.open('http://127.0.0.1:3001/api/status', timeout=2) as response:
                if response.status == 200:
                    break
        except (urllib.error.URLError, TimeoutError):
            pass
        if attempt == 14:
            raise RuntimeError('Platform did not become ready; inspect new-api service')
        time.sleep(1)
    print('PASS: exact Mailjet allowance installed; existing deny rules retained; new-api restarted')


if __name__ == '__main__':
    main()
