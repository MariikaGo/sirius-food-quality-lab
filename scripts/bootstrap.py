#!/usr/bin/env python3
from pathlib import Path
import secrets
p=Path(__file__).resolve().parent.parent/'.env'
if not p.exists():
    p.touch(mode=0o600,exist_ok=False)
    p.write_text("DEBUG=true\nCACHE_ENABLED=true\nTASKS_MODE=async\n"+''.join(k+'='+secrets.token_hex(32)+'\n' for k in ['JWT_SECRET', 'SERVICE_TOKEN', 'RABBITMQ_PASSWORD', 'GRAFANA_PASSWORD']))
    print('Создан локальный .env со случайными секретами')
