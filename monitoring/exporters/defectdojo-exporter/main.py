import os
import time
import urllib.error
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

API_URL = os.getenv("DD_API_URL", "http://defectdojo:8080")
API_TOKEN = os.getenv("DD_API_TOKEN", "")
SCRAPE_INTERVAL = int(os.getenv("SCRAPE_INTERVAL", "30"))
LIMIT = int(os.getenv("DD_PAGE_SIZE", "100"))

metrics = {}
last_scrape = 0.0
last_error = ""


def fetch_json(url):
    req = urllib.request.Request(url)
    req.add_header("Accept", "application/json")
    if API_TOKEN:
        req.add_header("Authorization", f"Token {API_TOKEN}")
    with urllib.request.urlopen(req, timeout=15) as resp:
        return resp.status, resp.read()


def scrape():
    global metrics, last_scrape, last_error
    found = {}
    try:
        url = f"{API_URL}/api/v2/findings/?limit={LIMIT}&is_active=true"
        status, body = fetch_json(url)
        if status != 200:
            last_error = f"defectdojo API returned HTTP {status}"
            return
        import json as _json

        data = _json.loads(body.decode())
        for finding in data.get("results", []):
            severity = finding.get("severity", "Info").lower()
            ftype = (finding.get("title") or "unknown").split(" ")[0].lower()
            product = "unknown"
            test = finding.get("test")
            if test and test.get("engagement"):
                engagement = test["engagement"]
                product = engagement.get("product") or (engagement.get("name") or "unknown")
                if not isinstance(product, str):
                    product = "unknown"
            key = (severity, ftype, str(product))
            found[key] = found.get(key, 0) + 1
        metrics = found
        last_scrape = time.time()
        last_error = ""
    except urllib.error.HTTPError as e:
        last_error = f"defectdojo API returned HTTP {e.code}"
    except Exception as e:
        last_error = f"scrape failed: {e}"


def render_metrics():
    lines = [
        "# HELP defectdojo_findings Number of active DefectDojo findings by severity/type/product",
        "# TYPE defectdojo_findings gauge",
    ]
    for (severity, ftype, product), count in sorted(metrics.items()):
        labels = f'severity="{severity}",finding_type="{ftype}",product="{product}"'
        lines.append(f"defectdojo_findings{{{labels}}} {count}")
    lines.append(
        "# HELP defectdojo_exporter_last_scrape_seconds Timestamp of last successful scrape"
    )
    lines.append("# TYPE defectdojo_exporter_last_scrape_seconds gauge")
    lines.append(f"defectdojo_exporter_last_scrape_seconds {last_scrape}")
    if last_error:
        lines.append(
            "# HELP defectdojo_exporter_scrape_error Whether the last scrape failed (1 = failed)"
        )
        lines.append("# TYPE defectdojo_exporter_scrape_error gauge")
        lines.append("defectdojo_exporter_scrape_error 1")
        lines.append(f'# ERROR {last_error}')
    return "\n".join(lines) + "\n"


class MetricsHandler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, format, *args):
        pass

    def do_GET(self):
        if self.path in ("/", "/metrics"):
            body = render_metrics().encode()
            self.send_response(200)
            self.send_header("Content-Type", "text/plain; version=0.0.4")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
        elif self.path == "/health":
            body = b'{"status":"ok"}'
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
        else:
            self.send_response(404)
            self.end_headers()


def main():
    import threading

    scrape()
    server = ThreadingHTTPServer(("0.0.0.0", 8081), MetricsHandler)
    print(f"defectdojo-exporter listening on :8081 (scrape interval {SCRAPE_INTERVAL}s)")
    threading.Thread(target=server.serve_forever, daemon=True).start()
    while True:
        time.sleep(SCRAPE_INTERVAL)
        scrape()


if __name__ == "__main__":
    main()
