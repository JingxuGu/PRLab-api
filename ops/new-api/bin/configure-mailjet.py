#!/usr/bin/env python3
"""Enter Mailjet credentials on the server TTY; never print or put them in argv."""
import argparse
import fcntl
import getpass
import http.cookiejar
import json
import os
from pathlib import Path
import re
import smtplib
import ssl
import subprocess
import sys
from email.message import EmailMessage
from email.utils import formataddr, formatdate, make_msgid
import urllib.request

BASE = Path(__file__).resolve().parents[1]
HOST = 'in-v3.mailjet.com'
PORT = 587
ORIGIN = 'http://127.0.0.1:3001'


def smtp_connection():
    client = smtplib.SMTP(HOST, PORT, timeout=20)
    try:
        client.ehlo()
        if not client.has_extn('starttls'):
            raise RuntimeError('STARTTLS unavailable')
        context = ssl.create_default_context()
        context.minimum_version = ssl.TLSVersion.TLSv1_2
        client.starttls(context=context)
        client.ehlo()
        return client
    except Exception:
        client.close()
        raise


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check', action='store_true', help='Check readiness without credentials or sending mail')
    args = parser.parse_args()
    os.umask(0o077)
    settings = json.loads((BASE / 'config/mailjet-settings.json').read_text())
    for name in ('from', 'recipient'):
        if not re.fullmatch(r'[A-Za-z0-9_.+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}', settings[name]):
            raise ValueError('Invalid configured email')
    cookies = http.cookiejar.CookieJar()
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(cookies))

    def request(path, data=None, uid=None, method=None):
        headers = {'Content-Type': 'application/json'}
        if uid is not None:
            headers['New-Api-User'] = str(uid)
        payload = None if data is None else json.dumps(data).encode()
        req = urllib.request.Request(ORIGIN + path, data=payload, headers=headers, method=method)
        with opener.open(req, timeout=20) as response:
            result = json.load(response)
        if not result.get('success'):
            raise RuntimeError('Platform request failed')
        return result.get('data')

    request('/api/status')
    with smtp_connection() as client:
        if not client.has_extn('auth'):
            raise RuntimeError('SMTP authentication unavailable')
    print('PASS: platform reachable; Mailjet STARTTLS certificate verified; no credentials used')
    if args.check:
        return
    if not sys.stdin.isatty() or not sys.stderr.isatty():
        raise RuntimeError('Run from an interactive SSH terminal with ssh -t')
    lock = (BASE / 'config/.mailjet-configure.lock').open('a')
    fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
    username = settings['admin_username']
    initial = BASE / 'config/initial-credentials.json'
    password = None
    if initial.exists():
        for account in json.loads(initial.read_text()).values():
            if account.get('username') == username:
                password = account.get('password')
                break
    if password is None:
        password = getpass.getpass('PRLab root administrator password (hidden): ')
    try:
        account = request('/api/user/login', {'username': username, 'password': password})
    except Exception:
        password = getpass.getpass('Administrator login failed; current root password (hidden): ')
        account = request('/api/user/login', {'username': username, 'password': password})
    password = None
    uid = account['id']
    if request('/api/user/self', uid=uid).get('role') != 100:
        raise RuntimeError('Root administrator required')
    try:
        subprocess.run([sys.executable, str(BASE / 'bin/backup-db.py')], check=True, capture_output=True)
        print('PASS: private database backup created')
        print('Enter Mailjet API Key and Secret Key below. One test email will be sent to the configured approval mailbox.')
        api_key = getpass.getpass('Mailjet API Key (hidden): ').strip()
        secret_key = getpass.getpass('Mailjet Secret Key (hidden): ').strip()
        for value in (api_key, secret_key):
            if not 16 <= len(value) <= 512 or any(c.isspace() for c in value) or not value.isascii():
                raise ValueError('Invalid credential format')
        message = EmailMessage()
        from_name = settings.get('from_name', '').strip()
        message['From'] = formataddr((from_name, settings['from']))
        message['To'] = settings['recipient']
        message['Subject'] = 'Mailjet SMTP connection test'
        message['Date'] = formatdate(localtime=True)
        message['Message-ID'] = make_msgid(domain=settings['from'].split('@')[1])
        message['Auto-Submitted'] = 'auto-generated'
        message.set_content('Mailjet 加密 SMTP 连接测试。\n这封邮件仅用于验证投递。\n邮件中不包含密码、API Key 或邀请链接。')
        with smtp_connection() as client:
            client.login(api_key, secret_key)
            refused = client.send_message(message)
            if refused:
                raise RuntimeError('Test recipient refused')
        print('PASS: Mailjet accepted the test email; confirm delivery in your mailbox')
        # Persist credentials only after verified TLS authentication and test acceptance.
        options = {
            'SMTPServer': HOST, 'SMTPPort': str(PORT), 'SMTPFrom': settings['from'],
            'SMTPSSLEnabled': 'false', 'SMTPForceAuthLogin': 'false',
            'SMTPAccount': api_key, 'SMTPToken': secret_key,
        }
        for key, value in options.items():
            request('/api/option/', {'key': key, 'value': value}, uid=uid, method='PUT')
        api_key = secret_key = None
        print('PASS: SMTP settings saved on the server; registration switches were not changed')
        (BASE / 'config/mailjet-configured.json').write_text(json.dumps({
            'smtp_configured': True, 'test_accepted': True, 'delivery_confirmed': False,
            'host': HOST, 'port': PORT, 'transport': 'STARTTLS',
        }, indent=2) + '\n')
    finally:
        try:
            request('/api/user/logout', uid=uid)
        except Exception:
            pass


if __name__ == '__main__':
    try:
        main()
    except (KeyboardInterrupt, EOFError):
        print('\nCancelled; no credential values printed.', file=sys.stderr)
        sys.exit(1)
    except Exception as error:
        # SMTP/HTTP exceptions may contain private values; never print their details.
        print('Configuration did not complete (' + type(error).__name__ + '). No credential values printed.', file=sys.stderr)
        sys.exit(1)
