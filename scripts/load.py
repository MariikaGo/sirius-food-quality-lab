import csv
import json
import os
import subprocess
import threading
import time
import urllib.request
import urllib.parse
from pathlib import Path

root = Path(__file__).resolve().parent.parent


def dc(*args, **kwargs):
    return subprocess.check_output(["docker", "compose", *args], cwd=root, text=True, **kwargs).strip()


def http(path, data, form=False):
    body = urllib.parse.urlencode(data).encode() if form else json.dumps(data).encode()
    headers = {"Content-Type": "application/x-www-form-urlencoded" if form else "application/json"}
    with urllib.request.urlopen(urllib.request.Request("http://127.0.0.1:8000" + path, data=body, headers=headers), timeout=30) as r:
        return json.load(r)


email = f"load-{time.time_ns()}@example.com"
password = "LoadFixture2026!"
user = http("/users/register", {"email": email, "password": password})["user_id"]
dc("exec", "-T", "db", "psql", "-U", "sirius", "-d", "sirius_practice10", "-c", f"UPDATE users SET balance_cents=10000000000 WHERE id={int(user)}")
token = http("/auth/login", {"username": email, "password": password}, True)["access_token"]
env = dict(os.environ, LOAD_TOKEN=token)
samples = []
stopped = threading.Event()


def monitor():
    while not stopped.wait(0.5):
        query = "SELECT coalesce(wait_event_type,'CPU'),coalesce(wait_event,'active'),count(*) FROM pg_stat_activity WHERE datname=current_database() AND state='active' AND pid<>pg_backend_pid() GROUP BY 1,2"
        text = dc("exec", "-T", "db", "psql", "-U", "sirius", "-d", "sirius_practice10", "-Atc", query)
        samples.append({"time": time.time(), "postgres_active": text})


def cleanup():
    dc("stop", "worker", "scheduler")
    try:
        print(dc("run", "--rm", "--no-deps", "api", "./loadcleanup", str(user)))
    finally:
        dc("up", "-d", "--no-build", "--wait", "worker", "scheduler")


if os.environ.get("LOAD_UI") == "1":
    try:
        subprocess.run(["docker", "compose", "--profile", "load", "run", "--rm", "--service-ports", "-e", "LOAD_TOKEN", "locust"], cwd=root, env=env, check=True)
    except KeyboardInterrupt:
        pass
    except subprocess.CalledProcessError as error:
        if error.returncode not in (130,137,143,-2):
            raise
    finally:
        cleanup()
else:
    thread = threading.Thread(target=monitor)
    thread.start()
    try:
        for users in (50, 200):
            subprocess.run(["docker", "compose", "--profile", "load", "run", "--rm", "-e", "LOAD_TOKEN", "locust", "-f", "/load/locustfile.py", "--host", "http://api:8000", "--headless", "-u", str(users), "-r", "20", "-t", "45s", "--csv", f"/evidence/load-{users}", "--html", f"/evidence/load-{users}.html"], cwd=root, env=env, check=True)
    finally:
        stopped.set()
        thread.join()
        (root / "evidence/db-load-samples.json").write_text(json.dumps(samples, indent=2))
        cleanup()
    result = []
    for users in (50, 200):
        rows = list(csv.DictReader((root / f"evidence/load-{users}_stats.csv").open()))
        result.append({"users": users, "rows": rows})
    (root / "evidence/load-summary.json").write_text(json.dumps({"workload": "70% menu, 20% item, 10% create; shared test account; pause 10-50ms", "phases": result}, indent=2))
    print("Результаты: evidence/load-summary.json и db-load-samples.json")
